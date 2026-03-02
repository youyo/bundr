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
	Clear   CacheClearCmd   `cmd:"" help:"Clear all local cache files."`
}

// CacheClearCmd はローカルキャッシュファイルを全て削除するサブコマンド。
type CacheClearCmd struct{}

// Run はキャッシュクリアコマンドを実行する。
func (c *CacheClearCmd) Run(appCtx *Context) error {
	return appCtx.CacheStore.Clear()
}

// CacheRefreshCmd はキャッシュを更新するサブコマンド。
type CacheRefreshCmd struct {
	Prefix string `arg:"" required:"" help:"SSM prefix to refresh (e.g. ps:/app/prod/)"`
}

// Run はキャッシュ更新コマンドを実行する。
// 1. ParseRef で prefix を検証
// 2. BackendFactory でバックエンドを作成
// 3. GetByPrefix(recursive=true) で全エントリ取得
// 4. CacheStore.Write でキャッシュに書き込む
func (c *CacheRefreshCmd) Run(appCtx *Context) error {
	// sm: (empty path) refreshes cache for all secrets
	var ref backend.Ref
	if c.Prefix == "sm:" {
		ref = backend.Ref{Type: backend.BackendTypeSM, Path: ""}
	} else {
		var parseErr error
		ref, parseErr = backend.ParseRef(c.Prefix)
		if parseErr != nil {
			return fmt.Errorf("cache refresh: invalid prefix: %w", parseErr)
		}
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
