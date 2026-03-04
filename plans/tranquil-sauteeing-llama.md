# Plan: sync コマンドの env キー正規化修正

## Context

`bundr sync --from ps:/slacklens/preview/ -t -` の出力に2つの問題がある：

1. **大文字化の欠如**: JSON mode パラメータ（`cli-store-mode=json`）の値を展開したとき、JSON 内のキーが大文字化されない（例: `apigateway.url` が `APIGATEWAY_URL` にならない）
2. **`.` → `_` 変換の欠如**: シェル変数名として無効な `.` が残ったまま出力される（例: `KNOWLEDGEBASE.ID` → `KNOWLEDGEBASE_ID` にすべき）

### `test` キーについて

`bundr get` の出力で `test` は:
```json
"test": "{\"apigateway.url\":\"...\",\"knowledgebase.arn\":\"...\"}"
```
これは `cli-store-mode=json` で格納されているため、`--raw` なしの sync では JSON 展開される。出力末尾の `apigateway.url=...`、`knowledgebase.arn=...` が `test` の展開結果。スキップされているわけではない。ただしキー正規化がないため小文字+ドットのまま出力されている。

## 問題の根本原因（`cmd/sync.go`）

### PS prefix キー生成（sync.go ~L96-97）
```go
key := strings.ToUpper(relPath)     // ✅ 大文字化あり
key = strings.ReplaceAll(key, "/", "_")  // ✅ / → _ あり
// ❌ . → _ 変換なし！
```
→ `agentCoreRuntime.arn` → `AGENTCORERUNTIME.ARN`（ドットが残る）

### JSON 展開（sync.go ~L160-173）
```go
for k, v := range obj {
    expanded = append(expanded, dotenv.Entry{Key: k, Value: ...})
    // ❌ k はそのまま。大文字化も . → _ 変換もなし！
}
```

### dotenv ファイル読み込み時（sync.go ~L120-121、単一ref JSON 解析）
```go
for k, v := range obj {
    entries = append(entries, dotenv.Entry{Key: k, Value: ...})
    // ❌ 同様に正規化なし
}
```

## 修正方針

**対象ファイル**: `cmd/sync.go` のみ

### 正規化ルール（dotenv/export 出力時）
```
1. strings.ToUpper(key)
2. strings.ReplaceAll(key, ".", "_")
3. strings.ReplaceAll(key, "/", "_")  ← 既存
```

この順番で適用する。

### 具体的な変更箇所

**変更 1: PS prefix キー生成（L96-97 付近）**
```go
// Before
key := strings.ToUpper(relPath)
key = strings.ReplaceAll(key, "/", "_")

// After
key := strings.ToUpper(relPath)
key = strings.ReplaceAll(key, ".", "_")  // 追加
key = strings.ReplaceAll(key, "/", "_")
```

**変更 2: JSON 展開ループ（L160-173 付近）**
```go
// Before
for k, v := range obj {
    expanded = append(expanded, dotenv.Entry{Key: k, Value: fmt.Sprintf("%v", v)})
}

// After
for k, v := range obj {
    normKey := strings.ToUpper(k)
    normKey = strings.ReplaceAll(normKey, ".", "_")
    normKey = strings.ReplaceAll(normKey, "/", "_")
    expanded = append(expanded, dotenv.Entry{Key: normKey, Value: fmt.Sprintf("%v", v)})
}
```

**変更 3: 単一 ref JSON 解析（L120-121 付近）**
- ここは dotenv ファイル → PS/SM への書き込み時のパースであり、出力側ではない
- dotenv ファイルのキーをそのまま使うので変更不要（変更しない）

## 期待される出力

```
# 修正後
AGENTCORERUNTIME_ARN=...       ← . が _ に
AGENTCORERUNTIME_ENDPOINTARN=...
...
APIGATEWAY_URL=...             ← . が _ に
...
KNOWLEDGEBASE_ID=...           ← . が _ に
...
APIGATEWAY_URL=...             ← test JSON 展開も大文字 + _ に
KNOWLEDGEBASE_ARN=...
```

## テスト修正

`cmd/sync_test.go` の既存テストでキー期待値を更新する（`.` → `_` 変換を含む）。

## 検証方法

1. `go test ./cmd/...` でテスト通過確認
2. `go build -o bundr ./... && ./bundr sync --from ps:/slacklens/preview/ -t -` で実出力確認
3. `golangci-lint run` で lint 通過確認

## 対象ファイル

- `cmd/sync.go` — 主要変更（2箇所）
- `cmd/sync_test.go` — テスト期待値更新

## スコープ外

- `vars.go` の `buildVars()` は既に `.` → `_` 変換あり（`exec` コマンド用）。変更不要
- `--from` が dotenv ファイルの場合のキー（ユーザーが明示的に書いた名前）は変換しない
