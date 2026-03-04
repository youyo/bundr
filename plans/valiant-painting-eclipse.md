# Plan: bundr v1.0 再設計 — `sync` 追加・`export`/`jsonize` 廃止・`put --store` 廃止

## コンテキスト

`.env` ↔ AWS (PS/SM) の双方向同期を実現する `sync` コマンドを追加する。
これに伴い、`sync` で代替できる `export`・`jsonize` を廃止し、`put --store` も廃止してシンプルな設計に統一する。
既存ユーザーなしのため Breaking Change を許容する（v1.0 バンプ）。

---

## 設計

### `sync` コマンド

```
bundr sync --from <source> --to <dest>
```

**source / destination 形式：**

| 形式 | 例 | 説明 |
|------|----|------|
| ファイルパス | `.env`, `/path/.env.prod` | .env 形式のファイル |
| stdin/stdout | `-` | .env 形式で stdin 読み込み / stdout 出力 |
| PS パス | `ps:/app/config` | 単一パラメータ |
| PS prefix | `ps:/app/` | 末尾 `/` で prefix 扱い |
| SM | `sm:secret-id` | 単一シークレット |

**`--to` の型でデフォルト動作が変わる：**

| `--to` の型 | デフォルト動作 |
|------------|-------------|
| ファイル / `-` (stdout) | JSON値を展開して .env 形式で出力 |
| `ps:/path` (末尾なし) | JSON 一括保存 |
| `ps:/prefix/` (末尾 `/`) | 展開してフラット保存（各キーを個別パラメータに） |
| `sm:secret` | JSON 一括保存 |

**`--raw` フラグ：** `--to` がファイル/stdout のとき、展開せず値をそのまま出力する

---

### 全パターン早見表

```bash
# .env → PS（JSON一括）
bundr sync --from .env --to ps:/app/config
# → ps:/app/config = {"DB_HOST":"localhost","DB_PORT":"5432"}

# .env → PS（フラット）
bundr sync --from .env --to ps:/app/
# → ps:/app/db_host = localhost
# → ps:/app/db_port = 5432

# .env → SM（JSON一括）
bundr sync --from .env --to sm:myapp-prod
# → sm:myapp-prod = {"DB_HOST":"localhost","DB_PORT":"5432"}

# PS（JSON値）→ stdout（.env形式、展開）
bundr sync --from ps:/app/config --to -
# → DB_HOST=localhost

# PS prefix → stdout（.env形式）
bundr sync --from ps:/app/ --to -
# → DB_HOST=localhost

# PS（JSON値）→ stdout（展開しない）
bundr sync --from ps:/app/config --to - --raw
# → {"DB_HOST":"localhost"}

# PS → SM（そのままコピー）
bundr sync --from ps:/app/config --to sm:backup
# → sm:backup = {"DB_HOST":"localhost"}

# PS prefix → SM（収集してJSON一括）
bundr sync --from ps:/app/ --to sm:backup
# → sm:backup = {"db_host":"localhost","db_port":"5432"}

# stdin → PS
cat .env | bundr sync --from - --to ps:/app/config

# SM → .env ファイル
bundr sync --from sm:prod --to .env
```

---

### キー変換規則

| from → to | キー変換 |
|-----------|---------|
| `.env`/stdin → PS (フラット) | `DB_HOST` → `db_host`（小文字化）|
| `.env`/stdin → PS (JSON) / SM | キーそのまま（大文字） |
| PS prefix → `.env`/SM | パス末尾を大文字化（`db_host` → `DB_HOST`）|
| PS (JSON値) → `.env` | JSON キーそのまま（大文字のまま展開）|
| SM → `.env` | JSON キーそのまま |

---

### `get` コマンド拡張

```bash
# 既存動作（単一パラメータ）
bundr get ps:/app/db_host         → "localhost"
bundr get ps:/app/config          → '{"DB_HOST":"localhost"}'  （JSON文字列そのまま）

# 追加：prefix（末尾/）で収集してJSON化
bundr get ps:/app/                → '{"db_host":"localhost","db_port":"5432"}'
```

---

### `put` の `--store` 廃止

```bash
# 変更前
bundr put ps:/app/db_host --value "localhost" --store raw
bundr put ps:/app/config  --value '{"k":"v"}' --store json

# 変更後（--store 廃止、値はそのまま保存）
bundr put ps:/app/db_host --value "localhost"
bundr put ps:/app/config  --value '{"k":"v"}'
```

- `cli-store-mode` タグは内部的に `raw` で統一（後方互換のため `get` はタグを引き続き参照）
- `--secure`, `--tier` フラグは維持

---

### 廃止するコマンド・機能

| 廃止対象 | 代替 |
|---------|------|
| `bundr export` | `bundr sync --from ps:... --to -` または `--to .env` |
| `bundr jsonize` | `bundr sync --from .env --to ps:/...` または `get ps:/...` |
| `put --store` | フラグ削除（常にそのまま保存） |

---

## 実装ユニット

### ユニット 1: `internal/dotenv` パッケージ（独立）

**ファイル**: `internal/dotenv/dotenv.go`, `internal/dotenv/dotenv_test.go`

.env 形式の純粋なパーサー + ライター。AWS 依存なし。

```go
type Entry struct { Key, Value string }

func Parse(r io.Reader) ([]Entry, error)    // コメント・空行スキップ、クォート対応
func Write(w io.Writer, entries []Entry) error  // KEY=VALUE 形式で出力
```

テストケース：
- コメント行・空行スキップ
- `KEY='value'`, `KEY="value"`, `KEY=value` の3形式
- 等号を含む値（`KEY=a=b` → value が `a=b`）

---

### ユニット 2: `cmd/sync.go` + `cmd/root.go` 修正（`internal/dotenv` に依存）

**ファイル**: `cmd/sync.go`, `cmd/sync_test.go`, `cmd/root.go`

`SyncCmd` の実装。

```go
type SyncCmd struct {
    From string `required:"" short:"f" help:"Source: file path, -, ps:/path, ps:/prefix/, sm:id"`
    To   string `required:"" short:"t" help:"Destination: file path, -, ps:/path, ps:/prefix/, sm:id"`
    Raw  bool   `help:"Output raw value without expanding JSON (only for file/stdout destination)"`
}
```

**実装ロジック**:

1. `from` の種類判定 → データ取得 → `[]Entry`
   - ファイル / `-`: `dotenv.Parse()`
   - `ps:/path`（末尾なし）: `Backend.Get()` → JSON なら展開、scalar はそのまま
   - `ps:/prefix/`（末尾 `/`）: `Backend.GetByPrefix()` → 各エントリを収集
   - `sm:`: `Backend.Get()` → JSON パース

2. `to` の種類判定 → データ書き込み
   - ファイル / `-`: `dotenv.Write()`（`--raw` なければ JSON 展開）
   - `ps:/path`: `json.Marshal(data)` → `Backend.Put({StoreMode: json})`
   - `ps:/prefix/`: 各エントリを `Backend.Put()` でループ（キー小文字化）
   - `sm:`: `json.Marshal(data)` → `Backend.Put({StoreMode: json})`

`cmd/root.go` に追加:
```go
Sync SyncCmd `cmd:"" help:"Sync parameters between .env, ps:, and sm:"`
```

テストケース（MockBackend）：
- `.env` → PS JSON一括
- `.env` → PS フラット（末尾 `/`）
- `.env` → SM
- PS JSON値 → stdout（展開）
- PS JSON値 → stdout `--raw`（展開しない）
- PS prefix → stdout
- PS → SM（コピー）

---

### ユニット 3: `get` の prefix 対応

**ファイル**: `cmd/get.go`, `cmd/get_test.go`

`get` に末尾 `/` の prefix 引数を追加対応。

```bash
bundr get ps:/app/    # → {"db_host":"localhost","db_port":"5432"}
```

- `ref.Path` が `/` で終わる → `Backend.GetByPrefix()` + JSON 収集
- それ以外は既存動作

---

### ユニット 4: `put` の `--store` 廃止

**ファイル**: `cmd/put.go`, `cmd/put_test.go`

`--store` フラグを削除し、常に raw モードで保存。

```go
// 変更前
type PutCmd struct {
    ...
    Store  string `default:"raw" enum:"raw,json" ...`
    ...
}

// 変更後
type PutCmd struct {
    ...
    // Store フィールド削除
    ...
}
```

内部: `opts.StoreMode = tags.StoreModeRaw` で固定。

---

### ユニット 5: `export` / `jsonize` 廃止

**ファイル**: `cmd/export.go`, `cmd/export_test.go`, `cmd/jsonize.go`, `cmd/jsonize_test.go`, `cmd/root.go`

- `export.go`, `export_test.go` を削除
- `jsonize.go`, `jsonize_test.go` を削除
- `cmd/root.go` から `Export`, `Jsonize` フィールドを削除
- `buildVars()` が `sync.go` 側に移動（または `export.go` 削除後に `sync.go` に実装）

---

### バージョン

- `main.go`: `v0.6.1` → `v1.0.0`

---

## e2e テストレシピ

AWS 不要。ユニットテストで検証。

```bash
# ビルド確認
go build -o /tmp/bundr ./...

# 全テスト
go test ./...

# help 確認
/tmp/bundr --help
/tmp/bundr sync --help
/tmp/bundr get --help

# stdin → stdout の動作確認（ビルドのみ、AWS不要）
echo "DB_HOST=localhost\nDB_PORT=5432" | /tmp/bundr sync --from - --to - --raw
# → DB_HOST=localhost
# → DB_PORT=5432
```

---

## 実装メモ

- `internal/dotenv.Write()` は `io.Writer` を受け取る（`os.Stdout` / `*os.File` 両対応）
- `isFileRef(s string) bool`: `ps:`/`sm:` で始まらず `-` でもない → ファイルパス
- `isPrefix(s string) bool`: 末尾が `/` → prefix 扱い
- `sync.go` で `buildVars()` は不要（`GetByPrefix` を直接呼ぶ）
- `put` の `--store` 廃止後、`cli-store-mode=raw` タグが全件に付くため `get`/`export` の既存動作は変わらない
- ユニット1（dotenv パッケージ）が他の全ユニットのブロッカー。worktree 並行では dotenv を先にマージするか、各ユニットが自分で dotenv を仮実装してマージ時に差し替える
