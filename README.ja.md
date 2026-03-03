# bundr

AWS Parameter Store と Secrets Manager を統合して操作する Go CLI。

[![Release](https://img.shields.io/github/v/release/youyo/bundr)](https://github.com/youyo/bundr/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/youyo/bundr)](https://goreportcard.com/report/github.com/youyo/bundr)

[English](README.md)

## インストール

### GitHub Actions

```yaml
steps:
  - uses: youyo/bundr@v0.6
```

特定バージョンを固定する場合:

```yaml
steps:
  - uses: youyo/bundr@v0.6.0
    with:
      bundr-version: v0.6.0
```

### Homebrew（推奨）

```bash
brew install youyo/tap/bundr
```

### GitHub Releases

[リリースページ](https://github.com/youyo/bundr/releases) からバイナリをダウンロード。

```bash
# ワンライナー（Linux/macOS）
curl -sSfL https://raw.githubusercontent.com/youyo/bundr/main/scripts/install.sh | bash

# または手動インストール:
# macOS (Apple Silicon)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_darwin_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# macOS (Intel)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_darwin_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (x86_64)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_linux_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (ARM64)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_linux_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/
```

### go install

```bash
go install github.com/youyo/bundr@latest
```

## クイックスタート

```bash
# 1. 値を保存
bundr put ps:/myapp/db_host --value localhost --store raw

# 機密値を SecureString として保存
bundr put ps:/myapp/api_key --value s3cr3t --store raw --secure

# 2. 値を取得
bundr get ps:/myapp/db_host

# 3. プレフィックス配下の一覧を表示
bundr ls ps:/myapp/

# 4. 環境変数としてエクスポート
eval "$(bundr export ps:/myapp/ --format shell)"

# 5. 環境変数を注入してコマンドを実行
bundr exec --from ps:/myapp/ -- node app.js
```

## Ref 構文

| Ref | バックエンド | 説明 |
|-----|------------|------|
| `ps:/path/to/key` | SSM Parameter Store | デフォルトは Standard tier。`--tier advanced` で最大 8KB |
| `sm:secret-id` | Secrets Manager | シークレット（バージョン管理あり）|

## コマンドリファレンス

### bundr put

値を保存します。

```bash
bundr put <ref> --value <string> --store raw|json [--secure] [--region <region>] [--profile <profile>]
```

| オプション | 説明 |
|----------|------|
| `--value` | 保存する値 |
| `--store` | ストレージモード（`raw` または `json`）|
| `--secure` | SSM SecureString タイプを使用（SSM Parameter Store のみ）|
| `--kms-key-id` | KMS キー ID または ARN |

```bash
# 文字列を raw モードで保存
bundr put ps:/app/db_host --value localhost --store raw

# JSON スカラーとして保存
bundr put ps:/app/db_port --value 5432 --store json

# SecureString として保存
bundr put ps:/app/api_key --value s3cr3t --store raw --secure
```

### bundr get

値を取得します。

```bash
bundr get <ref> [--raw|--json] [--region <region>] [--profile <profile>]
```

```bash
bundr get ps:/app/db_host
bundr get ps:/app/db_port --raw
```

### bundr export

プレフィックス配下のパラメータを環境変数形式で出力します。

```bash
bundr export <prefix> --format shell|dotenv|direnv [--recursive] [--upper] [--no-flatten] [--array-mode join|index|json]
```

| フォーマット | 出力形式 |
|------------|---------|
| `shell` | `export KEY=value` |
| `dotenv` | `KEY=value` |
| `direnv` | `export KEY=value` |

| フラグ | デフォルト | 説明 |
|------|---------|------|
| `--format` | | 出力フォーマット（必須）|
| `--recursive` | false | サブディレクトリを再帰的に展開 |
| `--upper` | true | 変数名を大文字化 |
| `--flatten-delim` | `_` | フラット化キーの区切り文字 |
| `--no-flatten` | false | JSON キーのフラット化を無効化 |
| `--array-mode` | `join` | 配列処理モード（join/index/json）|
| `--array-join-delim` | `,` | join モードの区切り文字 |

```bash
# シェルに export
eval "$(bundr export ps:/app/ --format shell)"

# .env ファイルに書き出し
bundr export ps:/app/ --format dotenv > .env

# direnv に対応
bundr export ps:/app/ --format direnv > .envrc
```

### bundr ls

プレフィックス配下のパラメータ一覧を表示します。

```bash
bundr ls <prefix> [--recursive]
```

```bash
bundr ls ps:/app/
bundr ls sm:myapp/             # Secrets Manager のプレフィックス
bundr ls sm:                   # Secrets Manager の全シークレット
bundr ls ps:/app/ --recursive  # サブディレクトリを再帰的に展開
```

### bundr exec

環境変数を注入してコマンドを実行します。複数の `--from` を指定でき、後のプレフィックスが優先されます。

> **Note**: v0.1.x では `bundr run` でしたが、v0.2.0 から `bundr exec` にリネームされました。

```bash
bundr exec [--from <prefix>]... -- <command> [args...]
```

```bash
# 単一プレフィックスから環境変数を注入
bundr exec --from ps:/app/ -- node server.js

# 複数プレフィックス（後者が優先）
bundr exec --from ps:/common/ --from ps:/app/prod/ -- python main.py
```

### bundr jsonize

1つ以上のプレフィックス配下のパラメータを JSON オブジェクトに変換して stdout に出力します。`--to` で保存先を指定することもできます。

```bash
bundr jsonize --frompath <prefix|ref> [--frompath <prefix|ref>]... [--to <ref>] [--force] [--compact]
```

| オプション | 説明 |
|----------|------|
| `--frompath` | 読み込み元の SSM プレフィックスまたは末端パラメータ（リーフ ref）（複数指定可）|
| `--to` | 保存先の ref（省略時は stdout に出力）|
| `--store` | 保存先のストレージモード（raw/json、デフォルト: json。`--to` 必須）|
| `--value-type` | 保存先の値タイプ（string/secure、デフォルト: string。`--to` 必須）|
| `--force` | 保存先が既に存在する場合に上書き（`--to` と合わせて使用）|
| `--compact` | インデントなしのコンパクト JSON で出力 |

```bash
# stdout に出力（デフォルト）
bundr jsonize --frompath ps:/app/

# 複数プレフィックスを統合
bundr jsonize --frompath ps:/app/db/ --frompath ps:/app/api/

# 末端パラメータ（リーフ）を指定
bundr jsonize --frompath ps:/app/db_host

# Secrets Manager に保存
bundr jsonize --frompath ps:/app/ --to sm:myapp-config

# 既存シークレットを上書き
bundr jsonize --frompath ps:/app/ --to sm:myapp-config --force
```

### bundr completion

シェル補完スクリプトを出力する。

```bash
bundr completion bash|zsh|fish
```

```bash
# Zsh に補完を追加
eval "$(bundr completion zsh)"

# Bash に補完を追加
eval "$(bundr completion bash)"

# Fish に補完を追加
bundr completion fish | source
```

Tab 補完でパラメータ階層を1段ずつたどれます:

```bash
bundr get ps:/<TAB>            # ps:/app/  ps:/config/
bundr get ps:/app/<TAB>        # ps:/app/prod/  ps:/app/stg/
bundr get ps:/app/prod/<TAB>   # ps:/app/prod/DB_HOST  ps:/app/prod/DB_PORT
```

### bundr cache

パラメータ一覧キャッシュを管理します（Tab 補完を高速化するため自動更新されます）。

```bash
bundr cache refresh                 # 全バックエンドを更新
bundr cache refresh ps:/app/        # 特定の Parameter Store プレフィックスを更新
bundr cache refresh sm:             # Secrets Manager の全シークレットを更新
bundr cache clear                   # ローカルキャッシュを完全に削除
```

## レシピ

### SecureString で機密パラメータを保存

SSM Parameter Store の SecureString タイプを使用してパラメータを暗号化して保存します:

```bash
bundr put ps:/app/api_key --value s3cr3t --store raw --secure
bundr put ps:/app/db_pass --value MyP@ss --store raw --secure
```

### GitHub Actions でのシークレット注入

```yaml
- name: AWS パラメータを注入してデプロイ
  run: bundr exec --from ps:/myapp/prod/ -- ./deploy.sh
  env:
    AWS_REGION: ap-northeast-1
```

### JSON フラット化と jsonize による往復変換

```bash
# 個別パラメータを書き込む
bundr put ps:/app/db_host --value localhost --store raw
bundr put ps:/app/db_port --value 5432 --store json

# Secrets Manager に JSON オブジェクトとしてまとめて保存
bundr jsonize --frompath ps:/app/ --to sm:myapp-config

# JSON を読み戻す
bundr get sm:myapp-config
```

### 配列処理モードの比較

JSON 値が配列の場合、`--array-mode` で出力形式を制御できます:

| モード | 入力 | 出力 |
|------|-----|------|
| `join`（デフォルト）| `["a","b","c"]` | `ITEMS=a,b,c` |
| `index` | `["a","b","c"]` | `ITEMS_0=a`, `ITEMS_1=b`, `ITEMS_2=c` |
| `json` | `["a","b","c"]` | `ITEMS=["a","b","c"]` |

```bash
# join モード（デフォルト）
bundr export ps:/app/ --format shell --array-mode join

# index モード
bundr export ps:/app/ --format shell --array-mode index

# json モード（配列を JSON のままで出力）
bundr export ps:/app/ --format shell --array-mode json
```

### キャッシュのリセット

補完キャッシュを完全にリセットする手順:

```bash
bundr cache clear          # キャッシュを削除
bundr cache refresh ps:/   # Parameter Store を再取得
bundr cache refresh sm:    # Secrets Manager を再取得
```

### 複数プレフィックスのマージ

`exec` コマンドは複数の `--from` を受け付け、後のプレフィックスが優先されます:

```bash
# 共通設定 + 環境固有設定（後者が優先）
bundr exec --from ps:/common/ --from ps:/app/prod/ -- python main.py
```

## 詳細トピック

### JSON フラット化

`store-mode=json` で保存した JSON 値は、エクスポート時に区切り文字（デフォルト `_`）でキーを展開して環境変数に変換します。

```bash
# 例: ps:/app/server に '{"host":"0.0.0.0","port":8080}' が json モードで保存されている場合
bundr export ps:/app/ --format shell
# export DB_HOST=localhost
# export SERVER_HOST=0.0.0.0
# export SERVER_PORT=8080
```

`--no-flatten` で無効化:

```bash
bundr export ps:/app/ --format shell --no-flatten
# export SERVER={"host":"0.0.0.0","port":8080}
```

### Tab 補完の既知制限

- ローカルビルド（`./bundr`）では補完が発火しない。`$PATH` に `bundr` として配置する必要がある
- aws-vault や AWS SSO などの短期クレデンシャルは補完プロセスに引き継がれないことがあります。クレデンシャルが有効なうちに、事前キャッシュを作成しておくことを推奨します:

```bash
bundr cache refresh ps:/
bundr cache refresh sm:
```

## グローバルフラグ

| フラグ | 環境変数 | 説明 |
|------|---------|------|
| `--region` | `AWS_REGION`, `BUNDR_AWS_REGION` | AWS リージョン |
| `--profile` | `AWS_PROFILE`, `BUNDR_AWS_PROFILE` | AWS プロファイル名 |
| `--kms-key-id` | `BUNDR_KMS_KEY_ID`, `BUNDR_AWS_KMS_KEY_ID` | KMS キー ID または ARN |

## 設定

### 設定優先順位

設定は以下の順序で適用されます（優先度が高いほど後の番号に記載されています）:

1. グローバル設定ファイル（`~/.config/bundr/config.toml`）
2. プロジェクト設定ファイル（`.bundr.toml`）
3. 標準 AWS 環境変数（`AWS_REGION`, `AWS_PROFILE`）
4. bundr 固有環境変数（`BUNDR_AWS_REGION`, `BUNDR_AWS_PROFILE`）— `AWS_*` を上書き
5. CLI フラグ（`--region`, `--profile`, `--kms-key-id`）— 最高優先

### プロジェクト設定ファイル（.bundr.toml）

プロジェクトのルートディレクトリに `.bundr.toml` を配置することで、プロジェクト固有の設定が可能です。

```toml
[aws]
region = "ap-northeast-1"
profile = "my-profile"
kms_key_id = "alias/my-key"
```

### グローバル設定ファイル（~/.config/bundr/config.toml）

```toml
[aws]
region = "ap-northeast-1"
profile = "default"
```

### 環境変数一覧

| 変数名 | 説明 |
|-------|------|
| `AWS_REGION` | AWS リージョン（標準） |
| `AWS_PROFILE` | AWS プロファイル名（標準） |
| `BUNDR_AWS_REGION` | AWS リージョン（`AWS_REGION` より優先） |
| `BUNDR_AWS_PROFILE` | AWS プロファイル名（`AWS_PROFILE` より優先） |
| `BUNDR_KMS_KEY_ID` | KMS キー ID または ARN |
| `BUNDR_AWS_KMS_KEY_ID` | `BUNDR_KMS_KEY_ID` のエイリアス |

## AWS 認証設定

bundr は AWS SDK v2 の標準認証チェーンを使用します:

1. 環境変数（`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`）
2. AWS プロファイル（`~/.aws/credentials`, `~/.aws/config`）
3. IAM ロール（EC2, ECS, Lambda 等）

特定のプロファイルを使用する場合:

```bash
# CLI フラグで指定
bundr ls ps:/app/ --profile my-profile

# 環境変数で指定
AWS_PROFILE=my-profile bundr ls ps:/app/

# 設定ファイルで指定（.bundr.toml）
# [aws]
# profile = "my-profile"
```

## タグスキーマ

bundr が管理するすべてのパラメータには以下のタグが自動的に付与されます:

| タグ名 | 値 | 説明 |
|-------|---|------|
| `cli` | `bundr` | bundr が管理するリソースを識別 |
| `cli-store-mode` | `raw` または `json` | ストレージモード（bundr が `get`/`export` 時にデコード方式を自動判定）|
| `cli-schema` | `v1` | スキーマバージョン |

## ライセンス

MIT

---

[English](README.md)
