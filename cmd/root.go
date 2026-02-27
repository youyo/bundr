package cmd

import (
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
)

// CLI is the Kong root command structure.
type CLI struct {
	Put    PutCmd    `cmd:"" help:"Store a value to AWS Parameter Store or Secrets Manager."`
	Get    GetCmd    `cmd:"" help:"Get a value from a backend."`
	Export ExportCmd `cmd:"" help:"Export parameters as environment variables."`
}

// Context holds shared dependencies injected into all commands.
type Context struct {
	Config *config.Config
	// BackendFactory creates a Backend for the given ref type.
	BackendFactory func(refType backend.BackendType) (backend.Backend, error)
}
