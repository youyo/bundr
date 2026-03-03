# Plan: Tab補完末尾スペース修正 + キャッシュのアカウント/リージョン別スコープ化

## Context

ユーザーが報告した2つの問題を修正する：

1. **Tab補完の末尾スペース問題**: 補完スクリプトに `-o nospace` がないため、bash/zsh が各候補の末尾にスペースを自動追加する。これにより階層ナビゲーションの Tab 連打ができない。
2. **キャッシュの混在問題**: キャッシュファイルが `~/.cache/bundr/{backendType}.json` のみで命名されており、AWS リージョン・プロファイルが異なってもファイルを共有してしまう。ユーザーは古いデータを見続ける原因にもなる。

---

## 調査結果

### Issue 1: 末尾スペース

- `cmd/completion.go:31-33`: bash/zsh の `complete -C {bin} bundr` に `-o nospace` がない
- `-o nospace` を追加すると bash/zsh は末尾スペースを自動追加しなくなり、`ps:/app/prod/` のような中間ノードにも Tab が再び効く
- **トレードオフ**: 最終的なリーフノード（`ps:/app/prod/DB_HOST`）を完成させた後、ユーザーが手動でスペースを1回押す必要がある。これは許容範囲内（ディレクトリ補完と同じ動作）

### Issue 2: キャッシュスコープ

- `internal/cache/cache.go:94`: `filepath.Join(s.baseDir, backendType+".json")` のみでファイル名決定
- **識別子の決定**:
  - `AWS_ACCESS_KEY_ID` 環境変数が設定されている場合: その先頭8文字を使用（`ak-AKIA1234` 形式）
    - `aws login` / `aws-vault exec` / `assume-role` ツールなどの短期クレデンシャルに対応
    - アクセスキーはアカウントに紐づくため、アカウント切り替えを自動で識別
  - フォールバック: `AWS_PROFILE` または config のプロファイル名
  - 最終フォールバック: `"default"`
- ファイル名形式: `{backendType}-{sanitize(region)}-{identifier}.json`
- **一貫性の確保**:
  - 補完時: `config.Load()` + env vars から識別子を計算 → キャッシュ読み込み
  - BGLauncher: 同じ env vars を子プロセスに渡すため、書き込みも同じスコープ ✓
  - `bundr --region X cache refresh`: env var スコープのファイルに書き込む（CLI フラグはキャッシュスコープに影響しない）。この動作を文書化する
- `go.mod` に `github.com/aws/aws-sdk-go-v2/service/sts` がすでに indirect で存在するが、補完は STS API なしで識別子を計算できるため使用しない
- **短期クレデンシャルのライフサイクル**: アクセスキーが更新されると新しいキャッシュファイルが作成される（正常動作。BG refresh が自動で新ファイルを作成する）
- **既存キャッシュ**: 旧形式 (`ps.json` 等) は orphaned になるが無害。`bundr cache clear` で全削除可能にする
- **Store インターフェース**: `Clear() error` メソッドを追加。`Write` シグネチャは変更しない

---

## 実装計画

### Work Unit 1: Tab補完末尾スペース修正（独立）

**対象ファイル**: `cmd/completion.go`, `cmd/completion_test.go`

```go
// Before
case "bash":
    script = fmt.Sprintf("complete -C %s bundr\n", bin)
case "zsh":
    script = fmt.Sprintf("autoload -U +X bashcompinit && bashcompinit\ncomplete -C %s bundr\n", bin)

// After
case "bash":
    script = fmt.Sprintf("complete -o nospace -C %s bundr\n", bin)
case "zsh":
    script = fmt.Sprintf("autoload -U +X bashcompinit && bashcompinit\ncomplete -o nospace -C %s bundr\n", bin)
```

fish は `test -z (commandline -cp)[-1]; and set COMP_LINE "$COMP_LINE "` が既にある（条件付きスペース付加）。変更なし。

テスト更新: `cmd/completion_test.go` の bash/zsh スクリプト確認テストに `-o nospace` を含むことをアサート。

---

### Work Unit 2: キャッシュスコープ化 + cache clear コマンド（独立）

#### 2a: `internal/cache/cache.go` の変更

1. `FileStore` に `region, identifier string` フィールド追加
2. `sanitizeCacheKey(s string) string` ヘルパー追加:
   - 空文字 → `"default"`
   - 英数字とハイフン以外を `-` に置換
3. `cacheFilePath(backendType string) string` ヘルパー追加:
   - `filepath.Join(s.baseDir, backendType+"-"+sanitize(region)+"-"+identifier+".json")`
4. `NewFileStore(region, identifier string)` に変更
5. `NewFileStoreWithDir(dir string)` はそのまま（region="", identifier="" → "default-default"）
6. `Read`, `Write`, `LastRefreshedAt`, `readFile` で `cacheFilePath` を使用
7. `Store` インターフェースに `Clear() error` 追加
8. `FileStore.Clear()` 実装: `~/.cache/bundr/*.json` を全削除（旧形式も含む）
9. `NoopStore.Clear()` 実装: nil を返す

#### 2b-new: `internal/cache/cache.go` に `CacheIdentifier()` 関数追加

```go
// CacheIdentifier は AWS アクセスキー (env) またはプロファイル名からキャッシュ識別子を返す。
// aws login や aws-vault など短期クレデンシャル使用時も正しくスコープ分離する。
func CacheIdentifier(profile string) string {
    if ak := os.Getenv("AWS_ACCESS_KEY_ID"); ak != "" {
        if len(ak) >= 8 {
            return "ak-" + ak[:8]
        }
        return "ak-" + ak
    }
    if profile != "" {
        return sanitizeCacheKey(profile)
    }
    return "default"
}
```

#### 2c: `main.go` の変更

```go
// Before
if fs, fsErr := cache.NewFileStore(); fsErr != nil {
// After
identifier := cache.CacheIdentifier(cfg.AWS.Profile)
if fs, fsErr := cache.NewFileStore(cfg.AWS.Region, identifier); fsErr != nil {
```

#### 2d: `cmd/cache.go` の変更

`CacheClearCmd` を追加:
```go
type CacheClearCmd struct{}
func (c *CacheClearCmd) Run(appCtx *Context) error {
    return appCtx.CacheStore.Clear()
}
```

#### 2e: `cmd/root.go` の変更

`CacheCmd` に `Clear CacheClearCmd` フィールド追加:
```go
type CacheCmd struct {
    Refresh CacheRefreshCmd `cmd:"" help:"Refresh the local cache by fetching paths from AWS."`
    Clear   CacheClearCmd   `cmd:"" help:"Clear all local cache files."`
}
```

#### 更新が必要なテスト

**`internal/cache/cache_test.go`**:
- `TestFileStore_Write_Filename`: `psa.json` → `psa-default-default.json`
- `TestFileStore_Write_JSONSchema`: `ps.json` → `ps-default-default.json`
- `TestFileStore_Read_CorruptedJSON`: ファイル作成先を `ps-default-default.json` に
- `TestFileStore_Read_SchemaMismatch`: 同上
- `TestFileStore_Read_CorruptedFile_ReturnsError`: 同上
- `TestFileStore_Write_DoesNotStoreSecretValues`: 同上
- `TestFileStore_Read_NoReadPermission`: 同上
- `TestNewFileStore_XDGCacheHome` / `TestNewFileStore_DefaultXDGPath` / `TestNewFileStore_RelativeXDGCacheHome`: `NewFileStore()` → `NewFileStore("", "")` に引数追加
- `TestNoopStore_Write`: `Store.Clear()` を実装する必要があるため MockStore 更新
- `Clear()` のテストケースを追加

**`cmd/cache_test.go`**:
- `MockStore` に `ClearFunc func() error` + `ClearCalls int` + `Clear()` メソッド追加
- `CacheClearCmd` のテストを追加

---

## ファイル名例

```
~/.cache/bundr/
├── ps-default-default.json              # region なし、profile なし
├── ps-us-east-1-default.json            # AWS_REGION=us-east-1, credential なし
├── ps-ap-northeast-1-stratalog.json     # AWS_REGION=ap-northeast-1, AWS_PROFILE=stratalog
├── sm-ap-northeast-1-stratalog.json
├── ps-ap-northeast-1-ak-ASIA1234.json   # aws login / aws-vault → AWS_ACCESS_KEY_ID あり
└── ps.json  # 旧形式（bundr cache clear で削除可能）
```

**識別子の決定ロジック**:
1. `AWS_ACCESS_KEY_ID` 環境変数がある → `ak-{先頭8文字}`
2. profile 設定がある → プロファイル名
3. それ以外 → `"default"`

---

## E2E 検証手順

### Unit 1 (末尾スペース)
```bash
go test ./cmd/...
# 動作確認（要 bundr ビルド + シェルソース）:
go build -o /tmp/bundr ./...
eval "$(/tmp/bundr completion bash)"
# bundr get ps:<TAB> → ps:/app/ が補完され末尾スペースなし → 続けて TAB 連打可能
```

### Unit 2 (キャッシュスコープ)
```bash
go test ./internal/cache/... ./cmd/...
# ファイル名確認:
go build -o /tmp/bundr ./...
AWS_REGION=us-east-1 /tmp/bundr cache refresh ps:/  # → ps-us-east-1-default.json を作成
ls ~/.cache/bundr/
# → ps-us-east-1-default.json が存在すること
/tmp/bundr cache clear  # → *.json が全削除されること
ls ~/.cache/bundr/
```

---

## 注意事項

- `--region`/`--profile` CLI フラグはキャッシュスコープに影響しない（env vars のみがスコープを決定）
- 旧形式キャッシュ (`ps.json` 等) は `bundr cache clear` で削除可能
- ユーザーが既存の補完スクリプトを `.bashrc`/`.zshrc` でソースしている場合、`eval "$(bundr completion bash)"` を再実行して `-o nospace` を含む新しいスクリプトを有効にする必要がある
