# Plan: bundr sync --format export 実装

## Context

`bundr sync -f ps:/prefix/ -t -` の stdout 出力は `KEY=VALUE\n`（dotenv 形式）のみで、
`eval $(bundr sync ...)` で現在シェルに環境変数を設定できない。

旧 `export` コマンドは `export KEY=VALUE\n` 形式を出力していたが v0.7.0 で削除されたため
このユースケースが失われた。`--format export` フラグで復活させる。

## 変更スコープ

| ファイル | 変更内容 |
|----------|---------|
| `internal/dotenv/dotenv.go` | `WriteExport()` 関数追加 |
| `cmd/sync.go` | `Format string` フィールド追加・`writeEntries` 分岐 |
| `cmd/sync_test.go` | `--format export` ケース追加 |

## ユニット分解（1ユニット）

変更量が小さく dotenv → sync の依存関係があるため、1ワーカーで全ファイルを実装する。

### Unit 1: --format export 全実装

**対象ファイル:**
- `internal/dotenv/dotenv.go`
- `internal/dotenv/dotenv_test.go`（存在する場合）
- `cmd/sync.go`
- `cmd/sync_test.go`

**変更詳細:**

#### Step 1: dotenv.WriteExport 追加（internal/dotenv/dotenv.go）

```go
// WriteExport outputs entries in "export KEY=VALUE" format to w.
// Suitable for use with eval: eval $(bundr sync -f ... -t -)
func WriteExport(w io.Writer, entries []Entry) error {
    for _, e := range entries {
        if _, err := fmt.Fprintf(w, "export %s=%s\n", e.Key, e.Value); err != nil {
            return err
        }
    }
    return nil
}
```

#### Step 2: SyncCmd に Format フィールド追加（cmd/sync.go）

```go
type SyncCmd struct {
    From   string `required:"" short:"f" help:"Source: file path, -, ps:/path, ps:/prefix/, sm:id"`
    To     string `required:"" short:"t" help:"Destination: file path, -, ps:/path, ps:/prefix/, sm:id"`
    Raw    bool   `help:"Output raw value without expanding JSON (only for file/stdout destination)"`
    Format string `default:"dotenv" enum:"dotenv,export" help:"Output format for file/stdout destination (dotenv or export)"`
}
```

#### Step 3: writeEntries の書き込みを Format で分岐（cmd/sync.go）

現在のファイル/stdout 書き込み部分（行 149-167）で `dotenv.Write` を `writeFn` に統一:

```go
writeFn := dotenv.Write
if c.Format == "export" {
    writeFn = dotenv.WriteExport
}

if c.Raw {
    return writeFn(w, entries)
}

// Non-raw: expand JSON values
var expanded []dotenv.Entry
for _, e := range entries {
    var obj map[string]any
    if err := json.Unmarshal([]byte(e.Value), &obj); err == nil {
        for k, v := range obj {
            expanded = append(expanded, dotenv.Entry{Key: k, Value: fmt.Sprintf("%v", v)})
        }
    } else {
        expanded = append(expanded, e)
    }
}
sortEntries(expanded)
return writeFn(w, expanded)
```

## e2e テストレシピ

```bash
# ビルド
go build -o /tmp/bundr-test ./...

# テスト用 .env 作成
printf "DB_HOST=localhost\nDB_PORT=5432\nAPP_KEY=secret\n" > /tmp/test.env

# 確認1: dotenv 形式（デフォルト・既存動作）
/tmp/bundr-test sync -f /tmp/test.env -t -
# 期待: APP_KEY=secret  DB_HOST=localhost  DB_PORT=5432（順不同の場合あり）

# 確認2: export 形式
/tmp/bundr-test sync -f /tmp/test.env -t - --format export
# 期待: export APP_KEY=secret  export DB_HOST=localhost  export DB_PORT=5432

# 確認3: eval で環境変数設定（メイン機能）
eval $(/tmp/bundr-test sync -f /tmp/test.env -t - --format export)
echo "DB_HOST=$DB_HOST"  # → "DB_HOST=localhost"
echo "DB_PORT=$DB_PORT"  # → "DB_PORT=5432"

# ユニットテスト
go test ./...
```

## 注意事項

- `--format` は `--to` がファイルまたは stdout（`-`）の場合のみ有効。バックエンド ref が `to` の場合は出力なし（無視）
- 値にスペースや特殊文字が含まれる場合のクォート処理は今回スコープ外（既存の dotenv.Write と同様）
- テストでの stdout キャプチャは `os.Pipe()` パターンを使う（sync_test.go の既存パターンを踏襲）
