package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/posener/complete"
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
)

// NewRefPredictor は ref-style 補完（ps:/path..., psa:/path..., sm:name...）の
// complete.Predictor を返す。main.go から kongplete.WithPredictor に渡す用途。
func NewRefPredictor(cacheStore cache.Store, bgLauncher BGLauncher) complete.Predictor {
	fn := newRefPredictor(cacheStore, bgLauncher)
	return complete.PredictFunc(func(a complete.Args) []string {
		return fn(a.Last)
	})
}

// NewPrefixPredictor は prefix-style 補完（ps:/prefix, psa:/prefix）の
// complete.Predictor を返す。main.go から kongplete.WithPredictor に渡す用途。
func NewPrefixPredictor(cacheStore cache.Store, bgLauncher BGLauncher) complete.Predictor {
	fn := newPrefixPredictor(cacheStore, bgLauncher)
	return complete.PredictFunc(func(a complete.Args) []string {
		return fn(a.Last)
	})
}

// hierarchicalFilter はキャッシュエントリから1階層分の候補を返す。
// refPath が指し示す親ディレクトリ直下の中間ノードまたはリーフノードのみを返し、
// 重複は排除する。
func hierarchicalFilter(refPath string, entries []cache.CacheEntry, refTypeStr string) []string {
	var parentPath string
	if refPath == "" || refPath == "/" {
		parentPath = "/"
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
	var candidates []string
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
func newRefPredictor(cacheStore cache.Store, bgLauncher BGLauncher) func(string) []string {
	return func(prefix string) []string {
		// 1. prefix からバックエンドタイプを判定
		ref, err := backend.ParseRef(prefix)
		if err != nil {
			return []string{}
		}

		backendType := string(ref.Type)
		bgArg := backendType + ":/"
		if backendType == "sm" {
			bgArg = "sm:"
		}

		// 2. キャッシュを読む
		entries, err := cacheStore.Read(backendType)
		if err == cache.ErrCacheNotFound {
			// キャッシュなし（初回）→ 次回補完のために BG でキャッシュ作成
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
			return []string{}
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
func newPrefixPredictor(cacheStore cache.Store, bgLauncher BGLauncher) func(string) []string {
	return func(prefix string) []string {
		// 空文字（全バックエンド）の場合は ps:/psa:/sm: 全パスを返す
		if prefix == "" {
			var candidates []string
			for _, backendType := range []string{"ps", "psa", "sm"} {
				// sm: のキャッシュリフレッシュ引数は "sm:"（空 path）、それ以外は "ps:/" 形式
				refreshArg := backendType + ":/"
				if backendType == "sm" {
					refreshArg = "sm:"
				}

				entries, err := cacheStore.Read(backendType)
				if err == cache.ErrCacheNotFound {
					_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", refreshArg)
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
			return []string{}
		}

		backendType := string(ref.Type)
		bgArg := backendType + ":/"
		if backendType == "sm" {
			bgArg = "sm:"
		}

		entries, err := cacheStore.Read(backendType)
		if err == cache.ErrCacheNotFound {
			// キャッシュなし（初回）→ 次回補完のために BG でキャッシュ作成
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
			return []string{}
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
