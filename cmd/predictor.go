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

// newRefPredictor は ref-style 補完の内部関数（テスト用に cmd パッケージ内でアクセス可能）。
func newRefPredictor(cacheStore cache.Store, bgLauncher BGLauncher) func(string) []string {
	return func(prefix string) []string {
		// 1. prefix からバックエンドタイプを判定
		ref, err := backend.ParseRef(prefix)
		if err != nil {
			return []string{}
		}

		backendType := string(ref.Type)

		// 2. sm: は空リスト返す（GetByPrefix の概念がない）
		if ref.Type == backend.BackendTypeSM {
			return []string{}
		}

		// 3. キャッシュを読む
		entries, err := cacheStore.Read(backendType)
		if err == cache.ErrCacheNotFound {
			// キャッシュなし（初回）→ 空リストを返す（BG 更新は別途）
			return []string{}
		} else if err != nil {
			// ErrCacheNotFound 以外のエラー → stderr ログ、空リスト返す
			fmt.Fprintf(os.Stderr, "cache read error for %s: %v\n", backendType, err)
			return []string{}
		}

		// 4. キャッシュあり → エントリをフィルタリング
		var candidates []string
		for _, e := range entries {
			if strings.HasPrefix(e.Path, ref.Path) {
				candidates = append(candidates, string(ref.Type)+":"+e.Path)
			}
		}

		// 5. BG 更新スロットリング：LastRefreshedAt を確認、10 秒以内ならスキップ
		lastRefresh := cacheStore.LastRefreshedAt(backendType)
		if time.Since(lastRefresh) > 10*time.Second {
			// 6. BG 起動
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", "--prefix", prefix)
		}

		return candidates
	}
}

// newPrefixPredictor は prefix-style 補完の内部関数（テスト用に cmd パッケージ内でアクセス可能）。
func newPrefixPredictor(cacheStore cache.Store, bgLauncher BGLauncher) func(string) []string {
	return func(prefix string) []string {
		// 空文字（全バックエンド）の場合は ps: と psa: 両方のパスを返す
		if prefix == "" {
			var candidates []string
			for _, backendType := range []string{"ps", "psa"} {
				entries, err := cacheStore.Read(backendType)
				if err == cache.ErrCacheNotFound {
					continue
				} else if err != nil {
					fmt.Fprintf(os.Stderr, "cache read error for %s: %v\n", backendType, err)
					continue
				}

				for _, e := range entries {
					candidates = append(candidates, backendType+":"+e.Path)
				}

				lastRefresh := cacheStore.LastRefreshedAt(backendType)
				if time.Since(lastRefresh) > 10*time.Second {
					_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", "--prefix", backendType+":/")
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

		// sm: は空リスト返す
		if ref.Type == backend.BackendTypeSM {
			return []string{}
		}

		entries, err := cacheStore.Read(backendType)
		if err == cache.ErrCacheNotFound {
			return []string{}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "cache read error for %s: %v\n", backendType, err)
			return []string{}
		}

		var candidates []string
		for _, e := range entries {
			if strings.HasPrefix(e.Path, ref.Path) {
				candidates = append(candidates, string(ref.Type)+":"+e.Path)
			}
		}

		lastRefresh := cacheStore.LastRefreshedAt(backendType)
		if time.Since(lastRefresh) > 10*time.Second {
			_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", "--prefix", prefix)
		}

		return candidates
	}
}
