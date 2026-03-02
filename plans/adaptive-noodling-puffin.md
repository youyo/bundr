# Plan: Tab補完候補が消えない問題の修正（COMP_POINT 切り捨て）

## Context

### 問題
`bundr get sm:stratalog/preview/slack-app` と完全入力して Tab を押しても、
`sm:stratalog/preview/` と `sm:stratalog/workspace/` という中間ディレクトリが候補として消えない。

### 根本原因の調査結果

#### posener/complete の動作（`complete.go:62-64`）
```go
if point >= 0 && point < len(line) {
    line = line[:point]   // ← COMP_POINT 位置で COMP_LINE を切り取る
}
a := newArgs(line)         // スペース分割で a.Last を決定
```

#### zsh の bashcompinit + complete -C の問題
zsh は `bashcompinit` 経由で bash の `complete -C` をエミュレートするとき、
**`COMP_POINT` を末尾スラッシュの後の位置に設定する**ことがある。

`sm:stratalog/preview/slack-app` を入力した場合：
- 実際の `COMP_POINT` = `"bundr get sm:stratalog/"` の長さ（= 23）← 末尾スラッシュで切れる
- `COMP_LINE[:23]` = `"bundr get sm:stratalog/"`
- `a.Last = "sm:stratalog/"`

この `a.Last` が predictor に渡され:
1. `ParseRef("sm:stratalog/")` → `{Type: "sm", Path: "stratalog/"}`
2. `hierarchicalFilter("stratalog/", entries, "sm")` → `["sm:stratalog/preview/", "sm:stratalog/workspace/"]`
3. フィルタ: `HasPrefix("sm:stratalog/preview/", "sm:stratalog/")` = **true** → 両方通過

#### 検証コマンド（e2e 根拠）
```bash
# 旧動作の再現（COMP_POINT = 23 で末尾スラッシュ切り）
COMP_LINE="bundr get sm:stratalog/preview/slack-app" COMP_POINT=23 ./bundr get sm:stratalog/preview/slack-app
# → sm:stratalog/preview/  sm:stratalog/workspace/  ← バグ

# 期待動作（COMP_POINT = 末尾）
COMP_LINE="bundr get sm:stratalog/preview/slack-app" COMP_POINT=40 ./bundr get sm:stratalog/preview/slack-app
# → sm:stratalog/preview/slack-app  ← 正しい
```

---

## 変更内容（作業単位: 1件）

### 変更ファイル
- `cmd/completion.go`（修正：bash/zsh スクリプト生成の修正）
- `cmd/completion_test.go`（テスト更新）

### 変更詳細

#### `cmd/completion.go` の bash/zsh スクリプト修正

`COMP_POINT` を `${#COMP_LINE}` に強制設定するラッパー関数を補完スクリプトに追加。

**Before（bash）:**
```
complete -o nospace -C /path/to/bundr bundr
```

**After（bash）:**
```
_bundr_complete() { COMP_POINT=${#COMP_LINE} /path/to/bundr "$@"; }
complete -o nospace -C _bundr_complete bundr
```

**Before（zsh）:**
```
autoload -U +X bashcompinit && bashcompinit
complete -o nospace -C /path/to/bundr bundr
```

**After（zsh）:**
```
autoload -U +X bashcompinit && bashcompinit
_bundr_complete() { COMP_POINT=${#COMP_LINE} /path/to/bundr "$@"; }
complete -o nospace -C _bundr_complete bundr
```

**Fish（変更なし）**: fish は `COMP_LINE` / `COMP_POINT` の仕組みが異なるため対象外。

#### `cmd/completion_test.go` の更新

スクリプト内容の期待値テストを更新する（`_bundr_complete` 関数が含まれることを確認）。

---

## 変更規模
- `cmd/completion.go`: +1行（各ケースに `_bundr_complete` 関数1行追加）
- `cmd/completion_test.go`: 期待値文字列の更新

---

## 検証方法

```bash
# ユニットテスト
go test ./cmd/... -v -run "TestCompletionCmd"
go test ./...

# e2e（ビルド後）
go build -o bundr .

# 旧問題の再現確認（COMP_POINT=23 でスラッシュ切り）
COMP_LINE="bundr get sm:stratalog/preview/slack-app" COMP_POINT=23 ./bundr get sm:stratalog/preview/slack-app
# 現在は sm:stratalog/preview/ と sm:stratalog/workspace/ が出る（バグ再現）

# 修正後の動作確認（COMP_POINT=末尾）
COMP_LINE="bundr get sm:stratalog/preview/slack-app" COMP_POINT=40 ./bundr get sm:stratalog/preview/slack-app
# 期待: sm:stratalog/preview/slack-app のみ

# 実際のシェル補完（zsh で eval 後）
eval "$(./bundr completion zsh)"
bundr get sm:stratalog/preview/slack-app<Tab>
# 期待: sm:stratalog/preview/slack-app のみ（候補が消える）
```
