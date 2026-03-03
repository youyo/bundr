package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/posener/complete"
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
)

// NewRefPredictor は ref-style 補完（ps:/path..., sm:name...）の
// complete.Predictor を返す。main.go から kongplete.WithPredictor に渡す用途。
func NewRefPredictor(cacheStore cache.Store, bgLauncher BGLauncher, factory BackendFactory) complete.Predictor {
	fn := newRefPredictor(cacheStore, bgLauncher, factory)
	return complete.PredictFunc(func(a complete.Args) []string {
		return fn(a.Last)
	})
}

// NewPrefixPredictor は prefix-style 補完（ps:/prefix）の
// complete.Predictor を返す。main.go から kongplete.WithPredictor に渡す用途。
func NewPrefixPredictor(cacheStore cache.Store, bgLauncher BGLauncher, factory BackendFactory) complete.Predictor {
	fn := newPrefixPredictor(cacheStore, bgLauncher, factory)
	return complete.PredictFunc(func(a complete.Args) []string {
		return fn(a.Last)
	})
}

// liveFetchPath は fetchLive に渡す prefix を計算する。
// PS/PSA パスは "/" 始まりの階層パスのため、末尾が "/" でない場合は親ディレクトリを返す。
// SM シークレット名（"/" を含まない）はそのまま返す（AWS API の扱いが PS と異なるため）。
// 例: "/s" → "/", "/stratalog/pr" → "/stratalog/", "/stratalog/" → "/stratalog/"
// 例: "my-secret" → "my-secret"（SM の場合は変更なし）
func liveFetchPath(refPath string) string {
	if !strings.HasPrefix(refPath, "/") {
		return refPath
	}
	if refPath == "/" {
		return "/"
	}
	if strings.HasSuffix(refPath, "/") {
		return refPath
	}
	if idx := strings.LastIndex(refPath, "/"); idx >= 0 {
		return refPath[:idx+1]
	}
	return "/"
}

// makeBGArg は cache refresh のバックグラウンド起動引数を返す。
// sm: はパスの概念がないため "sm:" を返し、それ以外は "ps:/" 形式を返す。
func makeBGArg(backendType string) string {
	if backendType == "sm" {
		return "sm:"
	}
	return backendType + ":/"
}

// hierarchicalFilter はキャッシュエントリから1階層分の候補を返す。
// refPath が指し示す親ディレクトリ直下の中間ノードまたはリーフノードのみを返し、
// 重複は排除する。
func hierarchicalFilter(refPath string, entries []cache.CacheEntry, refTypeStr string) []string {
	var parentPath string
	if refPath == "/" {
		parentPath = "/"
	} else if refPath == "" {
		// SM パスは先頭スラッシュなし（"stratalog/key"）→ parentPath = ""
		// PS/PSA パスは先頭スラッシュあり（"/app/key"）→ parentPath = "/"
		if refTypeStr != "sm" {
			parentPath = "/"
		}
		// sm の場合は "" のまま（Go zero value）
	} else if strings.HasSuffix(refPath, "/") {
		parentPath = refPath
	} else {
		if idx := strings.LastIndex(refPath, "/"); idx >= 0 {
			parentPath = refPath[:idx+1]
		} else {
			parentPath = ""
		}
	}

	seen := make(map[string]struct{})
	candidates := make([]string, 0)
	for _, e := range entries {
		if refPath != "" && !strings.HasPrefix(e.Path, refPath) {
			continue
		}
		relative := strings.TrimPrefix(e.Path, parentPath)
		slashIdx := strings.Index(relative, "/")
		var candidate string
		if slashIdx < 0 {
			candidate = refTypeStr + ":" + e.Path
		} else {
			candidate = refTypeStr + ":" + parentPath + relative[:slashIdx+1]
		}
		if _, ok := seen[candidate]; !ok {
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// newRefPredictor は ref-style 補完の内部関数（テスト用に cmd パッケージ内でアクセス可能）。
func newRefPredictor(cacheStore cache.Store, bgLauncher BGLauncher, factory BackendFactory) func(string) []string {
	return func(prefix string) []string {
		// 1. prefix からバックエンドタイプを判定
		ref, err := backend.ParseRef(prefix)
		if err != nil {
			// "sm:", "ps:" — パスなしのバックエンドプレフィックス
			// ParseRef は空パスを拒否するため、補完時は特別扱いする
			if prefix == "sm:" || prefix == "ps:" {
				btStr := strings.TrimSuffix(prefix, ":")
				bgArg := makeBGArg(btStr)
				entries, readErr := cacheStore.Read(btStr)
				if readErr == cache.ErrCacheNotFound {
					_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
					var livePath string
					if btStr != "sm" {
						livePath = "/"
					}
					liveEntries := fetchLive(factory, btStr, livePath)
					return hierarchicalFilter("", liveEntries, btStr)
				} else if readErr == nil {
					candidates := hierarchicalFilter("", entries, btStr)
					lastRefresh := cacheStore.LastRefreshedAt(btStr)
					if time.Since(lastRefresh) > 10*time.Second {
						_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
					}
					return candidates
				}
			}
			return []string{}
		}

		backendType := string(ref.Type)
		bgArg := makeBGArg(backendType)

		// 2. キャッシュを読む
		entries, err := cacheStore.Read(backendType)
		if err == cache.ErrCacheNotFound {
			// キャッシュなし → BG でキャッシュ作成 + リアルタイム API で候補を返す
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
			liveEntries := fetchLive(factory, backendType, liveFetchPath(ref.Path))
			return hierarchicalFilter(ref.Path, liveEntries, string(ref.Type))
		} else if err != nil {
			// ErrCacheNotFound 以外のエラー → stderr ログ、空リスト返す
			fmt.Fprintf(os.Stderr, "cache read error for %s: %v\n", backendType, err)
			return []string{}
		}

		// 4. キャッシュあり → 階層フィルタリングで候補生成
		candidates := hierarchicalFilter(ref.Path, entries, string(ref.Type))

		// 5. BG 更新スロットリング：LastRefreshedAt を確認、10 秒以内ならスキップ
		lastRefresh := cacheStore.LastRefreshedAt(backendType)
		if time.Since(lastRefresh) > 10*time.Second {
			// 6. BG 起動
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
		}

		return candidates
	}
}

// newPrefixPredictor は prefix-style 補完の内部関数（テスト用に cmd パッケージ内でアクセス可能）。
func newPrefixPredictor(cacheStore cache.Store, bgLauncher BGLauncher, factory BackendFactory) func(string) []string {
	return func(prefix string) []string {
		// 空文字（全バックエンド）の場合は ps:/sm: 全パスを返す
		if prefix == "" {
			var candidates []string
			for _, backendType := range []string{"ps", "sm"} {
				refreshArg := makeBGArg(backendType)

				entries, err := cacheStore.Read(backendType)
				if err == cache.ErrCacheNotFound {
					_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", refreshArg)
					// sm は空パス、ps は "/" でリアルタイム取得
					var livePath string
					if backendType != "sm" {
						livePath = "/"
					}
					liveEntries := fetchLive(factory, backendType, livePath)
					candidates = append(candidates, hierarchicalFilter("", liveEntries, backendType)...)
					continue
				} else if err != nil {
					fmt.Fprintf(os.Stderr, "cache read error for %s: %v\n", backendType, err)
					continue
				}

				candidates = append(candidates, hierarchicalFilter("", entries, backendType)...)

				lastRefresh := cacheStore.LastRefreshedAt(backendType)
				if time.Since(lastRefresh) > 10*time.Second {
					_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", refreshArg)
				}
			}
			return candidates
		}

		// 通常の prefix 指定
		ref, err := backend.ParseRef(prefix)
		if err != nil {
			// "sm:", "ps:" — パスなしのバックエンドプレフィックス
			// ParseRef は空パスを拒否するため、補完時は特別扱いする
			if prefix == "sm:" || prefix == "ps:" {
				btStr := strings.TrimSuffix(prefix, ":")
				bgArg := makeBGArg(btStr)
				entries, readErr := cacheStore.Read(btStr)
				if readErr == cache.ErrCacheNotFound {
					_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
					var livePath string
					if btStr != "sm" {
						livePath = "/"
					}
					liveEntries := fetchLive(factory, btStr, livePath)
					return hierarchicalFilter("", liveEntries, btStr)
				} else if readErr == nil {
					candidates := hierarchicalFilter("", entries, btStr)
					lastRefresh := cacheStore.LastRefreshedAt(btStr)
					if time.Since(lastRefresh) > 10*time.Second {
						_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
					}
					return candidates
				}
			}
			return []string{}
		}

		backendType := string(ref.Type)
		bgArg := makeBGArg(backendType)

		entries, err := cacheStore.Read(backendType)
		if err == cache.ErrCacheNotFound {
			// キャッシュなし → BG でキャッシュ作成 + リアルタイム API で候補を返す
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
			liveEntries := fetchLive(factory, backendType, liveFetchPath(ref.Path))
			return hierarchicalFilter(ref.Path, liveEntries, string(ref.Type))
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "cache read error for %s: %v\n", backendType, err)
			return []string{}
		}

		candidates := hierarchicalFilter(ref.Path, entries, string(ref.Type))

		lastRefresh := cacheStore.LastRefreshedAt(backendType)
		if time.Since(lastRefresh) > 10*time.Second {
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
		}

		return candidates
	}
}

// fetchLive は AWS API を直接呼んでパス一覧を取得する（タグ取得なし）。
// エラー時は nil を返す。factory が nil の場合も nil を返す。
func fetchLive(factory BackendFactory, backendType, prefix string) []cache.CacheEntry {
	if factory == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b, err := factory(backend.BackendType(backendType))
	if err != nil {
		return nil
	}

	entries, err := b.GetByPrefix(ctx, prefix, backend.GetByPrefixOptions{
		Recursive:    true,
		SkipTagFetch: true,
	})
	if err != nil {
		return nil
	}

	result := make([]cache.CacheEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, cache.CacheEntry{Path: e.Path})
	}
	return result
}
