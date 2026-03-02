package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
)

// LsCmd represents the "ls" subcommand.
type LsCmd struct {
	From      string `arg:"" required:"" predictor:"prefix" help:"Source prefix (e.g. ps:/app/prod/)"`
	Recursive bool   `name:"recursive" help:"List recursively (default: non-recursive)"`

	out io.Writer // for testing; nil means os.Stdout
}

// Run executes the ls command.
func (c *LsCmd) Run(appCtx *Context) error {
	if c.out == nil {
		c.out = os.Stdout
	}

	// sm: (empty path) lists all secrets without a prefix filter
	var ref backend.Ref
	if c.From == "sm:" {
		ref = backend.Ref{Type: backend.BackendTypeSM, Path: ""}
	} else {
		var parseErr error
		ref, parseErr = backend.ParseRef(c.From)
		if parseErr != nil {
			return fmt.Errorf("ls command failed: invalid ref: %w", parseErr)
		}
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("ls command failed: create backend: %w", err)
	}

	entries, err := b.GetByPrefix(context.Background(), ref.Path, backend.GetByPrefixOptions{Recursive: c.Recursive})
	if err != nil {
		return fmt.Errorf("ls command failed: %w", err)
	}

	// コマンド実行後に即時キャッシュへ書き込む（Tab 補完の初回キャッシュミスを防ぐ）
	if appCtx.CacheStore != nil {
		cacheEntries := make([]cache.CacheEntry, 0, len(entries))
		for _, e := range entries {
			cacheEntries = append(cacheEntries, cache.CacheEntry{
				Path:      e.Path,
				StoreMode: e.StoreMode,
			})
		}
		_ = appCtx.CacheStore.Write(string(ref.Type), cacheEntries)
	}

	// フル ref 形式（ps:/path/to/key）に変換してソート
	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		refs = append(refs, string(ref.Type)+":"+entry.Path)
	}
	sort.Strings(refs)

	for _, r := range refs {
		fmt.Fprintln(c.out, r)
	}

	return nil
}
