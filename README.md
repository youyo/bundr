# bundr

AWS Parameter Store (Standard/Advanced) と Secrets Manager を統合する単一バイナリ Go CLI。タグをポータブルなメタデータとして使用し、JSON/raw ストレージモードをサポートします。

## インストール

### GitHub Releases からダウンロード

[Releases](https://github.com/youyo/bundr/releases) から最新バイナリをダウンロード:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_darwin_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_darwin_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_linux_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/
```

### go install

```bash
go install github.com/youyo/bundr@latest
```

## クイックスタート

```bash
# Parameter Store に値を保存
bundr put ps:/myapp/prod/DB_HOST --value localhost --store raw

# 値を取得
bundr get ps:/myapp/prod/DB_HOST

# prefix 配下を環境変数形式でエクスポート
bundr export --from ps:/myapp/prod/ --format shell

# prefix 配下から JSON を構築してパラメータに保存
bundr jsonize ps:/myapp/prod/config --frompath ps:/myapp/prod/

# シェル補完をインストール
bundr install-completions --shell zsh
```

## グローバルフラグ

全コマンドで使用可能なグローバルフラグ（設定の最高優先度）:

| フラグ | 環境変数 | 説明 |
|--------|----------|------|
| `--region` | `BUNDR_AWS_REGION` | AWS リージョン |
| `--profile` | `BUNDR_AWS_PROFILE` | AWS プロファイル名 |
| `--kms-key-id` | `BUNDR_AWS_KMS_KEY_ID` | KMS キー ID または ARN |

```bash
bundr --region ap-northeast-1 --profile prod get ps:/myapp/DB_HOST
```

## Ref 構文

| 構文 | バックエンド |
|------|-------------|
| `ps:/path/to/key` | SSM Parameter Store (Standard) |
| `psa:/path/to/key` | SSM Parameter Store (Advanced) |
| `sm:secret-id` | Secrets Manager |

## コマンドリファレンス

### `bundr put <ref> --value <string> --store raw|json`

AWS Parameter Store または Secrets Manager に値を保存します。

```bash
bundr put ps:/myapp/prod/DB_PORT --value 5432 --store raw
bundr put ps:/myapp/prod/DB_CONFIG --value '{"host":"localhost"}' --store json
```

### `bundr get <ref>`

バックエンドから値を取得します。`cli-store-mode` タグを参照して自動デコードします。

```bash
bundr get ps:/myapp/prod/DB_HOST
bundr get sm:myapp/prod/secret
```

### `bundr export --from <ref> --format shell|dotenv|direnv`

prefix 配下のパラメータを環境変数形式でエクスポートします。

```bash
bundr export --from ps:/myapp/prod/ --format shell    # export KEY=value
bundr export --from ps:/myapp/prod/ --format dotenv   # KEY=value
bundr export --from ps:/myapp/prod/ --format direnv   # export KEY=value
```

### `bundr jsonize <target-ref> --frompath <prefix>`

prefix 配下のパラメータから nested JSON を構築して target-ref に保存します。

```bash
bundr jsonize ps:/myapp/prod/config --frompath ps:/myapp/prod/
```

### `bundr cache refresh`

補完キャッシュ（`~/.cache/bundr/`）を手動更新します。

```bash
bundr cache refresh
```

### `bundr install-completions --shell bash|zsh`

シェル補完スクリプトをインストールします。

```bash
bundr install-completions --shell zsh
bundr install-completions --shell bash
```

## 設定

設定の優先順位や設定ファイルの詳細は [docs/configuration.md](docs/configuration.md) を参照してください。
