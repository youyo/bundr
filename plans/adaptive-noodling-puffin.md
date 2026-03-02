# Plan: `sm:` Tab補完 修正（空パス問題）

## Context

### 問題

- `bundr get sm:` + Tab → 補完候補が出ない（全シークレット一覧が表示されるべき）
- `bundr get sm:s` + Tab → 補完される（"s" で始まるシークレットが出る）
- 同様に `ps:` や `psa:` でも（パスなしで） Tab を押した場合も同問題が潜在する

### 根本原因

`backend.ParseRef("sm:")` が `"invalid ref "sm:": path is empty"` エラーを返す。

`newRefPredictor` / `newPrefixPredictor` は ParseRef エラー時に即 `[]string{}` を返す設計のため、`sm:` 入力時は一切の補完処理が走らない。

`sm:s` が動く理由：`ParseRef("sm:s")` → `Ref{Type:"sm", Path:"s"}` として成功する。

| 入力 | ParseRef 結果 | 補完 |
|---|---|---|
| `sm:` | エラー（path空） | ✗ 出ない ← バグ |
| `sm:s` | OK (Path="s") | ✓ 出る |
| `ps:/` | OK (Path="/") | ✓ 出る |
| `ps:` | エラー（path空） | ✗ 出ない（同問題） |

`hierarchicalFilter("", entries, "sm")` 自体は正しく動く（TestHierarchicalFilter_SMBackend で確認済み）。問題は ParseRef のゲートで弾かれること。

---

## 変更内容

### 変更ファイル

- `cmd/predictor.go`（修正）
- `cmd/pred_test.go`（テスト追加）

### 変更詳細

#### 1. `newRefPredictor` の ParseRef エラーハンドリング拡張

`ref, err := backend.ParseRef(prefix)` の直後、`if err != nil` ブロックを拡張：

```go
if err != nil {
    // "sm:", "ps:", "psa:" — パスなしのバックエンドプレフィックス
    // ParseRef は空パスを拒否するため、補完時は特別扱いする
    if prefix == "sm:" || prefix == "ps:" || prefix == "psa:" {
        btStr := strings.TrimSuffix(prefix, ":")
        bgArg := makeBGArg(btStr)
        entries, readErr := cacheStore.Read(btStr)
        if readErr == cache.ErrCacheNotFound {
            _ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
            var livePath string
            if btStr != "sm" {
                livePath = "/"
            }
            liveEntries := fetchLive(factory, btStr, livePath)
            return hierarchicalFilter("", liveEntries, btStr)
        } else if readErr == nil {
            candidates := hierarchicalFilter("", entries, btStr)
            lastRefresh := cacheStore.LastRefreshedAt(btStr)
            if time.Since(lastRefresh) > 10*time.Second {
                _ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
            }
            return candidates
        }
    }
    return []string{}
}
```

#### 2. `newPrefixPredictor` の同様修正

`prefix != ""` のパスで同じ修正を適用（`bundr exec --from sm:` のケース対応）。

#### 3. テスト追加（`cmd/pred_test.go`）

- `TestNewRefPredictor_SMColon_CacheHit`：`sm:` + キャッシュあり → シークレット一覧返す
- `TestNewRefPredictor_SMColon_CacheMiss_NilFactory`：`sm:` + キャッシュなし + factory=nil → 空リスト
- `TestNewRefPredictor_SMColon_CacheMiss_WithFactory`：`sm:` + キャッシュなし + factory → 全シークレット返す
- `TestNewRefPredictor_PSColon_CacheHit`：`ps:` + キャッシュあり → `ps:/` 始まる候補
- `TestNewPrefixPredictor_SMColon_CacheHit`：PrefixPredictor 版

---

## 変更規模

- `cmd/predictor.go`: +20行（`newRefPredictor` と `newPrefixPredictor` 各+10行）
- `cmd/pred_test.go`: +80〜100行（テスト5件）

---

## 検証方法

### ユニットテスト

```bash
go test ./cmd/... -v -run "TestNewRefPredictor_SM|TestNewRefPredictor_PS|TestNewPrefixPredictor_SM"
go test ./...
```

### E2E 確認

```bash
go build -o bundr .

# sm: でのタブ補完エミュレーション（キャッシュあり環境）
COMP_LINE="bundr get sm:" COMP_POINT=13 bundr get sm:
# 期待: sm:secret-name-1 等が出力される

# sm:s でも引き続き動作
COMP_LINE="bundr get sm:s" COMP_POINT=14 bundr get sm:s
# 期待: sm:s で始まるシークレット名が出力される
```

注意: E2E テストは AWS 認証が必要。ユニットテストで MockBackend を使って同等の検証が可能。
