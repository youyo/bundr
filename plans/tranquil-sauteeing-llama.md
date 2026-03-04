# Plan: sync コマンドの JSON 展開バグ修正（test キー消失・重複キー問題）

## Context

`bundr sync --from ps:/slacklens/preview/ -t -` で2つの問題が発生している（v0.7.3 現在）：

1. **`test` キーが消える**: PS パラメータ `test` の値がたまたま JSON 文字列のため、`writeEntries` の JSON 展開ループが `test` のキーを飲み込む
2. **重複キーが発生**: `apiGateway.url` パラメータ（直接）と `test` の JSON 展開（間接）が同じキー `APIGATEWAY_URL` を2重に生成

## 根本原因の分析

`writeEntries` に「全エントリの値を JSON としてパースして展開する」ロジックがある（L164-180）。これが問題の原因：
- `cli-store-mode` タグを**無視**して任意の JSON 値を展開してしまう
- PS prefix 読み込みでは既にリーフ値が返されているのに、さらに展開を試みる
- **単一 ref 読み込み（`readEntries` L117-129）では既に JSON 展開済み** → `writeEntries` の展開は二重で無意味かつ有害

## 修正方針（シンプル版）

**対象ファイル**: `cmd/sync.go` + `cmd/sync_test.go`

`StoreModeJSON` の prefix パラメータ展開は稀なユースケースのため対応しない。

### 変更: `writeEntries` から JSON 展開ループを削除するだけ

PS prefix の各エントリはリーフ値なので、`writeEntries` で展開する必要はない。
単一 ref の JSON 展開は `readEntries`（L117-129）で既に処理済み。

```go
// After: 展開ループを削除、エントリをそのまま書き出す
if !isBackendRef(c.To) {
    ...
    writeFn := dotenv.Write
    if c.Format == "export" {
        writeFn = dotenv.WriteExport
    }
    sortEntries(entries)
    return writeFn(w, entries)
}
```

`readEntries` prefix case は変更しない。

## 期待される動作の変化

| ケース | 修正前 | 修正後 |
|--------|--------|--------|
| PS prefix + JSON 文字列値（`test` ケース） | 展開される・元キー消失 ❌ | **展開されない・元キー保持** ✅ |
| PS prefix + 通常スカラー値 | 展開されない（そのまま） | 変化なし |
| PS 単一 ref + JSON 値 | 展開される（readEntries で） | 変化なし |
| file/stdin + JSON 文字列 | 展開される（意図しない動作）❌ | **展開されない** ✅ |

## テスト追加

新テスト:
- `TestSyncCmd_PS_Prefix_JSON_Value_NotExpanded`: `StoreModeRaw` + JSON 値のパラメータが展開されず元キーが保持されることを確認（`test` ケースの再現）

既存テストへの影響：全て変化なし（単一 ref は readEntries で展開済みのため）

## 検証方法

```bash
go test ./cmd/...
go build ./...
```

## 対象ファイル

- `cmd/sync.go` — 変更 1（readEntries prefix に StoreMode 展開追加）、変更 2（writeEntries から展開削除）
- `cmd/sync_test.go` — 新テスト2件追加

## スコープ外

- `vars.go` / `internal/dotenv/` — 変更不要
- `--raw` の既存動作（単一 ref）— 変更なし
