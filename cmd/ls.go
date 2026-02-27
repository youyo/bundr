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
	From        string `arg:"" required:"" predictor:"prefix" help:"Source prefix (e.g. ps:/app/prod/)"`
	NoRecursive bool   `name:"no-recursive" help:"List only direct children"`

	out io.Writer // for testing; nil means os.Stdout
}

// Run executes the ls command.
func (c *LsCmd) Run(appCtx *Context) error {
	if c.out == nil {
		c.out = os.Stdout
	}

	ref, err := backend.ParseRef(c.From)
	if err != nil {
		return fmt.Errorf("ls command failed: invalid ref: %w", err)
	}

	if ref.Type == backend.BackendTypeSM {
		return fmt.Errorf("ls command failed: sm: backend is not supported (use ps: or psa:)")
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("ls command failed: create backend: %w", err)
	}

	entries, err := b.GetByPrefix(context.Background(), ref.Path, backend.GetByPrefixOptions{Recursive: !c.NoRecursive})
	if err != nil {
		return fmt.Errorf("ls command failed: %w", err)
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
