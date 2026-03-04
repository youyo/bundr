# Plan: ps:/sm: フルネームエイリアス対応 + README 拡充

## Context

`ps:` / `sm:` という短縮プレフィックスは慣れれば便利だが、初見ユーザーには何のことか分かりづらい。
`parameterstore:` / `secretsmanager:` のようなフルネームも受け入れることで、直感性を向上させる。
README も合わせて拡充し、AWS サービス名とプレフィックスの対応を明示する。

## 調査結果

### ref パースの流れ

1. `internal/backend/ref.go` の `ParseRef()` がコロンで分割しプレフィックスを switch で判定
2. `BackendType` 定数は `"ps"` / `"sm"` — 内部的にはこのまま維持
3. `cmd/sync.go` に `isBackendRef()` 関数があり `ps:` / `sm:` を直接チェック
4. `cmd/ls.go` に `sm:` の特殊ケース処理（全 Secrets 列挙用の空パス対応）

### 変更が必要なファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/backend/ref.go` | `ParseRef()` にエイリアス case 追加 |
| `internal/backend/ref_test.go` | エイリアステストケース追加 |
| `cmd/sync.go` | `isBackendRef()` にエイリアスチェック追加 |
| `cmd/ls.go` | `"sm:"` 特殊ケースに `"secretsmanager:"` も追加 |
| `README.md` | フルネーム構文の説明追加・クイックスタート拡充 |

## 実装方針

### エイリアス設計

- フルネームは `ParseRef` 内部で短縮名に正規化（BackendType は変更しない）
- 許容するエイリアス:
  - `parameterstore:` → `ps:` と同じ
  - `secretsmanager:` → `sm:` と同じ
- 出力は常に短縮形（`ps:`, `sm:`）を維持（後方互換性）

### 変更詳細

#### 1. `internal/backend/ref.go` — ParseRef エイリアス追加

```go
case "ps", "parameterstore":
    if path == "" {
        return Ref{}, fmt.Errorf("invalid ref %q: path is empty", raw)
    }
    return Ref{Type: BackendTypePS, Path: path}, nil
case "sm", "secretsmanager":
    if path == "" {
        return Ref{}, fmt.Errorf("invalid ref %q: path is empty", raw)
    }
    return Ref{Type: BackendTypeSM, Path: path}, nil
```

#### 2. `cmd/sync.go` — isBackendRef 更新

```go
func isBackendRef(s string) bool {
    return strings.HasPrefix(s, "ps:") || strings.HasPrefix(s, "sm:") ||
        strings.HasPrefix(s, "parameterstore:") || strings.HasPrefix(s, "secretsmanager:")
}
```

#### 3. `cmd/ls.go` — sm: 特殊ケース拡張

```go
if c.From == "sm:" || c.From == "secretsmanager:" {
    ref = backend.Ref{Type: backend.BackendTypeSM, Path: ""}
} else {
```

#### 4. `internal/backend/ref_test.go` — テストケース追加

- `parameterstore:/app/key` → Type=BackendTypePS, Path=/app/key
- `secretsmanager:my-secret` → Type=BackendTypeSM, Path=my-secret

#### 5. `README.md` — Ref syntax セクション拡充

- フルネームとショートハンドの対応表を追加
- クイックスタートコメントに補足
- 「初めて使う場合は ... も使える」という案内追加

## ワークユニット分割

### Unit 1: コアロジック変更（ref.go + ref_test.go + sync.go + ls.go）
**ファイル**: `internal/backend/ref.go`, `internal/backend/ref_test.go`, `cmd/sync.go`, `cmd/ls.go`
**内容**: ParseRef にエイリアス追加、isBackendRef/ls 特殊ケース更新、テスト追加

### Unit 2: README 拡充
**ファイル**: `README.md`
**内容**: フルネーム構文の説明、Ref Syntax セクション拡充、ビギナー向け補足

## E2E テストレシピ

```bash
# ビルド
go build -o /tmp/bundr ./...

# ユニットテスト
go test ./...

# エイリアスの動作確認（モックなしでパースのみ確認可能）
# ref_test.go が通れば十分。実 AWS は不要
```

## 注意事項

- `BackendType` 定数（`"ps"` / `"sm"`）は変更しない（内部表現の後方互換を維持）
- 出力（ls コマンド等の `string(ref.Type) + ":"` 形式）も変更しない
