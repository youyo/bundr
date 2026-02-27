package cmd

import (
	"context"
	"fmt"

	"github.com/youyo/bundr/internal/backend"
)

// GetCmd represents the "get" subcommand.
type GetCmd struct {
	Ref  string `arg:"" help:"Target ref (e.g. ps:/app/prod/DB_HOST, sm:secret-id)"`
	Raw  bool   `help:"Force raw output (ignore cli-store-mode tag)"`
	JSON bool   `name:"json" help:"Force JSON decode output"`
}

// Run executes the get command.
func (c *GetCmd) Run(appCtx *Context) error {
	ref, err := backend.ParseRef(c.Ref)
	if err != nil {
		return fmt.Errorf("get command failed: invalid ref: %w", err)
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("get command failed: create backend: %w", err)
	}

	opts := backend.GetOptions{
		ForceRaw:  c.Raw,
		ForceJSON: c.JSON,
	}

	val, err := b.Get(context.Background(), c.Ref, opts)
	if err != nil {
		return fmt.Errorf("get command failed: %w", err)
	}

	fmt.Println(val)
	return nil
}
