# Plan: SM Tab補完で先頭スラッシュが付くバグ修正

## Context

### 問題
`bundr get sm:` + Tab → `sm:/stratalog/`（**先頭スラッシュ付き**）が出る（v0.4.10 時点）。
実際のシークレット: `sm:stratalog/preview/slack-app`（先頭スラッシュなし）。

### 根本原因
`cmd/predictor.go` の `hierarchicalFilter` 関数（68-78行目）で `refPath == ""` のとき
`parentPath = "/"` と固定している。

**PS/PSA（正しい）:**
- エントリ: `/app/prod/KEY`（先頭 "/" あり）
- `TrimPrefix("/app/prod/KEY", "/")` → `"app/prod/KEY"`
- candidate = `"ps:" + "/" + "app/"` = `"ps:/app/"` ✓

**SM（バグ）:**
- エントリ: `stratalog/preview/slack-app`（先頭 "/" なし）
- `TrimPrefix("stratalog/preview/slack-app", "/")` → 変わらない
- candidate = `"sm:" + "/" + "stratalog/"` = `"sm:/stratalog/"` ✗ ← バグ

### テストギャップ
`TestHierarchicalFilter_SMBackend` は `{Path: "my-secret"}` のような "/" なしパスのみで、
`stratalog/preview/slack-app` のような "/" 含む SM パスがテストされていなかった。

---

## 変更内容（作業単位: 1件）

### 変更ファイル
- `cmd/predictor.go`（修正：条件分岐1つ）
- `cmd/pred_test.go`（テスト追加：2件）

### 変更詳細

#### 1. `hierarchicalFilter` の parentPath 計算修正

**Before（68-71行目）:**
```go
var parentPath string
if refPath == "" || refPath == "/" {
    parentPath = "/"
}
```

**After:**
```go
var parentPath string
if refPath == "/" {
    parentPath = "/"
} else if refPath == "" {
    // SM パスは先頭スラッシュなし（"stratalog/key"）→ parentPath = ""
    // PS/PSA パスは先頭スラッシュあり（"/app/key"）→ parentPath = "/"
    if refTypeStr != "sm" {
        parentPath = "/"
    }
    // sm の場合は "" のまま（Go zero value）
}
```

修正後の SM 計算:
- `parentPath = ""`
- `relative = "stratalog/preview/slack-app"`（変化なし）
- candidate = `"sm:" + "" + "stratalog/"` = `"sm:stratalog/"` ✓

#### 2. テスト追加

**`TestHierarchicalFilter_SMHierarchical`**: SM の "/" 含むシークレット名でのフィルタリング
```go
entries := []cache.CacheEntry{
    {Path: "stratalog/preview/slack-app"},
    {Path: "stratalog/workspace/T02DD82CE"},
}
candidates := hierarchicalFilter("", entries, "sm")
// expected: ["sm:stratalog/"] （重複排除）
```

**`TestHierarchicalFilter_SMHierarchical_DeepPath`**: SM の深いパスでのフィルタリング
```go
candidates := hierarchicalFilter("stratalog/", entries, "sm")
// expected: ["sm:stratalog/preview/", "sm:stratalog/workspace/"]
```

---

## 変更規模
- `cmd/predictor.go`: +4行（条件分岐の分割）
- `cmd/pred_test.go`: +45行（テスト2件）

---

## 検証方法

```bash
# ユニットテスト
go test ./cmd/... -v -run "TestHierarchicalFilter_SM"
go test ./...
```

E2E（AWS 認証必要）:
```bash
go build -o bundr .
bundr cache refresh sm:
COMP_LINE="bundr get sm:" COMP_POINT=13 ./bundr get sm:
# 期待: sm:stratalog/ 等（先頭スラッシュなし）
```
