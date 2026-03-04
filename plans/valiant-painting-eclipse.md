# Plan: bundr v0.7.0 ドキュメント全面更新

## コンテキスト

v0.7.0 リリースで以下の Breaking Changes が入った：
- `sync` コマンド新規追加（.env ↔ PS/SM 双方向同期）
- `export` / `jsonize` コマンド廃止
- `put --store` フラグ廃止（常に raw 保存）
- `get` の prefix 対応（末尾 `/` で GetByPrefix → JSON 出力）

これらを README および関連ドキュメントに反映する。
現状: README は 605 行・export/jsonize/--store が多数記載・sync が未記載。

---

## 調査サマリー

更新が必要なファイル：

| ファイル | 優先度 | 主な変更 |
|---------|--------|---------|
| `README.md` | 🔴 最重要 | sync 追加、export/jsonize/--store 削除、get prefix 追加 |
| `CLAUDE.md` | 🔴 高 | ディレクトリ構造・Milestones・コマンドリファレンス更新 |
| `docs/specs/bundr-spec-v1.1.md` | 🟡 中 | psa: 削除、jsonize 削除、sync 仕様追加 |
| `docs/configuration.md` | 🟡 中 | sync コマンド追加 |
| `plans/bundr-roadmap.md` | 🟢 低 | M6/M7 完了・sync 追記 |

---

## 作業ユニット（5 つ）

### Unit 0: lint 修正 + v0.7.1 バンプ（ブロッカー）

**ファイル**: `cmd/sync_test.go`, `main.go`

**lint エラー（errcheck）**:
- `sync_test.go:130` — `buf.ReadFrom(r)` 戻り値未チェック
- `sync_test.go:164` — `buf.ReadFrom(r)` 戻り値未チェック
- `sync_test.go:198` — `buf.ReadFrom(r)` 戻り値未チェック
- `sync_test.go:247` — `w.WriteString(...)` 戻り値未チェック

**修正方法**: `buf.ReadFrom(r)` を `io.ReadAll(r)` + エラーチェックに置き換え。`w.WriteString(...)` はエラーチェック追加。

```go
// 修正前
var buf bytes.Buffer
buf.ReadFrom(r)
output := buf.String()

// 修正後
out, err := io.ReadAll(r)
if err != nil {
    t.Fatalf("failed to read: %v", err)
}
output := string(out)
```

```go
// 修正前
w.WriteString("KEY1=val1\nKEY2=val2\n")

// 修正後
if _, err := w.WriteString("KEY1=val1\nKEY2=val2\n"); err != nil {
    t.Fatalf("failed to write: %v", err)
}
```

**バージョンバンプ**: `main.go` の `version = "0.7.0"` → `"0.7.1"`

---

### Unit 1: README.md 全面更新

**ファイル**: `README.md`

**変更内容**:

1. **What it does** — export/jsonize の箇条書きを削除、sync 追加
2. **Quick start** — `--store raw` 削除、`bundr export` を `bundr sync` に変更
3. **Recipes** — `export` セクション削除、`jsonize` セクション削除、`sync` セクション新規追加、`put` から `--store` 例を削除、`get` に prefix 例を追加
4. **Command reference**
   - `bundr put`: `--store` フラグ行を削除
   - `bundr get`: `--describe` フラグ追加、prefix 使用法追加
   - `bundr export` セクション削除
   - `bundr jsonize` セクション削除
   - `bundr sync` セクション新規追加
5. **Advanced topics** — `JSON flattening` と `Array handling modes` セクションを削除（export 廃止に伴い不要）
6. その他の `--store` 言及をすべて除去

**sync Recipes セクション（追加内容）**:
```bash
# .env → PS（JSON一括）
bundr sync --from .env --to ps:/app/config
# → ps:/app/config = {"DB_HOST":"localhost","DB_PORT":"5432"}

# .env → PS（フラット展開）
bundr sync --from .env --to ps:/app/
# → ps:/app/db_host = localhost
# → ps:/app/db_port = 5432

# .env → SM（JSON一括）
bundr sync --from .env --to sm:myapp-prod

# PS（JSON値）→ stdout（.env 形式に展開）
bundr sync --from ps:/app/config --to -
# → DB_HOST=localhost

# PS prefix → stdout（.env 形式）
bundr sync --from ps:/app/ --to -

# 値をそのまま出力（展開しない）
bundr sync --from ps:/app/config --to - --raw
# → {"DB_HOST":"localhost"}

# PS → SM（コピー）
bundr sync --from ps:/app/config --to sm:backup

# stdin → PS
cat .env | bundr sync --from - --to ps:/app/config

# SM → .env ファイルに書き出し
bundr sync --from sm:prod --to .env
```

**bundr sync Command reference（追加）**:
```
bundr sync -f <source> -t <dest> [--raw]

  -f, --from  Source (file path, -, ps:/path, ps:/prefix/, sm:id)
  -t, --to    Destination (file path, -, ps:/path, ps:/prefix/, sm:id)
      --raw   Output raw value without expanding JSON (file/stdout only)

--to の末尾 / の有無でストレージ方式が変わる:
  ps:/path    → JSON一括保存
  ps:/prefix/ → キー小文字化してフラット展開（各キーを個別パラメータに）
  sm:id       → JSON一括保存
```

---

### Unit 2: CLAUDE.md 更新

**ファイル**: `CLAUDE.md`（プロジェクトルート）

**変更内容**:

1. **Directory Structure** セクション
   - `export.go`, `jsonize.go` を削除
   - `sync.go`, `vars.go` を追加
   - `internal/dotenv/` を追加

2. **Milestones** テーブル
   - M8 として `sync コマンド追加・export/jsonize 廃止` を Done で追加

3. **Ref Syntax** セクション
   - `psa:/path/to/key` の行を削除（v0.6.0 で廃止済み）

---

### Unit 3: docs/ 更新

**ファイル**: `docs/specs/bundr-spec-v1.1.md`, `docs/configuration.md`

**変更内容**:

`docs/specs/bundr-spec-v1.1.md`:
- `psa` バックエンド記載を削除 → `ps: --tier advanced` に統一
- `bundr jsonize` コマンド仕様を削除
- `bundr export` コマンド仕様を削除
- `bundr sync` コマンド仕様を追加（from/to の型とデフォルト動作テーブル）

`docs/configuration.md`:
- `bundr sync` コマンドへの言及を追加（環境変数に影響されるコマンド一覧に追加）

---

### Unit 4: plans/bundr-roadmap.md 更新

**ファイル**: `plans/bundr-roadmap.md`

**変更内容**:
- M5, M6, M7 の Status を Done に更新
- v0.7.0 / M8 として sync コマンド追加・export/jsonize 廃止を記録
- Current Focus 更新

---

## e2e テストレシピ

ドキュメント変更のため AWS は不要。以下で確認する：

```bash
cd /Users/youyo/src/github.com/youyo/bundr

# ビルド
go build -o /tmp/bundr .

# コマンド一覧（export/jsonize が消えていること）
/tmp/bundr --help

# sync のヘルプが正しいこと
/tmp/bundr sync --help

# put のヘルプに --store がないこと
/tmp/bundr put --help

# テスト全通過
go test ./...
```

---

## ワーカー共通インストラクション

各ユニットは独立して実装可能。依存関係なし。

実装後：
1. `Skill` ツールで `skill: "simplify"` を実行
2. `go test ./...` でテスト通過を確認
3. e2e テストレシピを実行
4. `gh pr create` で PR を作成
5. 最後に `PR: <url>` を出力
