# Bundr — アーキテクチャ

## ディレクトリ構造
```
bundr/
├── main.go                    # エントリーポイント + BackendFactory
├── go.mod / go.sum
├── CLAUDE.md                  # プロジェクト開発ガイド
├── cmd/
│   ├── root.go               # Kong CLI 構造体 + Context + BackendFactory interface
│   ├── put.go / put_test.go  # put コマンド (M1 完了)
│   └── get.go / get_test.go  # get コマンド (M1 完了)
└── internal/
    ├── backend/
    │   ├── interface.go      # Backend interface (Put/Get/GetByPrefix予定)
    │   ├── ref.go            # Ref パース (ps:, psa:, sm:)
    │   ├── ps.go             # SSM Parameter Store 実装
    │   ├── sm.go             # Secrets Manager 実装
    │   └── mock.go           # テスト用 MockBackend
    ├── tags/
    │   └── tags.go           # タグ定数 (TagCLI, TagStoreMode, TagFlatten...)
    └── config/
        └── config.go         # Viper 設定管理 (TOML + env vars)
```

## 主要な依存ライブラリ
- `github.com/alecthomas/kong` — 宣言的 CLI パーサー
- `github.com/spf13/viper` — TOML + 環境変数設定管理
- `github.com/aws/aws-sdk-go-v2/service/ssm` — SSM Parameter Store
- `github.com/aws/aws-sdk-go-v2/service/secretsmanager` — Secrets Manager

## 設計パターン

### BackendFactory パターン
```go
// Context に BackendFactory を注入してテスト可能にする
type Context struct {
    Config         *config.Config
    BackendFactory func(refType backend.BackendType) (backend.Backend, error)
}
```

### Kong コマンド実装パターン
```go
type PutCmd struct {
    Ref   string `arg:""`
    Value string `short:"v" required:""`
    Store string `default:"raw" enum:"raw,json"`
}
func (c *PutCmd) Run(appCtx *Context) error { ... }
```

### Backend interface
```go
type Backend interface {
    Put(ctx context.Context, ref string, opts PutOptions) error
    Get(ctx context.Context, ref string, opts GetOptions) (string, error)
    // M2 で追加予定:
    // GetByPrefix(ctx context.Context, prefix string) (map[string]string, error)
}
```

### AWS SDK モック
- SSMClient / smClient インターフェースを独自定義
- テスト時に mockSSMClient を注入して AWS 呼び出しを隔離

## 設定優先度
CLI flags > env vars > .bundr.toml > ~/.config/bundr/config.toml

## エラーハンドリング規約
```go
// コンテキストを付加したエラーラッピング
return fmt.Errorf("put command failed: %w", err)
return fmt.Errorf("get command failed: invalid ref: %w", err)
```
