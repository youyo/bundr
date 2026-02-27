package cmd

import (
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
	"github.com/youyo/bundr/internal/config"
)

// CLI is the Kong root command structure.
type CLI struct {
	Region   string `help:"AWS region (overrides all other region settings)" optional:"" name:"region"`
	Profile  string `help:"AWS profile (overrides all other profile settings)" optional:"" name:"profile"`
	KMSKeyID string `help:"KMS key ID or ARN for encryption" env:"BUNDR_KMS_KEY_ID" optional:"" name:"kms-key-id"`

	Put        PutCmd        `cmd:"" help:"Store a value to AWS Parameter Store or Secrets Manager."`
	Get        GetCmd        `cmd:"" help:"Get a value from a backend."`
	Export     ExportCmd     `cmd:"" help:"Export parameters as environment variables."`
	Ls         LsCmd         `cmd:"" help:"List parameter paths."`
	Exec       ExecCmd       `cmd:"" help:"Execute a command with parameters as environment variables."`
	Completion CompletionCmd `cmd:"" help:"Output shell completion script."`
	Jsonize    JsonizeCmd    `cmd:"" help:"Build a nested JSON from parameter prefix and store it."`
	Cache      CacheCmd      `cmd:"" help:"Manage local completion cache."`
}

// BGLauncher はバックグラウンド更新プロセスの起動を抽象化する。
// main.go では ExecBGLauncher を注入。テスト時は MockBGLauncher を差し替え。
type BGLauncher interface {
	Launch(args ...string) error
}

// ExecBGLauncher は exec.Command(os.Args[0], args...).Start() で実装する本番用 BGLauncher。
type ExecBGLauncher struct{}

// Launch はバックグラウンドプロセスを起動する（fire-and-forget）。
// os.Args[0] を実行ファイルとして使用し、COMP_* 環境変数をフィルタリングして
// 子プロセスが補完処理として即座に終了しないようにする。
func (l *ExecBGLauncher) Launch(args ...string) error {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	// COMP_* 環境変数をフィルタリングして子プロセスに継承させない
	env := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "COMP_") {
			env = append(env, e)
		}
	}
	cmd.Env = env
	return cmd.Start()
}

// Context holds shared dependencies injected into all commands.
type Context struct {
	Config *config.Config
	// BackendFactory creates a Backend for the given ref type.
	BackendFactory func(refType backend.BackendType) (backend.Backend, error)
	// CacheStore はキャッシュ操作のインターフェース（テスト時は MockStore を差し替え）。
	CacheStore cache.Store
	// BGLauncher はバックグラウンド更新プロセスの起動（テスト時は MockBGLauncher を差し替え）。
	BGLauncher BGLauncher
}
