# Tab補完 first response 改善

## Context

`bundr get ps:/` でTabを押しても、キャッシュが存在しない場合（初回・`bundr cache clear`後）は
候補が出ない。BG起動で数秒後にキャッシュができてから2回目以降に出る設計だが、
first responseの悪さが致命的で「使えない状態」になっている。

### 根本原因

現在の `predictor.go:94-97` の処理:
```go
if err == cache.ErrCacheNotFound {
    _ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
    return []string{}  // ← 初回は空を返すだけ
}
```

キャッシュなしのとき空リストを返して終わり。BG起動でキャッシュが作られるまで
何度Tabを押しても候補が出ない。

### 追加問題: N+1 API 呼び出し

`ps.GetByPrefix` は各パラメータに `ListTagsForResource` を呼ぶ（N+1問題）。
40パラメータなら41回API呼び出し → BG起動のキャッシュ作成が遅い。
リアルタイム呼び出しでもこの問題があるため、タグ取得スキップオプションが必要。

## 修正方針

**キャッシュなし時のフォールバック: リアルタイムAPI呼び出し**

- キャッシュあり → 既存通り（即座にキャッシュから返す）
- キャッシュなし → **リアルタイムAPIを直接呼んで候補を返す** + BG起動でキャッシュ作成

これにより：
- 初回Tab: 数百ms でAPI呼び出し → 候補が出る
- 2回目以降: キャッシュから即座に返す（高速）

## 変更内容

### ファイル一覧（6ファイル）

1. `internal/backend/interface.go` (+2行)
2. `internal/backend/ps.go` (+5行変更)
3. `cmd/predictor.go` (+15行変更)
4. `cmd/root.go` (+3行変更) ← BackendFactory を predictor に追加
5. `main.go` (+5行変更) ← BackendFactory を補完前に作成
6. テスト: `cmd/pred_test.go` / `internal/backend/ps_test.go`

---

### 1. `internal/backend/interface.go`

```go
type GetByPrefixOptions struct {
    Recursive    bool
    SkipTagFetch bool // タグ取得スキップ（補完・cache refresh 専用）。StoreMode = ""
}
```

### 2. `internal/backend/ps.go` (ps.go:183-191)

```go
// 変更前
storeMode, err := b.getStoreMode(ctx, path)
if err != nil { ... }

// 変更後
var storeMode string
if !opts.SkipTagFetch {
    storeMode, err = b.getStoreMode(ctx, path)
    if err != nil { ... }
}
```

### 3. `cmd/predictor.go` — `newRefPredictor` の変更

BackendFactory を追加し、ErrCacheNotFound 時にリアルタイムAPIを呼ぶ:

```go
func newRefPredictor(
    cacheStore cache.Store,
    bgLauncher BGLauncher,
    factory BackendFactory,  // 追加
) func(string) []string {
    return func(prefix string) []string {
        ref, err := backend.ParseRef(prefix)
        if err != nil { return []string{} }

        backendType := string(ref.Type)
        bgArg := makeBGArg(backendType)

        entries, err := cacheStore.Read(backendType)
        if err == cache.ErrCacheNotFound {
            // キャッシュなし → リアルタイムAPIで取得してキャッシュにも書き込む
            _ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
            entries = fetchLive(factory, backendType, ref.Path)
            return hierarchicalFilter(ref.Path, entries, string(ref.Type))
        } else if err != nil {
            return []string{}
        }

        candidates := hierarchicalFilter(ref.Path, entries, string(ref.Type))
        if time.Since(cacheStore.LastRefreshedAt(backendType)) > 10*time.Second {
            _ = bgLauncher.Launch(os.Args[0], "cache", "refresh", bgArg)
        }
        return candidates
    }
}

// fetchLive は AWS API を直接呼んでパス一覧を取得する（タグ取得なし）。
// エラー時は空スライスを返す。
func fetchLive(factory BackendFactory, backendType, prefix string) []cache.CacheEntry {
    if factory == nil { return nil }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    b, err := factory(backend.BackendType(backendType))
    if err != nil { return nil }

    entries, err := b.GetByPrefix(ctx, prefix, backend.GetByPrefixOptions{
        Recursive:    true,
        SkipTagFetch: true,
    })
    if err != nil { return nil }

    result := make([]cache.CacheEntry, 0, len(entries))
    for _, e := range entries {
        result = append(result, cache.CacheEntry{Path: e.Path})
    }
    return result
}
```

同様に `newPrefixPredictor` も同じパターンで変更（`ls` / `export` / `exec` の引数補完に使用）。

### 4. `cmd/root.go` — `BackendFactory` 型定義の確認

```go
// BackendFactory は BackendType からバックエンドを生成する関数型。
type BackendFactory func(backend.BackendType) (backend.Backend, error)
```
（すでに定義済みか確認 → なければ追加）

`refPredictor` / `prefixPredictor` 構造体に `factory BackendFactory` フィールドを追加:

```go
type ExecBGLauncher struct{}
// ... 既存コード ...
```

実際には `main.go` の構造体に追加するほうが適切。

### 5. `main.go` — BackendFactory を補完前に作成

```go
// 9. → 2.5 に移動: BackendFactory を補完前に作成
factory := newBackendFactory(cfg)  // ← kongplete.Complete() の前に移動

// ... existing cacheStore, bgLauncher ...

kongplete.Complete(parser,
    kongplete.WithPredictor("ref", &refPredictor{
        store:      cacheStore,
        bgLauncher: bgLauncher,
        factory:    factory,   // ← 追加
    }),
    kongplete.WithPredictor("prefix", &prefixPredictor{
        store:      cacheStore,
        bgLauncher: bgLauncher,
        factory:    factory,   // ← 追加
    }),
)

// ... kong.Parse() ... ApplyCLIOverrides ...

// BackendFactory は補完時の cfg を使って作成済みなので再作成不要
// ただし ApplyCLIOverrides で cfg が変わるため、コマンド実行時は再作成が必要
factory = newBackendFactory(cfg)  // CLIフラグ反映後に再作成
```

**注意**: 補完時の factory は CLIフラグ（`--region`, `--profile`）を反映しないが、
これは既存のキャッシュ補完と同じ制限で許容範囲。

### 6. `SkipTagFetch: true` を使う箇所（StoreMode 不要なコードパス）

| 呼び出し元 | SkipTagFetch | 理由 |
|-----------|-------------|------|
| 補完時 `fetchLive()` | `true` | パスのみ必要 |
| `cmd/cache.go` `CacheRefreshCmd.Run()` | `true` | キャッシュはパスのみ |
| `cmd/ls.go` `LsCmd.Run()` | `true` | パスのみ表示・キャッシュ保存（StoreMode 不要） |
| `cmd/export.go` `buildVars()` | `false` (デフォルト) | StoreMode で JSON/raw デコードが必要 |
| `cmd/exec.go` `ExecCmd.Run()` | `false` (デフォルト) | export 経由で使用 |

```go
// cache.go
entries, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{
    Recursive:    true,
    SkipTagFetch: true,
})

// ls.go
entries, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{
    Recursive:    c.Recursive,
    SkipTagFetch: true,
})

// export.go — 変更なし（StoreMode が必要）
entries, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{Recursive: true})
```

---

## 変更の影響分析

| 機能 | 変更 | 影響 |
|------|------|------|
| `bundr get ps:/` [Tab] 初回 | リアルタイムAPI→候補表示 | ✅ 改善 |
| `bundr get ps:/` [Tab] 2回目以降 | キャッシュから返す | 変更なし |
| `bundr ls ps:/str` [Tab] 初回 | リアルタイムAPI→候補表示 | ✅ 改善 |
| `bundr export ps:/str` [Tab] 初回 | リアルタイムAPI→候補表示 | ✅ 改善 |
| `bundr exec --from ps:/str` [Tab] 初回 | リアルタイムAPI→候補表示 | ✅ 改善 |
| `bundr ls ps:/` 実行 | SkipTagFetch=true で高速化 | ✅ 改善 |
| `bundr cache refresh ps:/` | SkipTagFetch=true で高速化 | ✅ 改善 |
| `bundr export ps:/` 実行 | 変更なし（StoreMode 必要） | 変更なし |
| `sm:` 補完 | 同様にリアルタイムAPI対応 | ✅ 改善 |

## 検証方法

```bash
# 1. ビルド
go build -o /tmp/bundr_test .

# 2. 全テスト確認
go test ./...

# 3. 補完動作確認（キャッシュを削除した状態で）
# ※ ~/.cache/bundr/ の json ファイルを削除してから:
COMP_LINE="bundr get ps:/" COMP_POINT=17 /tmp/bundr_test get ps:/
# → キャッシュなしでも候補が返ること

# 4. sm: も確認
COMP_LINE="bundr get sm:" COMP_POINT=14 /tmp/bundr_test get sm:

# 5. Homebrew 版で実際にTab補完を確認（新シェルで）
```
