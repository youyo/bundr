# Plan: Hierarchical Tab Completion for `get` Command

## Context

ユーザーが `bundr get` の後で Tab を連打し、階層を 1 段ずつたどれるようにしたい。

**現在の問題**: `ps:/app` を入力して Tab を押すと、`/app` 配下のキー全て
(`ps:/app/prod/DB_HOST`, `ps:/app/prod/DB_PORT`, `ps:/app/stg/DB_HOST` …) が
一度に候補として返されるため、ディレクトリ感がなく選択しづらい。

**目標**:
- `ps:` → Tab → `ps:/app/`, `ps:/config/` (第1階層のみ)
- `ps:/app/` → Tab → `ps:/app/prod/`, `ps:/app/stg/` (第2階層のみ)
- `ps:/app/prod/` → Tab → `ps:/app/prod/DB_HOST`, `ps:/app/prod/DB_PORT` (リーフ)

---

## 調査結果

### 現行コードの問題箇所 (`cmd/predictor.go:56-61`)

```go
// 現在: 全マッチを返す
for _, e := range entries {
    if strings.HasPrefix(e.Path, ref.Path) {
        candidates = append(candidates, string(ref.Type)+":"+e.Path)
    }
}
```

バグも発見: L67 の stale BG refresh で `"--prefix"` フラグを渡しているが
Kong struct にそのフラグは存在しない → BG 起動はされるが引数が無視される。

### キャッシュ構造

```
CacheEntry { Path: "/app/prod/DB_HOST", StoreMode: "raw" }
```

値は保存されない（パス + StoreMode のみ）。

---

## 実装方針

### 核心: `hierarchicalFilter` 関数の追加

現在のフラットフィルタを、「次の1階層のみ返す」ロジックに置き換える。

**アルゴリズム**:
1. `refPath` から `parentPath`（最後の `/` まで）を求める
2. `refPath` にマッチするエントリのみ通す
3. `parentPath` を除いた `relative` パスを求める
4. `relative` に `/` があれば → 中間ノード（末尾 `/` 付きで返す）
5. `relative` に `/` がなければ → リーフノード（フルパスで返す）
6. 重複排除

**ケース別トレース**:

| input (refPath) | parentPath | entry | relative | candidate |
|---|---|---|---|---|
| `""` | `"/"` | `/app/prod/DB_HOST` | `app/prod/DB_HOST` | `ps:/app/` |
| `"/app"` | `"/"` | `/app/prod/DB_HOST` | `app/prod/DB_HOST` | `ps:/app/` |
| `"/app/"` | `"/app/"` | `/app/prod/DB_HOST` | `prod/DB_HOST` | `ps:/app/prod/` |
| `"/app/pr"` | `"/app/"` | `/app/prod/DB_HOST` | `prod/DB_HOST` | `ps:/app/prod/` |
| `"/app/prod/"` | `"/app/prod/"` | `/app/prod/DB_HOST` | `DB_HOST` | `ps:/app/prod/DB_HOST` |
| `""` | `"/"` | SM: `my-secret` | `my-secret` | `sm:my-secret` |

---

## 変更ファイルと詳細

### 1. `cmd/predictor.go`

**追加: `hierarchicalFilter` 関数**

```go
func hierarchicalFilter(refPath string, entries []cache.CacheEntry, refTypeStr string) []string {
    var parentPath string
    if refPath == "" || refPath == "/" {
        parentPath = "/"
    } else if strings.HasSuffix(refPath, "/") {
        parentPath = refPath
    } else {
        if idx := strings.LastIndex(refPath, "/"); idx >= 0 {
            parentPath = refPath[:idx+1]
        } else {
            parentPath = ""  // SM: "/" を含まないシークレット名
        }
    }

    seen := make(map[string]struct{})
    var candidates []string
    for _, e := range entries {
        if refPath != "" && !strings.HasPrefix(e.Path, refPath) {
            continue
        }
        relative := strings.TrimPrefix(e.Path, parentPath)
        slashIdx := strings.Index(relative, "/")
        var candidate string
        if slashIdx < 0 {
            candidate = refTypeStr + ":" + e.Path
        } else {
            candidate = refTypeStr + ":" + parentPath + relative[:slashIdx+1]
        }
        if _, ok := seen[candidate]; !ok {
            seen[candidate] = struct{}{}
            candidates = append(candidates, candidate)
        }
    }
    return candidates
}
```

**`newRefPredictor` の候補生成部分を置き換え**:

```go
// Before:
var candidates []string
for _, e := range entries {
    if strings.HasPrefix(e.Path, ref.Path) {
        candidates = append(candidates, string(ref.Type)+":"+e.Path)
    }
}

// After:
candidates := hierarchicalFilter(ref.Path, entries, string(ref.Type))
```

**BG refresh 引数のバグ修正** (L67 の `"--prefix"` は Kong に存在しない):

```go
// Before (bug):
_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", "--prefix", prefix)

// After:
bgArg := string(ref.Type) + ":/"
if string(ref.Type) == "sm" { bgArg = "sm:" }
_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
```

キャッシュミス時の BG 引数も同様にバックエンドルートへ変更:
```go
// Before (L47):
_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", prefix)
// After:
bgArg := string(ref.Type) + ":/"
if string(ref.Type) == "sm" { bgArg = "sm:" }
_ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
```

**`newPrefixPredictor` も同様に変更**:
- 空文字ループ内: `backendType+":"+e.Path` の列挙 → `hierarchicalFilter("", entries, backendType)` の結果を append
- 通常 prefix 指定時: `hierarchicalFilter(ref.Path, entries, string(ref.Type))` に置き換え
- stale BG refresh の `"--prefix"` バグも同様に修正

### 2. `cmd/pred_test.go`

**既存テストの修正**:

`TestNewRefPredictor_CacheHitBGRefresh` (pred-cmd-001):
- `len(candidates) != 2` → `len(candidates) != 1`
- 候補が `"ps:/app/prod/"` であることを検証（`/app/prod/DB_HOST` と `/app/prod/DB_PORT` は `ps:/app/prod/` に統合される）

**新規テスト追加**:

```go
// TestHierarchicalFilter_IntermediateNode: "/app/pr" → "ps:/app/prod/" (重複排除)
// TestHierarchicalFilter_LeafNode: "/app/prod/" → "ps:/app/prod/DB_HOST", "ps:/app/prod/DB_PORT"
// TestHierarchicalFilter_EmptyRefPath: "" → "ps:/app/", "ps:/config/"
// TestHierarchicalFilter_SMBackend: SM "my-secret" はリーフとして返る
// TestHierarchicalFilter_Deduplication: 重複エントリが重複なしで返る
// TestNewRefPredictor_HierarchicalLeaf: refPredictor が階層リーフを返す
// TestNewRefPredictor_HierarchicalDirectory: refPredictor が階層ディレクトリを返す
```

---

## 影響を受ける既存テスト（変更不要）

以下のテストは assertions が十分柔軟なので変更不要:
- `pred-cmd-011`: `strings.HasPrefix(c, "ps:/app")` → `"ps:/app/prod/"` もこれを満たす ✓
- `pred-cmd-007`: `strings.HasPrefix(c, "ps:/")` → `"ps:/app/"` もこれを満たす ✓
- `pred-cmd-009`: `strings.HasPrefix(c, "ps:/")` → 同上 ✓

---

## E2E 検証レシピ

### 自動検証 (ユニットテスト)
```bash
go test ./cmd/... -v -run "TestHierarchical|TestNewRef|TestNewPrefix"
go test ./...
```

### ビルド確認
```bash
go build -o bundr ./...
```

### 手動検証 (bash completion 動作確認)

```bash
# 1. キャッシュを準備 (実 AWS または手動作成)
mkdir -p ~/.cache/bundr
cat > ~/.cache/bundr/ps.json << 'EOF'
{"schema_version":"v1","backend_type":"ps","updated_at":"2026-03-02T00:00:00Z","last_refreshed_at":"2026-03-02T00:00:00Z","entries":[
  {"path":"/app/prod/DB_HOST","store_mode":"raw"},
  {"path":"/app/prod/DB_PORT","store_mode":"raw"},
  {"path":"/app/stg/DB_HOST","store_mode":"raw"},
  {"path":"/config/KEY","store_mode":"raw"}
]}
EOF

# 2. bash 補完の動作確認
COMP_LINE="bundr get ps:/app/" COMP_POINT=18 ./bundr
# 期待出力: ps:/app/prod/ と ps:/app/stg/ のみ（/app/prod/DB_HOST などではない）

COMP_LINE="bundr get ps:/app/prod/" COMP_POINT=24 ./bundr
# 期待出力: ps:/app/prod/DB_HOST と ps:/app/prod/DB_PORT
```

---

## 作業単位

この変更は 2 ファイル (`cmd/predictor.go` + `cmd/pred_test.go`) のみで完結し、
他コンポーネントへの影響はない。単一の worktree で実装する。
