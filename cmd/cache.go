package cmd

import (
	"context"
	"fmt"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
)

// CacheCmd は cache サブコマンドの Kong 構造体。
type CacheCmd struct {
	Refresh CacheRefreshCmd `cmd:"" help:"Refresh the local cache by fetching paths from AWS."`
}

// CacheRefreshCmd はキャッシュを更新するサブコマンド。
type CacheRefreshCmd struct {
	Prefix string `arg:"" required:"" help:"SSM prefix to refresh (e.g. ps:/app/prod/)"`
}

// Run はキャッシュ更新コマンドを実行する。
// 1. ParseRef で prefix を検証（sm: は対象外）
// 2. BackendFactory でバックエンドを作成
// 3. GetByPrefix(recursive=true) で全エントリ取得
// 4. CacheStore.Write でキャッシュに書き込む
func (c *CacheRefreshCmd) Run(appCtx *Context) error {
	ref, err := backend.ParseRef(c.Prefix)
	if err != nil {
		return fmt.Errorf("cache refresh: invalid prefix: %w", err)
	}

	// sm: は補完対象外（GetByPrefix の概念がない）
	if ref.Type == backend.BackendTypeSM {
		return fmt.Errorf("cache refresh: sm: backend is not supported for completion cache")
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("cache refresh: create backend: %w", err)
	}

	entries, err := b.GetByPrefix(context.Background(), ref.Path, backend.GetByPrefixOptions{
		Recursive: true,
	})
	if err != nil {
		return fmt.Errorf("cache refresh: fetch parameters: %w", err)
	}

	// ParameterEntry を CacheEntry に変換（値は保存しない）
	cacheEntries := make([]cache.CacheEntry, 0, len(entries))
	for _, e := range entries {
		cacheEntries = append(cacheEntries, cache.CacheEntry{
			Path:      e.Path,
			StoreMode: e.StoreMode,
		})
	}

	// バックエンドタイプ名を取得（"ps", "psa"）
	backendType := string(ref.Type)
	if err := appCtx.CacheStore.Write(backendType, cacheEntries); err != nil {
		return fmt.Errorf("cache refresh: write cache: %w", err)
	}

	return nil
}
