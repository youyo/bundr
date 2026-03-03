package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/alecthomas/kong"
	"github.com/willabides/kongplete"
	"github.com/youyo/bundr/cmd"
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
	"github.com/youyo/bundr/internal/config"
)

var version = "0.6.0" // goreleaser ldflags で上書き（-X main.version=...）

func main() {
	// 1. 設定ロード
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. CacheStore 構築（失敗時は補完を無効にして通常 CLI を継続）
	var cacheStore cache.Store
	identifier := cache.CacheIdentifier(cfg.AWS.Profile)
	if fs, fsErr := cache.NewFileStore(cfg.AWS.Region, identifier); fsErr != nil {
		fmt.Fprintf(os.Stderr, "warning: cache init failed (completion disabled): %v\n", fsErr)
		cacheStore = cache.NewNoopStore()
	} else {
		cacheStore = fs
	}

	// 3. BGLauncher 構築
	bgLauncher := &cmd.ExecBGLauncher{}

	// 3.5. 補完用 BackendFactory を事前構築（CLIフラグ反映前の cfg を使用）
	completionFactory := cmd.BackendFactory(newBackendFactory(cfg))

	// 4. Kong パーサー構築
	cli := cmd.CLI{}
	parser := kong.Must(&cli,
		kong.Name("bundr"),
	)

	// 5. kongplete で補完リクエストを処理
	kongplete.Complete(parser,
		kongplete.WithPredictor("ref", cmd.NewRefPredictor(cacheStore, bgLauncher, completionFactory)),
		kongplete.WithPredictor("prefix", cmd.NewPrefixPredictor(cacheStore, bgLauncher, completionFactory)),
	)

	// 6. --version フラグを早期チェック（Parse 前に処理）
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("bundr %s\n", version)
		os.Exit(0)
	}
	args := os.Args[1:]

	// 7. 通常コマンドを解析（cli.Region 等が確定する）
	// 引数なし実行時はヘルプを表示
	if len(args) == 0 {
		args = []string{"--help"}
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	// 8. CLI フラグで設定をオーバーライド（CLI flags > env vars > TOML）
	config.ApplyCLIOverrides(cfg, cli.Region, cli.Profile, cli.KMSKeyID)

	// 9. BackendFactory 構築（最終的な cfg で）
	factory := newBackendFactory(cfg)

	// 10. コマンドを実行
	err = kctx.Run(&cmd.Context{
		Config:         cfg,
		BackendFactory: factory,
		CacheStore:     cacheStore,
		BGLauncher:     bgLauncher,
	})
	if err != nil {
		var exitErr *cmd.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
		os.Exit(1)
	}
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
		case backend.BackendTypePS:
			// psa: refs are normalized to BackendTypePS by ParseRef
			return backend.NewPSBackend(ssm.NewFromConfig(awsCfg)), nil
		case backend.BackendTypeSM:
			return backend.NewSMBackend(secretsmanager.NewFromConfig(awsCfg)), nil
		default:
			return nil, fmt.Errorf("unsupported backend type: %s", bt)
		}
	}
}
