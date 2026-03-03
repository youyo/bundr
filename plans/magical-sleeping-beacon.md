# Plan: `bundr exec --env-file` — .env ファイル内の bundr ref 解決

**作成日**: 2026-03-03
**担当**: implementation phase

## Context

1Password CLI の `op run --env-file .env` のように、`.env` ファイル内に
`KEY=ps:/path/to/param` 形式で bundr ref を記述し、`bundr exec` 実行時に
ref を解決して環境変数として注入できるようにする。

既存の `--from ps:/prefix/` はプレフィックス全体を展開するが、本機能は
個々のパラメータを明示的に特定の環境変数名にマッピングする点で補完的。

**SM ref (`sm:`) は `--from` では使えないが、`--env-file` では使用可**。
理由: `--from` は `GetByPrefix` を使うため SM はサポート外だが、
`--env-file` は個別の `Get()` 呼び出しのため SM ref も自然に動作する。

## 使用例

```bash
# 基本形
bundr exec --env-file .env.bundr -- node server.js

# --from との併用（env-file が後者優先）
bundr exec --from ps:/app/ --env-file .env.bundr -- cmd

# 複数 env-file（後から指定したファイルが優先）
bundr exec --env-file base.env --env-file override.env -- cmd
```

```
# .env.bundr のフォーマット
# コメント行はスキップ
DB_PASSWORD=ps:/myapp/db-password
API_KEY=sm:myapp/api-key
PORT=8080              # bundr ref でない通常値はそのまま使用
EMPTY=                 # 空値はそのまま空文字列
```

## 変更ファイル

| ファイル | 変更種別 | 内容 |
|---------|---------|------|
| `cmd/envfile.go` | 新規作成 | `parseEnvFile`, `isBundrRef`, `resolveEnvFile` |
| `cmd/envfile_test.go` | 新規作成 | 上記3関数の単体テスト |
| `cmd/exec.go` | 変更 | `ExecCmd` に `EnvFile []string` フラグ追加、`Run()` に解決ループ追加 |
| `cmd/exec_test.go` | 変更 | `--env-file` の統合テスト追加 |

## 実装詳細

### Step 1: `cmd/envfile.go` 新規作成 (Red → Green)

```go
package cmd

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/youyo/bundr/internal/backend"
)

// parseEnvFile は .env 形式ファイルを読み込み KEY→VALUE マップを返す。
// - '#' で始まる行・空行はスキップ
// - インラインコメントは非対応（値はそのまま使用）
// - KEY= は空文字列として許可
// - '=' のない行は行番号付きエラー
func parseEnvFile(filePath string) (map[string]string, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("open env file %q: %w", filePath, err)
    }
    defer f.Close()

    result := make(map[string]string)
    scanner := bufio.NewScanner(f)
    lineNum := 0
    for scanner.Scan() {
        lineNum++
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        idx := strings.IndexByte(line, '=')
        if idx < 0 {
            return nil, fmt.Errorf("env file %q line %d: missing '=' in %q", filePath, lineNum, line)
        }
        key := strings.TrimSpace(line[:idx])
        value := line[idx+1:]  // 値はトリムしない（パスワード等の先頭空白を保持）
        if key == "" {
            return nil, fmt.Errorf("env file %q line %d: empty key", filePath, lineNum)
        }
        result[key] = value
    }
    return result, scanner.Err()
}

// isBundrRef は value が ps:, psa:, sm: のいずれかで始まる bundr ref かどうかを返す。
func isBundrRef(value string) bool {
    return strings.HasPrefix(value, "ps:") ||
        strings.HasPrefix(value, "psa:") ||
        strings.HasPrefix(value, "sm:")
}

// resolveEnvFile は .env ファイルをパースし、bundr ref を解決して
// map[KEY]VALUE を返す。ref でない値はそのまま使用。
func resolveEnvFile(ctx context.Context, filePath string, factory BackendFactory) (map[string]string, error) {
    raw, err := parseEnvFile(filePath)
    if err != nil {
        return nil, err
    }

    result := make(map[string]string, len(raw))
    for key, value := range raw {
        if !isBundrRef(value) {
            result[key] = value
            continue
        }
        ref, err := backend.ParseRef(value)
        if err != nil {
            return nil, fmt.Errorf("resolve ref %q for key %q: %w", value, key, err)
        }
        b, err := factory(ref.Type)
        if err != nil {
            return nil, fmt.Errorf("create backend for %q: %w", value, err)
        }
        resolved, err := b.Get(ctx, value, backend.GetOptions{})
        if err != nil {
            return nil, fmt.Errorf("resolve ref %q (key %q): %w", value, key, err)
        }
        result[key] = resolved
    }
    return result, nil
}
```

### Step 2: `cmd/envfile_test.go` 新規作成 (Red フェーズ)

テストケース一覧:

```
TestParseEnvFile
  P-01: KEY=VALUE を正しくパース
  P-02: # コメント行をスキップ
  P-03: 空行をスキップ
  P-04: KEY= (空値) を空文字列として扱う
  P-05: ファイルが存在しない → エラー（os.Open 失敗）
  P-06: '=' のない行 → 行番号付きエラー
  P-07: 空キー（=VALUE）→ エラー

TestIsBundrRef
  B-01: "ps:/path/to/key" → true
  B-02: "psa:/path/to/key" → true
  B-03: "sm:secret-name" → true
  B-04: "8080" → false
  B-05: "" → false
  B-06: "https://example.com" → false
  B-07: "ps:" のみ（パスなし）→ true（ref 扱い。Get 時にエラーになる）

TestResolveEnvFile
  R-01: 通常値はそのまま返す
  R-02: ps: ref を解決して値を返す
  R-03: sm: ref を解決して値を返す（--from と異なり sm: が使える）
  R-04: 通常値と ref 混在を正しく解決する
  R-05: ファイルが存在しない → エラー
  R-06: ref が解決できない（key not found）→ キー名を含むエラーメッセージ
```

テストでは `t.TempDir()` + `os.WriteFile` で一時 `.env` ファイルを作成し、
`MockBackend` で AWS 呼び出しを差し替える。

### Step 3: `cmd/exec.go` 変更

`ExecCmd` struct に `EnvFile []string` フラグを追加:

```go
type ExecCmd struct {
    From           []string `short:"f" name:"from" optional:"" predictor:"prefix" help:"..."`
    EnvFile        []string `name:"env-file" optional:"" help:"Path to .env file with bundr refs; repeatable, later files take precedence"`
    // ... 既存フラグ ...
}
```

`Run()` に `--env-file` 処理ループを追加（`--from` の直後、環境変数マージの前）:

```go
// Process --env-file (resolves bundr refs); later files take precedence over --from.
for _, filePath := range c.EnvFile {
    fileVars, err := resolveEnvFile(context.Background(), filePath, appCtx.BackendFactory)
    if err != nil {
        return fmt.Errorf("exec command failed: %w", err)
    }
    for k, v := range fileVars {
        vars[k] = v
    }
}
```

### Step 4: `cmd/exec_test.go` 変更

`TestExecCmd_EnvFile` テーブルを追加（既存の `setupExecCmd` の opts パターンを流用）:

```
EF-01: --env-file で通常値を環境変数に注入
EF-02: --env-file で ps: ref を解決して注入
EF-03: --env-file で sm: ref を解決して注入
EF-04: --from と --env-file 併用（env-file が後者優先）
EF-05: 複数 --env-file（後から指定のファイルが優先）
EF-06: --env-file が存在しないファイル → エラー
EF-07: --env-file 内の ref が解決不能 → エラー（キー名を含む）
```

既存 `setupExecCmd` の `opts ...func(*ExecCmd)` パターンで
`func(c *ExecCmd) { c.EnvFile = []string{filePath} }` として設定する。

## 環境変数マージ優先順序

```
os.Environ()                    ← ベース（システム環境変数）
  + --from 展開した変数          ← 後者 --from が優先
  + --env-file 解決した変数      ← --from より後者として優先（最終的に勝つ）
```

`vars` マップに逐次上書きし、最後に `os.Environ()` へ `append` する
既存パターンをそのまま踏襲する。

## エラーハンドリング

| 状況 | 挙動 |
|------|------|
| ファイルが存在しない | `open env file "...": no such file or directory` |
| `=` のない行 | `env file "..." line N: missing '=' in "..."` |
| ref が解決できない | `resolve ref "ps:/..." (key "DB_PASSWORD"): ...` |
| backend 生成失敗 | `create backend for "ps:/...": ...` |

すべてのエラーは `fmt.Errorf("exec command failed: %w", err)` でラップして返す。

## TDD 実装順序

1. **Red**: `cmd/envfile_test.go` を書く（コンパイルエラー状態）
2. **Green**: `cmd/envfile.go` を実装してテストを通す
3. **Red**: `cmd/exec_test.go` に `TestExecCmd_EnvFile` を追加
4. **Green**: `cmd/exec.go` に `EnvFile` フラグと処理ループを追加
5. **Refactor**: テストがグリーンのまま整理（過剰な複雑さがあれば）

## 検証方法

```bash
# 単体テスト
go test ./cmd/... -run TestParseEnvFile -v
go test ./cmd/... -run TestIsBundrRef -v
go test ./cmd/... -run TestResolveEnvFile -v
go test ./cmd/... -run TestExecCmd_EnvFile -v

# 全テスト（既存の回帰も確認）
go test ./...

# ビルド確認
go build ./...

# (実環境) e2e
echo "DB_PASS=ps:/test/db-pass" > /tmp/test.env
bundr exec --env-file /tmp/test.env -- printenv DB_PASS
```

## 非対応事項（スコープ外）

- `export --env-file`: 必要なら将来追加。今回は `exec` のみ
- YAML/TOML 形式: `.env` 形式（KEY=VALUE）のみ
- インラインコメント（`KEY=VALUE # comment`）: 値として扱う（後続の検討課題）
- `exec` 以外のコマンド（`export`, `run` 等）への波及: 今回はなし
