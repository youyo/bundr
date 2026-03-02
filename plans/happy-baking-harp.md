# Plan: Tab補完の自動動作 + cache refresh 不要化 + ls デフォルト非再帰化

## Context

**ユーザーの問題（3点）**:
1. Tab 連打でターゲット補完できない（初回は常に空）
2. `bundr cache refresh` を手動実行しないと補完が動かない
3. `ls` のデフォルトが再帰で、非再帰を `--no-recursive` で指定するのが不便

**根本原因（問題1・2）**:

現在 BG キャッシュリフレッシュは **補完時（predictor.go）にのみ**トリガーされる。

```
[Tab 1回目] → cache miss → BG refresh 起動（非同期） → 空候補返す
             ↑ここでユーザーは待つ必要がある
[Tab 2回目] → cache hit → 候補表示
```

`ls`・`export`・`exec` コマンドは AWS から GetByPrefix でデータを取得しているにも関わらず、
その結果をキャッシュに書き込んでいない。コマンド実行後にすぐ Tab を押しても候補が出ない。

**解決策（問題1・2）**:

コマンド実行時にフェッチ済みエントリを即座にキャッシュへ書き込む（余分な API コールなし）。

```
[ls ps:/app/ 実行] → GetByPrefix → 結果取得 → キャッシュ即時書き込み
[get ps:/app/<Tab>] → cache hit → 即座に候補表示 ← ここが改善される
```

**キャッシュの鮮度（「古いキャッシュを見続けている」について）**:

現在の BG refresh アーキテクチャ（predictor.go）:
- Tab 押下時: `time.Since(lastRefresh) > 10*time.Second` なら BG refresh 起動
- 10秒スロットリングにより、過剰な AWS API コールを防止
- キャッシュに TTL なし（削除まで永続）

今回の改善後:
- ls/export/exec 実行 → 即時同期書き込み（LastRefreshedAt 更新）
- 次の Tab（10秒後以降）→ BG refresh が自動的に最新化
- これにより「ls してすぐ Tab」が常に動く

**問題3（ls デフォルト変更）**:

現在: デフォルト再帰、`--no-recursive` で非再帰
変更: デフォルト非再帰、`--recursive` で再帰（Unix ls との一貫性）

---

## 変更パターン：コマンド実行後のキャッシュ書き込み（ls / buildVars 共通）

```go
// b.GetByPrefix(...) 成功直後に追加
if appCtx.CacheStore != nil {
    cacheEntries := make([]cache.CacheEntry, 0, len(entries))
    for _, e := range entries {
        cacheEntries = append(cacheEntries, cache.CacheEntry{
            Path:      e.Path,
            StoreMode: e.StoreMode,
        })
    }
    _ = appCtx.CacheStore.Write(string(ref.Type), cacheEntries)
}
```

- `Write` はバックエンド全エントリを上書き（マージではない）
- エラーは無視（キャッシュ更新失敗でコマンド本体を失敗させない）
- `cmd/cache.go` の CacheRefreshCmd.Run() が同じ変換パターンを実装済み（参考）

---

## 作業ユニット（3ユニット、独立して実装可能）

### Unit 1: ls コマンド改善
**ファイル**: `cmd/ls.go`, `cmd/ls_test.go`

**変更 A: デフォルト非再帰化**

```go
// Before
type LsCmd struct {
    From        string `arg:"" required:"" predictor:"prefix" ...`
    NoRecursive bool   `name:"no-recursive" help:"List only direct children"`
}
// GetByPrefix(..., Recursive: !c.NoRecursive)

// After
type LsCmd struct {
    From      string `arg:"" required:"" predictor:"prefix" ...`
    Recursive bool   `name:"recursive" help:"List recursively (default: non-recursive)"`
}
// GetByPrefix(..., Recursive: c.Recursive)
```

**変更 B: GetByPrefix 後にキャッシュ書き込み**

`entries, err := b.GetByPrefix(...)` の成功後（line 44-47 の後）に上記パターンを追加。
キャッシュ書き込みには `cache` パッケージを import する（既存 import に追加）。

**テスト更新（ls_test.go）**:
- テストケース L-03 の `--no-recursive` → `--recursive` に変更
- テスト用 Context に MockStore を追加
- 新テストケース: ls 実行後に MockStore.Write() が呼ばれること（backendType, entries 確認）
- MockStore は同一パッケージ内の既存実装（`cmd/cache_test.go` 等）を参照して利用

---

### Unit 2: export・exec キャッシュ自動更新
**ファイル**: `cmd/export.go`, `cmd/exec_test.go`, `cmd/export_test.go`

`buildVars()` 関数（`cmd/export.go:29`）は export と exec の両コマンドから共有される。
この関数内の `GetByPrefix()` 成功後（`export.go:44-47` の後）に上記キャッシュ書き込みパターンを追加。

buildVars のシグネチャ: `func buildVars(ctx context.Context, appCtx *Context, opts VarsBuildOptions)`
→ `appCtx *Context` から `appCtx.CacheStore` と `ref.Type` にアクセス可能。

**テスト追加**:
- export_test.go: buildVars 経由でキャッシュが更新されることを確認
- exec_test.go: exec コマンド実行後にキャッシュが更新されることを確認

---

### Unit 3: README ドキュメント更新
**ファイル**: `README.md`, `README.ja.md`

ls セクションの `--no-recursive` を `--recursive` に変更。
具体的に変更する箇所:
- README.md ls セクションの `--no-recursive` 例 → `--recursive` に変更
- README.ja.md 同様
- sm: 例の `--no-recursive` フラグも更新

---

## テスト戦略（E2E 検証）

**Unit テスト（AWS 不要、MockBackend + MockStore）**:

```bash
cd /Users/youyo/src/github.com/youyo/bundr
go test ./...              # 全テスト PASS
go build -o /tmp/bundr ./... # ビルド確認
```

**手動 E2E（AWS 環境がある場合）**:

```bash
# 1. キャッシュクリア
rm -rf ~/.cache/bundr/

# 2. ls 実行でキャッシュ自動生成を確認
/tmp/bundr ls ps:/ 2>/dev/null

# 3. キャッシュが作成されたか確認
ls -la ~/.cache/bundr/
cat ~/.cache/bundr/ps.json | python3 -m json.tool | head -20

# 4. Tab 補完が即座に動くか確認
# （Shell で: /tmp/bundr get ps:/<Tab> が即座に候補を返す）

# 5. ls デフォルト非再帰の確認
# bundr ls ps:/app/ → 直下のキーのみ表示
# bundr ls ps:/app/ --recursive → 全ネストのキーを表示
```

**Unit テストで確認すること**:
1. ls/buildVars 実行後に `cacheStore.Write()` が呼ばれること
2. 渡される entries が GetByPrefix の結果と一致すること
3. `appCtx.CacheStore == nil` でもパニックしないこと
4. `Write()` エラー時にコマンド本体がエラーを返さないこと
5. ls のデフォルトが非再帰になっていること（`--recursive` なし → 直下のみ）

---

## 対象外（今後の検討）

- `get` コマンドのキャッシュ更新: 単一エントリのみなので効果が薄い。BG refresh 起動で対応可
- キャッシュのマージ（merge write）: 現状の上書きで運用上は十分
- 10秒スロットリングの設定化: 現状の固定値で問題なし
- キャッシュ TTL の追加: ls/export/exec 実行後の即時更新で陳腐化を防げる
