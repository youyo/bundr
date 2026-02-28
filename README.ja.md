# bundr

AWS Parameter Store と Secrets Manager を統合する単一バイナリ Go CLI。

[![Release](https://img.shields.io/github/v/release/youyo/bundr)](https://github.com/youyo/bundr/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/youyo/bundr)](https://goreportcard.com/report/github.com/youyo/bundr)

[English](README.md)

## インストール

### Homebrew（推奨）

```bash
brew install youyo/tap/bundr
```

### GitHub Releases

[リリースページ](https://github.com/youyo/bundr/releases) からバイナリをダウンロード。

```bash
# macOS (Apple Silicon)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Darwin_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Darwin_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Linux_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (ARM64)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Linux_arm64.tar.gz | tar xz
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
| `ps:/path/to/key` | SSM Parameter Store (Standard) | 標準パラメータ（最大4KB）|
| `psa:/path/to/key` | SSM Parameter Store (Advanced) | 拡張パラメータ（最大8KB）|
| `sm:secret-id` | Secrets Manager | シークレット（バージョン管理あり）|

## コマンドリファレンス

### bundr put

値を保存する。

```bash
bundr put <ref> --value <string> --store raw|json [--region <region>] [--profile <profile>]
```

| オプション | 説明 |
|----------|------|
| `--value` | 保存する値 |
| `--store` | ストレージモード（`raw` または `json`）|
| `--kms-key-id` | KMS キー ID または ARN |

```bash
# 文字列を raw モードで保存
bundr put ps:/app/db_host --value localhost --store raw

# JSON スカラーとして保存
bundr put ps:/app/db_port --value 5432 --store json
```

### bundr get

値を取得する。

```bash
bundr get <ref> [--raw|--json] [--region <region>] [--profile <profile>]
```

```bash
bundr get ps:/app/db_host
bundr get ps:/app/db_port --raw
```

### bundr export

プレフィックス配下のパラメータを環境変数形式で出力する。

```bash
bundr export <prefix> --format shell|dotenv|direnv [--no-recursive]
```

| フォーマット | 出力形式 |
|------------|---------|
| `shell` | `export KEY=value` |
| `dotenv` | `KEY=value` |
| `direnv` | `export KEY=value` |

```bash
# シェルに export
eval "$(bundr export ps:/app/ --format shell)"

# .env ファイルに書き出し
bundr export ps:/app/ --format dotenv > .env

# direnv に対応
bundr export ps:/app/ --format direnv > .envrc
```

### bundr ls

プレフィックス配下のパラメータ一覧を表示する。

```bash
bundr ls <prefix> [--no-recursive]
```

```bash
bundr ls ps:/app/
bundr ls ps:/app/ --no-recursive  # サブディレクトリを展開しない
```

### bundr exec

環境変数を注入してコマンドを実行する。複数の `--from` を指定でき、後のプレフィックスが優先される。

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

プレフィックス配下のパラメータを JSON として1つのキーにまとめて保存する。

```bash
bundr jsonize <target-ref> --frompath <prefix> [--force]
```

```bash
bundr jsonize sm:myapp-config --frompath ps:/app/
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

### bundr cache

パラメータ一覧キャッシュを管理する（補完高速化のため自動管理）。

```bash
bundr cache refresh
```

## グローバルフラグ

| フラグ | 環境変数 | 説明 |
|------|---------|------|
| `--region` | `AWS_REGION`, `BUNDR_AWS_REGION` | AWS リージョン |
| `--profile` | `AWS_PROFILE`, `BUNDR_AWS_PROFILE` | AWS プロファイル名 |
| `--kms-key-id` | `BUNDR_KMS_KEY_ID`, `BUNDR_AWS_KMS_KEY_ID` | KMS キー ID または ARN |

## 設定

### 設定優先順位

設定は以下の順序で適用される（後の設定が先の設定を上書きする）:

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

## シェル補完

```bash
# Zsh（~/.zshrc に追加）
eval "$(bundr completion zsh)"

# Bash（~/.bashrc に追加）
eval "$(bundr completion bash)"

# Fish（~/.config/fish/config.fish に追加）
bundr completion fish | source
```

## タグスキーマ

bundr が管理するすべてのパラメータには以下のタグが自動的に付与されます:

| タグ名 | 値 | 説明 |
|-------|---|------|
| `cli` | `bundr` | bundr が管理するリソースを識別 |
| `cli-store-mode` | `raw` または `json` | ストレージモード（`get`/`export` 時の自動デコードに使用）|
| `cli-schema` | `v1` | スキーマバージョン |

## ライセンス

MIT

---

[English](README.md)
