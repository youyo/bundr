package main

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/alecthomas/kong"
	"github.com/youyo/bundr/cmd"
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
)

func main() {
	cli := cmd.CLI{}
	kctx := kong.Parse(&cli)

	cfg, err := config.Load()
	if err != nil {
		kctx.FatalIfErrorf(err)
	}

	err = kctx.Run(&cmd.Context{
		Config:         cfg,
		BackendFactory: newBackendFactory(cfg),
	})
	kctx.FatalIfErrorf(err)
}

// newBackendFactory returns a BackendFactory that creates real AWS backends.
func newBackendFactory(cfg *config.Config) func(backend.BackendType) (backend.Backend, error) {
	return func(bt backend.BackendType) (backend.Backend, error) {
		opts := []func(*awsconfig.LoadOptions) error{}
		if cfg.AWS.Region != "" {
			opts = append(opts, awsconfig.WithRegion(cfg.AWS.Region))
		}
		if cfg.AWS.Profile != "" {
			opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.AWS.Profile))
		}

		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			return nil, fmt.Errorf("load AWS config: %w", err)
		}

		switch bt {
		case backend.BackendTypePS, backend.BackendTypePSA:
			return backend.NewPSBackend(ssm.NewFromConfig(awsCfg)), nil
		case backend.BackendTypeSM:
			return backend.NewSMBackend(secretsmanager.NewFromConfig(awsCfg)), nil
		default:
			return nil, fmt.Errorf("unsupported backend type: %s", bt)
		}
	}
}
