# Plan: bundr ls で sm: バックエンドをサポートする

## Context

`bundr ls` コマンドは現在 sm: バックエンドを明示的に拒否している。
ユーザーは `bundr ls sm:partner-ops/` や `bundr ls sm:` で Secrets Manager のシークレット一覧を取得したい。

現在の問題:
- `bundr ls sm:` → `invalid ref: path is empty`（ParseRef がパス空を拒否）
- `bundr ls sm:partner-ops/` → `sm: backend is not supported`（ls.go のガードが拒否）

## Research Summary

| ファイル | 現状 | 変更要否 |
|---------|------|---------|
| `internal/backend/sm.go` | `GetByPrefix` は "not supported" エラーのみ返す実装。`smClient` に `ListSecrets` なし | **要変更** |
| `internal/backend/sm_test.go` | `mockSMClient` に `ListSecrets` なし | **要変更** |
| `cmd/ls.go:32-34` | `BackendTypeSM` を明示的に弾くガード | **要変更** |
| `cmd/ls_test.go:L-04` | sm: がエラーになることを確認するテスト | **要変更** |
| `internal/backend/interface.go` | `GetByPrefix` はインターフェースに含まれている | 変更不要 |
| `internal/backend/mock.go` | sm: ref も ParseRef 経由で処理できる | 変更不要 |

## Work Units

### Unit 1: SMBackend.GetByPrefix 実装（sm.go + sm_test.go）

**ファイル**: `internal/backend/sm.go`, `internal/backend/sm_test.go`

`smClient` インターフェースに `ListSecrets` を追加し、`SMBackend.GetByPrefix` を実装する。

**`sm.go` の変更:**
```go
// smClient インターフェースに追加
ListSecrets(ctx context.Context, input *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
```

```go
// GetByPrefix 実装
func (b *SMBackend) GetByPrefix(ctx context.Context, prefix string, opts GetByPrefixOptions) ([]ParameterEntry, error) {
    var entries []ParameterEntry
    var nextToken *string

    for {
        input := &secretsmanager.ListSecretsInput{}
        if prefix != "" {
            input.Filters = []smtypes.Filter{
                {Key: smtypes.FilterNameStringTypeName, Values: []string{prefix}},
            }
        }
        if nextToken != nil {
            input.NextToken = nextToken
        }

        out, err := b.client.ListSecrets(ctx, input)
        if err != nil {
            return nil, fmt.Errorf("list secrets: %w", err)
        }

        for _, secret := range out.SecretList {
            name := aws.ToString(secret.Name)

            // Recursive=false: / を含む remainder はスキップ
            if !opts.Recursive && prefix != "" {
                remainder := strings.TrimPrefix(name, prefix)
                if strings.Contains(remainder, "/") {
                    continue
                }
            }

            storeMode := getTagValue(secret.Tags, tags.TagStoreMode)
            entries = append(entries, ParameterEntry{
                Path:      name,
                Value:     "",
                StoreMode: storeMode,
            })
        }

        nextToken = out.NextToken
        if nextToken == nil {
            break
        }
    }

    return entries, nil
}
```

**重要点:**
- `ListSecretsOutput.SecretList[].Tags` にタグが含まれるので `DescribeSecret` を個別呼び出し不要
- `strings` パッケージを import 追加
- `FilterNameStringTypeName` = `"name"` フィルタで prefix マッチング
- prefix = "" の場合は Filters なし → 全シークレットを返す

**`sm_test.go` の変更:**
- `mockSMClient` に `ListSecrets` メソッド追加（`secrets` map から prefix フィルタリング）
- テストケース追加:
  - `TestSMBackend_GetByPrefix_Empty` — prefix="" で全シークレット返す
  - `TestSMBackend_GetByPrefix_WithPrefix` — prefix="partner-ops/" でフィルタリング
  - `TestSMBackend_GetByPrefix_Recursive_False` — `--no-recursive` でサブパスをスキップ
  - `TestSMBackend_GetByPrefix_Pagination` — ページネーション（NextToken）

### Unit 2: ls コマンドの sm: サポート

**ファイル**: `cmd/ls.go`, `cmd/ls_test.go`

**`ls.go` の変更:**
1. sm: ガード（行32-34）を削除
2. `sm:` (path 空) のケースを許容する処理を追加

```go
// sm: のみ（全シークレット一覧）を許容する処理
var ref backend.Ref
if c.From == "sm:" {
    // sm: は全シークレット一覧（path = ""）として扱う
    ref = backend.Ref{Type: backend.BackendTypeSM, Path: ""}
} else {
    var parseErr error
    ref, parseErr = backend.ParseRef(c.From)
    if parseErr != nil {
        return fmt.Errorf("ls command failed: invalid ref: %w", parseErr)
    }
}
```

3. 出力フォーマット: sm: の場合は `sm:` + entry.Path（PSと同じパターン）

**`ls_test.go` の変更:**
- L-04 を sm: が動作することを確認するテストに変更
  - `sm:` で全シークレット一覧が取得できる
  - `sm:partner-ops/` で prefix フィルタリングが動作する
  - `--no-recursive` が動作する
- `newLsTestContext` を MockBackend が sm: ref も扱えるように確認（mock は ref type に関係なく動作するため変更不要の可能性が高い）

### Unit 3: cache refresh の sm: サポート

**ファイル**: `cmd/cache.go`, `cmd/cache_test.go`

**`cache.go` の変更:**
- sm: ガード（行32-35）を削除
- sm: を `"sm"` として `CacheStore.Write` できるようにする
- cache.go の help テキストも sm: に対応した記述に更新

**`cache_test.go` の変更:**
- sm: prefix での cache refresh テスト追加
- `sm:` (全シークレット) での cache refresh テスト追加

### Unit 4: predictor の sm: サポート

**ファイル**: `cmd/predictor.go`, `cmd/pred_test.go`

**`predictor.go` の変更:**
- `newRefPredictor` の sm: 空リスト返すガード（行43-46）を削除
  → sm: の場合も通常の ref 解析 → キャッシュ読み取りに続く
- `newPrefixPredictor` の sm: ガード（行116-118）を削除
  → prefix="" の場合は ps/psa/sm 全部のキャッシュを統合して返す

**補完の動作:**
```
sm: → sm: バックエンドのキャッシュ全エントリを補完候補として返す
sm:part → "part" で始まるエントリをフィルタリング
```

**`pred_test.go` の変更:**
- sm: ref/prefix での predictor 動作テスト追加

## e2e テストレシピ

AWS モックを使った TDD アプローチのため、実AWS環境不要。

```bash
# ユニットテスト実行
go test ./...

# ビルド確認
go build -o bundr ./...

# 手動確認（AWS 環境がある場合）
./bundr ls sm:
./bundr ls sm:partner-ops/
./bundr ls --no-recursive sm:partner-ops/
```

ユニットテストの PASS がバリデーション基準。

## 実装順序

Unit 1〜4 は以下の依存関係がある:
- Unit 2, 3, 4 は Unit 1 の `SMBackend.GetByPrefix` 実装に依存
- Unit 1 が完了すれば、Unit 2/3/4 は独立して並列実装可能

ただし、worktree を使った並列実装では全 Unit を同時に走らせて問題ない（Unit 1 の変更がなくても、Unit 2/3/4 は sm: ガードの削除とテスト変更から始められる）。

最終マージ時に Unit 1 が先にマージされていれば OK。

## 変更ファイルまとめ

| ファイル | 変更内容 |
|---------|---------|
| `internal/backend/sm.go` | `smClient.ListSecrets` 追加、`GetByPrefix` 実装、`strings` import 追加 |
| `internal/backend/sm_test.go` | `mockSMClient.ListSecrets` 追加、GetByPrefix テスト4件追加 |
| `cmd/ls.go` | sm: ガード削除、`sm:` 空 path 特別処理追加 |
| `cmd/ls_test.go` | L-04 更新、sm: テストケース追加 |
| `cmd/cache.go` | sm: ガード削除 |
| `cmd/cache_test.go` | sm: cache refresh テスト追加 |
| `cmd/predictor.go` | sm: 空リストガード削除、sm: キャッシュ読み取り対応 |
| `cmd/pred_test.go` | sm: predictor テスト追加 |
