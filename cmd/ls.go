package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/youyo/bundr/internal/backend"
)

// LsCmd represents the "ls" subcommand.
type LsCmd struct {
	From      string `arg:"" required:"" predictor:"prefix" help:"Source prefix (e.g. ps:/app/prod/)"`
	Recursive bool   `name:"recursive" help:"List recursively (default: non-recursive)"`
	Describe  bool   `name:"describe" help:"Show metadata as JSON array instead of refs"`

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

	if c.Describe {
		return c.runDescribe(context.Background(), b, ref)
	}

	entries, err := b.GetByPrefix(context.Background(), ref.Path, backend.GetByPrefixOptions{
		Recursive:    c.Recursive,
		SkipTagFetch: true,
	})
	if err != nil {
		return fmt.Errorf("ls command failed: %w", err)
	}

	// コマンド実行後に即時キャッシュへ書き込む（Tab 補完の初回キャッシュミスを防ぐ）
	if appCtx.CacheStore != nil {
		_ = appCtx.CacheStore.Write(string(ref.Type), toCacheEntries(entries))
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

// runDescribe outputs entries as a JSON array with "ref" + metadata fields.
func (c *LsCmd) runDescribe(ctx context.Context, b backend.Backend, ref backend.Ref) error {
	entries, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{
		Recursive:       c.Recursive,
		SkipTagFetch:    false,
		IncludeMetadata: true,
	})
	if err != nil {
		return fmt.Errorf("ls command failed: %w", err)
	}

	// Sort entries by path for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	result := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		m := make(map[string]any, len(entry.Metadata)+1)
		m["ref"] = string(ref.Type) + ":" + entry.Path
		for k, v := range entry.Metadata {
			m[k] = v
		}
		result = append(result, m)
	}

	return printJSON(c.out, result)
}
