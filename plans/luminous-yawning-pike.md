---
title: psa: 完全削除 — ps: を唯一の SSM プレフィックスにする
project: bundr
author: claude
created: 2026-03-04
status: Done
---

# psa: 完全削除計画

## Context（背景・動機）

前フェーズ（ps:/psa: 統一）で `psa:` を `BackendTypePS + AdvancedTier: true` に正規化して後方互換を維持した。
しかし `psa:` を残すことで次のコストが発生している:

- `Ref.AdvancedTier` が `psa:` 専用の概念としてコアな構造体に残る
- `resolveTier` に `ref.AdvancedTier || opts.AdvancedTier` の 2 経路が存在する
- `cmd/put.go` に `ref.AdvancedTier → opts.AdvancedTier` の伝播ブロックが必要
- `predictor.go` / エラーメッセージ / テストデータに `psa:` が散在する（10+ ファイル）

`psa:` を完全に削除することでこれらを一掃できる。
代替手段は `bundr put ps:/path --tier advanced` で完全に対応可能。

---

## 設計方針

### 変更内容

| 項目 | 変更前 | 変更後 |
|------|--------|--------|
| `ParseRef("psa:...")` | `Ref{Type: BackendTypePS, AdvancedTier: true}` | エラー返却 |
| `Ref.AdvancedTier` フィールド | あり | **削除** |
| `resolveTier` の条件 | `opts.AdvancedTier \|\| ref.AdvancedTier` | `opts.AdvancedTier` のみ |
| `put.go` の伝播ブロック | あり | **削除** |
| バックエンド補完候補 | `ps:`, `psa:`, `sm:` | `ps:`, `sm:` |

### スコープ外（削除しない）

- `PutOptions.AdvancedTier` / `TierExplicit` — `--tier advanced` フラグとして存続
- `PSBackend.resolveTier()` 自体 — `--tier advanced` の処理に引き続き必要
- `SSMClient.DescribeParameters` — auto-detect に引き続き必要
- `sm:` バックエンド（無関係）

---

## 変更ファイル一覧

| ファイル | 変更内容 |
|----------|----------|
| `internal/backend/ref.go` | `psa:` ケースをエラーに変更、`AdvancedTier` フィールド削除、エラーメッセージ更新 |
| `internal/backend/ps.go` | `resolveTier` から `ref.AdvancedTier` 参照を削除、コメント更新 |
| `cmd/put.go` | `ref.AdvancedTier` → `opts.AdvancedTier` 伝播ブロックを削除 |
| `cmd/predictor.go` | `psa:` 候補を削除（`"ps:", "psa:", "sm:"` → `"ps:", "sm:"`、3 箇所） |
| `cmd/export.go` | エラーメッセージ更新（`use ps: or psa:` → `use ps:`） |
| `cmd/jsonize.go` | 同上 |
| `internal/backend/ref_test.go` | `psa:` テストを「エラー期待」に更新、`wantAdvancedTier` フィールド削除 |
| `internal/backend/ps_test.go` | `TestPSBackend_PutAdvancedTier_PSAPrefix` 削除 |
| `cmd/put_test.go` | `TestPutCmd_RunPSA` 削除 |
| `cmd/pred_test.go` | `psa:` 期待値を削除（pred-005/007/008 関連） |
| `cmd/export_test.go` | `psa:` テストデータを `ps:` に置換 |
| `cmd/exec_test.go` | `psa:` テストデータを `ps:` に置換 |
| `cmd/jsonize_test.go` | `psa:` テストデータを `ps:` に置換 |
| `main_test.go` | `psa:` 候補期待を削除 |

---

## 実装手順

### Step 1: `internal/backend/ref.go`

```go
// Ref.AdvancedTier フィールドを削除
type Ref struct {
    Type BackendType
    Path string
}

// psa: をエラーに変更
case "psa":
    return Ref{}, fmt.Errorf("psa: prefix is no longer supported; use ps: with --tier advanced instead")

// エラーメッセージ更新
return Ref{}, fmt.Errorf("invalid ref %q: missing prefix (expected ps: or sm:)", raw)
```

### Step 2: `internal/backend/ps.go`

```go
// resolveTier の条件を単純化
func (b *PSBackend) resolveTier(ctx context.Context, ref Ref, opts PutOptions) ssmtypes.ParameterTier {
    if opts.AdvancedTier {  // ref.AdvancedTier を削除
        return ssmtypes.ParameterTierAdvanced
    }
    // ... 残りは同じ
}
```

### Step 3: `cmd/put.go`

```go
// 以下の伝播ブロックを削除:
// if ref.AdvancedTier {
//     opts.AdvancedTier = true
// }
```

### Step 4: `cmd/predictor.go` — 3 箇所を更新

```go
// Before:
if prefix == "sm:" || prefix == "ps:" || prefix == "psa:" {

// After:
if prefix == "sm:" || prefix == "ps:" {
```

空文字の全バックエンド列挙箇所:
```go
// Before: ps:, psa:, sm: の 3 バックエンドで BG 起動
// After:  ps:, sm: の 2 バックエンドで BG 起動
```

### Step 5: テストファイルの `psa:` 参照更新

- `ref_test.go`: `psa:` を `wantErr: true` に変更、`wantAdvancedTier` 削除
- `ps_test.go`: `TestPSBackend_PutAdvancedTier_PSAPrefix` を削除
- `put_test.go`: `TestPutCmd_RunPSA` を削除
- `pred_test.go`: `psa:` 候補期待を削除、BG 起動回数を 3→2 に変更
- `export_test.go` / `exec_test.go` / `jsonize_test.go`: `psa:/` → `ps:/` に置換
- `main_test.go`: `psa:` 候補チェックを削除

---

## ユーザー影響

- **破壊的変更**: `psa:` を使った既存スクリプトは `ps: --tier advanced` への変更が必要
- **バージョン**: v0.6.0 の変更として扱う（CHANGELOG に記載）
- **移行手順**: `bundr put psa:/path --value x` → `bundr put ps:/path --tier advanced --value x`

---

## 検証方法

```bash
go test -race ./...
```

e2e は不要（AWS 接続なし、unit tests で完結）。
