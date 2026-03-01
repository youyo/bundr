# バグ修正: SM バックエンドの `sm:` プレフィックスが AWS API に渡る

## コンテキスト

`bundr get sm:partner-ops/preview/oauth-secrets` が以下のエラーで失敗する:

```
ValidationException: Invalid name. Must be a valid name containing alphanumeric characters,
or any of the following: -/_+=.@!
```

AWS Secrets Manager API が `sm:` を含む名前を拒否している。

## 根本原因

`internal/backend/sm.go` の `Get()` と `Put()` は、受け取った `ref` 文字列（例: `"sm:partner-ops/..."` ）を **そのまま** AWS API に渡している。

比較:
- **PS バックエンド** (`ps.go:109`, `ps.go:35`): `ParseRef(ref)` を内部で呼んで `parsed.Path` のみ AWS に渡す → 正常動作
- **SM バックエンド** (`sm.go:90-91`, `sm.go:53-54`): `ParseRef(ref)` を呼ばず `ref` をそのまま渡す → `sm:` プレフィックスが混入してエラー

既存テスト (`sm_test.go`) が全て通っているのは、テストコードが `"my-secret"` のようにプレフィックスなしの値を直接渡しているため。cmd 層からは `"sm:my-secret"` として渡されるが、このパスはテストでカバーされていなかった。

## 修正方針

`sm.go` の `Get()` と `Put()` に `ParseRef(ref)` 呼び出しを追加し、`parsed.Path` のみ AWS API に渡す。これは `ps.go` が既に実装しているパターンと同一。

## 変更ファイル

### 1. `internal/backend/sm_test.go`（TDD: Red）

テスト追加:
- `TestSMBackend_PutWithSMPrefix` — `Put("sm:my-secret", ...)` が正常動作し `"my-secret"` キーで格納されること
- `TestSMBackend_GetWithSMPrefix` — `Get("sm:my-secret", ...)` が `"sm:my-secret"` キーではなく `"my-secret"` キーから取得できること

### 2. `internal/backend/sm.go`（TDD: Green）

**`Put()` (line 35-85)**:
```go
func (b *SMBackend) Put(ctx context.Context, ref string, opts PutOptions) error {
    parsed, err := ParseRef(ref)        // ← 追加
    if err != nil {
        return err
    }
    secretName := parsed.Path           // ← 追加
    // 以降の `ref` を全て `secretName` に置き換え
    ...
}
```

**`Get()` (line 88-120)**:
```go
func (b *SMBackend) Get(ctx context.Context, ref string, opts GetOptions) (string, error) {
    parsed, err := ParseRef(ref)        // ← 追加
    if err != nil {
        return "", err
    }
    secretName := parsed.Path           // ← 追加
    // 以降の `ref` を全て `secretName` に置き換え
    ...
}
```

## 変更箇所（具体的）

| ファイル | 変更前 | 変更後 |
|---------|--------|--------|
| `sm.go:53` | `Name: aws.String(ref)` | `Name: aws.String(secretName)` |
| `sm.go:67` | `SecretId: aws.String(ref)` | `SecretId: aws.String(secretName)` |
| `sm.go:76` | `SecretId: aws.String(ref)` | `SecretId: aws.String(secretName)` |
| `sm.go:91` | `SecretId: aws.String(ref)` | `SecretId: aws.String(secretName)` |
| `sm.go:105` | `SecretId: aws.String(ref)` | `SecretId: aws.String(secretName)` |

## 影響範囲

- `sm.go` のみ（2メソッド）
- cmd 層（`get.go`, `put.go`）は変更不要（PS バックエンドも cmd 層から full ref で呼ばれているため）
- `export` / `exec` / `jsonize` も SM は `GetByPrefix` 非対応のため影響なし

## 検証

```bash
# ユニットテスト（全通過確認）
go test ./internal/backend/...

# E2E（実際の AWS 環境で確認）
bundr get sm:partner-ops/preview/oauth-secrets
# → エラーなく値が返ること

bundr put sm:test-fix-$(date +%s) --value "hello" --store raw
# → OK が返ること
```
