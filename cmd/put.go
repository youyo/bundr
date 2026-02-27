package cmd

import (
	"context"
	"fmt"

	"github.com/youyo/bundr/internal/backend"
)

// PutCmd represents the "put" subcommand.
type PutCmd struct {
	Ref    string `arg:"" predictor:"ref" help:"Target ref (e.g. ps:/app/prod/DB_HOST, sm:secret-id)"`
	Value  string `short:"v" required:"" help:"Value to store"`
	Store  string `short:"s" default:"raw" enum:"raw,json" help:"Storage mode (raw|json)"`
	Secure bool   `help:"Use SecureString (SSM Parameter Store only)"`
}

// Run executes the put command.
func (c *PutCmd) Run(appCtx *Context) error {
	ref, err := backend.ParseRef(c.Ref)
	if err != nil {
		return fmt.Errorf("put command failed: invalid ref: %w", err)
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("put command failed: create backend: %w", err)
	}

	opts := backend.PutOptions{
		Value:     c.Value,
		StoreMode: c.Store,
	}

	if c.Secure {
		opts.ValueType = "secure"
	}

	if err := b.Put(context.Background(), c.Ref, opts); err != nil {
		return fmt.Errorf("put command failed: %w", err)
	}

	fmt.Println("OK")
	return nil
}
