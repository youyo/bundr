# Plan: README 英語化・ユースケース追記

## Context

現行の `README.md` は全文日本語で書かれており、以下の問題がある：

1. **国際リーチがない** — OSS として英語が主言語であるべき
2. **ユースケースが薄い** — コマンドリファレンス羅列で「なぜ使うか」が伝わらない
3. **AIっぽい文体** — "強力な", "シームレスに", "統合する" などの過剰表現
4. **バグ**: `exec` コマンドの使い方が実装と不一致（README は位置引数だが実装は `--from` フラグ）
5. **バグ**: 設定優先順位の記述が混乱（リストと注記が矛盾）

## Goals

- `README.md` を英語・自然な文体に書き直す
- 全コマンドの実際のユースケースを追加
- 日本語は `README.ja.md` に移動し、相互リンクを設置

## 実装方針

### ファイル

- `README.md` — 英語版（新規書き直し）
- `README.ja.md` — 日本語版（現 README.md を移動 + バグ修正）

### README.md 構成

```
1. Header: タイトル + バッジ + [日本語](README.ja.md) リンク
2. What it does: 動詞で始まる4〜5 bullet
3. Install: Homebrew > go install > Manual binary
4. Quick start: 5ステップ（正しい exec 構文で）
5. Ref syntax: テーブル
6. Recipes: コマンドごとのユースケース（メインセクション）
7. Command reference: 全フラグテーブル
8. Configuration: 優先順位 + TOML + env vars
9. AWS auth: 認証チェーン説明
10. Shell completion: 永続化方法
11. License + 日本語リンク
```

### 文体指針（AIっぽさ排除）

**禁止**:
- "powerful", "seamless", "streamline", "effortless", "robust", "unified"
- "This allows you to...", "Making it easy to...", "Simply", "Just"

**推奨**:
- 動詞から始まる短文: `Stores a value to SSM Parameter Store.`
- コマンドの挙動を主語に: `The later --from prefix takes precedence.`
- `git`, `docker`, `kubectl` に近い冷静・簡潔・正確なスタイル

### 修正必須バグ

1. **exec コマンドのフラグ**: `cmd/exec.go:49` で `From []string` は `short:"f" name:"from"` フラグ
   - 誤: `bundr exec ps:/myapp/ -- node app.js`
   - 正: `bundr exec --from ps:/myapp/ -- node app.js`

2. **設定優先順位**: `internal/config/config.go:110-128` で `applyEnvOverrides` は:
   - AWS_REGION/PROFILE を先に適用 → BUNDR_AWS_REGION/PROFILE で上書き
   - 正しい優先順位: CLI flags > **BUNDR_AWS_*** > **AWS_*** > .bundr.toml > config.toml

### Recipes セクション内容（各コマンドのユースケース）

各コマンドで「Problem → Solution → コード例」形式で記述

**put**:
- 文字列パラメータを raw で保存
- JSON エンコードして数値を保存
- Secrets Manager への保存
- SecureString（暗号化）として保存

**get**:
- 値を取得して出力
- スクリプト内で変数代入: `DB_HOST=$(bundr get ps:/app/prod/DB_HOST)`
- `--raw` でデバッグ（タグ無視）

**ls**:
- prefix 配下の全パス一覧
- `--no-recursive` で直下のみ
- パイプで活用: `bundr ls ps:/app/ | wc -l`

**export**:
- `eval "$(bundr export ps:/app/ --format shell)"` でシェルに適用
- `.env` ファイル生成
- direnv 連携
- CI/CD パイプライン内での使用例

**exec**:
- 単一プレフィックスから注入
- 複数プレフィックス（後者が優先）: `bundr exec --from ps:/common/ --from ps:/app/prod/ -- python main.py`
- 注入された変数確認: `bundr exec --from ps:/app/ -- env | grep DB`

**jsonize**:
- SSM の複数パラメータを1つの SM シークレットに集約

**completion**:
- `eval "$(bundr completion zsh)"` で即時有効化
- `.zshrc` / `.bashrc` / fish への永続化

**cache**:
- `bundr cache refresh ps:/app/` で手動更新
- 新規パラメータ追加後の使い方

### README.ja.md 方針

現在の `README.md` 内容をベースに：
- 英語 README へのリンクを冒頭に追加
- `exec` コマンドのバグ修正（`--from` フラグ形式に）
- 設定優先順位の記述を修正（BUNDR_AWS_* > AWS_* を正確に）

## 変更ファイル

| ファイル | 操作 |
|---------|------|
| `README.md` | 全文英語に書き直し |
| `README.ja.md` | 新規作成（現 README.md の内容 + バグ修正） |

## Verification

- README.md を GitHub でプレビューして表示確認
- `exec` の使い方が `cmd/exec.go` と一致しているか目視確認
- 設定優先順位が `internal/config/config.go` と一致しているか目視確認
- `README.ja.md` のリンクが README.md から正しく張られているか確認
