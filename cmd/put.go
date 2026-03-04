package cmd

import (
	"context"
	"fmt"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/tags"
)

// PutCmd represents the "put" subcommand.
type PutCmd struct {
	Ref    string `arg:"" predictor:"ref" help:"Target ref (e.g. ps:/app/prod/DB_HOST, sm:secret-id)"`
	Value  string `short:"v" required:"" help:"Value to store"`
	Secure bool   `help:"Use SecureString (SSM Parameter Store only)"`
	Tier   string `help:"Parameter Store tier override (standard|advanced). Omit to auto-detect existing tier." enum:"standard,advanced," default:""`
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
		StoreMode: tags.StoreModeRaw,
	}

	if c.Secure {
		opts.ValueType = backend.ValueTypeSecure
	}

	switch c.Tier {
	case "advanced":
		opts.AdvancedTier = true
		opts.TierExplicit = true
	case "standard":
		opts.AdvancedTier = false
		opts.TierExplicit = true
	}

	if err := b.Put(context.Background(), c.Ref, opts); err != nil {
		return fmt.Errorf("put command failed: %w", err)
	}

	fmt.Println("OK")
	return nil
}
