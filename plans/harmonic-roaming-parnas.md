# Plan: bundr ls を標準 ls コマンドと同じ階層表示に修正

## Context

`bundr ls ps:/` を実行しても何も返さない。ユーザーは標準の `ls` コマンドのように、
**次の1レベルだけを表示し、段階的に掘り下げる** 挙動を期待している。

```
bundr ls ps:/              → ps:/cdk-bootstrap, ps:/stratalog
bundr ls ps:/stratalog     → ps:/stratalog/preview, ps:/stratalog/prod
bundr ls ps:/stratalog/prod → ps:/stratalog/prod/acm.certificateArn, ...
```

## 根本原因

現在の実装は `GetParametersByPath(Recursive=false)` を使い「直下の1レベルのみ取得」しているが、
AWS SSM パラメータは `/prefix/subpath/name` の2レベル以上が常識的。
**直下1レベルにパラメータが存在しないため、常に空が返る。**

## 解決策

**SSM からは常に再帰的に全パラメータを取得し、表示時に「次の1レベルのみ」を抽出する。**

`--recursive` フラグなし（デフォルト）：次レベルのみのディレクトリ表示（新動作）
`--recursive` フラグあり：全パラメータのフラット一覧（既存動作）

### 次レベル抽出アルゴリズム

```
prefix = strings.TrimRight(ref.Path, "/") + "/"  // "/stratalog/"

各エントリに対して:
  rel = strings.TrimPrefix(entry.Path, prefix)    // "preview/acm.certificateArn"
  if strings.Contains(rel, "/"):
    → "directory" = ref.Type + ":" + prefix + rel[:idx]  // "ps:/stratalog/preview"
  else:
    → leaf param = ref.Type + ":" + entry.Path           // "ps:/stratalog/prod/acm"
```

重複除去 (seen map) してソートして表示。

## 変更ファイル

### `cmd/ls.go`

1. `import "strings"` を追加
2. `GetByPrefix` を常に `Recursive: true` で呼ぶ
3. `c.Recursive` フラグで表示ロジックを分岐:
   - `false`（デフォルト）: 次レベル抽出アルゴリズムを適用
   - `true`: 既存の全表示ロジックをそのまま使用
4. `Recursive` のヘルプ文を更新（`"List all parameters recursively (default: next-level view only)"``）

```go
// 変更後の GetByPrefix 呼び出し
entries, err := b.GetByPrefix(context.Background(), ref.Path, backend.GetByPrefixOptions{
    Recursive:    true,  // 常に全取得（次レベル表示に必要）
    SkipTagFetch: true,
})

var refs []string
if c.Recursive {
    // --recursive: 全パラメータをフラット表示
    refs = make([]string, 0, len(entries))
    for _, entry := range entries {
        refs = append(refs, string(ref.Type)+":"+entry.Path)
    }
} else {
    // デフォルト: 次レベルのみ（ディレクトリ表示）
    normalizedPrefix := strings.TrimRight(ref.Path, "/") + "/"
    seen := make(map[string]bool)
    refs = make([]string, 0)
    for _, entry := range entries {
        rel := strings.TrimPrefix(entry.Path, normalizedPrefix)
        if rel == entry.Path {
            continue
        }
        var key string
        if idx := strings.Index(rel, "/"); idx == -1 {
            key = string(ref.Type) + ":" + entry.Path
        } else {
            key = string(ref.Type) + ":" + normalizedPrefix + rel[:idx]
        }
        if !seen[key] {
            seen[key] = true
            refs = append(refs, key)
        }
    }
}
sort.Strings(refs)
for _, r := range refs {
    fmt.Fprintln(c.out, r)
}
```

### `cmd/ls_test.go`

1. **L-03 の `want` を更新**: `recursive=false` + ネストパラメータがある場合は
   `ps:/app/sub` も出力される（ディレクトリとして）
   ```go
   want: []string{
       "ps:/app/db_host",    // leaf
       "ps:/app/sub",        // virtual directory prefix
   },
   ```

2. **L-06 を追加**: ルート `ps:/` 指定 + ネストパラメータ → トップレベルディレクトリが返ること
   ```
   データ: ps:/app/key, ps:/other/key2
   期待: ps:/app, ps:/other
   ```

## 検証手順

```bash
# ユニットテスト
go test ./cmd/ -v -run TestLsCmd

# ビルド
go build -o /tmp/bundr_test ./...

# 動作確認（AWS環境）
/tmp/bundr_test ls ps:/              # → /cdk-bootstrap, /stratalog など
/tmp/bundr_test ls ps:/stratalog     # → /stratalog/preview, /stratalog/prod
/tmp/bundr_test ls ps:/stratalog/prod # → leaf パラメータ一覧
/tmp/bundr_test ls ps:/ --recursive   # → 全パラメータのフラット一覧

# 全テスト
go test ./...
```

## 影響範囲

- 変更ファイル: `cmd/ls.go`（20行程度変更）、`cmd/ls_test.go`（2ケース変更/追加）
- Breaking change: `--recursive` なしの挙動が変わる（バグ修正として扱う）
- `runDescribe` は既存の `c.Recursive` 動作を維持（影響なし）
