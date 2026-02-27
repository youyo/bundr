# bundr 設定リファレンス

## 設定の優先順位

高い優先度から低い優先度の順:

1. **CLI フラグ** (`--region`, `--profile`, `--kms-key-id`)
2. **環境変数** (`BUNDR_AWS_REGION`, `BUNDR_AWS_PROFILE`, `BUNDR_AWS_KMS_KEY_ID`)
3. **プロジェクト設定ファイル** (`.bundr.toml` — カレントディレクトリ)
4. **グローバル設定ファイル** (`~/.config/bundr/config.toml`)

## 設定ファイル形式（TOML）

### プロジェクト設定: `.bundr.toml`

プロジェクトルートに `.bundr.toml` を配置します:

```toml
[aws]
region = "ap-northeast-1"
profile = "my-aws-profile"
kms_key_id = "arn:aws:kms:ap-northeast-1:123456789012:key/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

### グローバル設定: `~/.config/bundr/config.toml`

全プロジェクト共通のデフォルト設定:

```toml
[aws]
region = "ap-northeast-1"
profile = "default"
```

## 環境変数

| 環境変数 | 対応する設定 | 説明 |
|----------|-------------|------|
| `BUNDR_AWS_REGION` | `aws.region` | AWS リージョン (例: `ap-northeast-1`) |
| `BUNDR_AWS_PROFILE` | `aws.profile` | AWS 名前付きプロファイル (例: `prod`) |
| `BUNDR_AWS_KMS_KEY_ID` | `aws.kms_key_id` | KMS キー ID または ARN |

```bash
export BUNDR_AWS_REGION=ap-northeast-1
export BUNDR_AWS_PROFILE=prod
export BUNDR_AWS_KMS_KEY_ID=arn:aws:kms:ap-northeast-1:123456789012:key/xxxxxxxx
```

## CLI フラグ

全コマンドで使用可能なグローバルフラグ（最高優先度）:

| フラグ | 対応する環境変数 | 説明 |
|--------|----------------|------|
| `--region <value>` | `BUNDR_AWS_REGION` | AWS リージョンを指定 |
| `--profile <value>` | `BUNDR_AWS_PROFILE` | AWS プロファイルを指定 |
| `--kms-key-id <value>` | `BUNDR_AWS_KMS_KEY_ID` | KMS キー ID または ARN を指定 |

CLI フラグは環境変数や設定ファイルの値を上書きします:

```bash
# 設定ファイルに us-east-1 が設定されていても ap-northeast-1 が使われる
bundr --region ap-northeast-1 get ps:/myapp/DB_HOST

# 複数フラグを組み合わせ
bundr --region ap-northeast-1 --profile prod --kms-key-id alias/mykey put ps:/myapp/secret --value "val" --store raw
```

## 設定の例

### 開発環境と本番環境の切り替え

```bash
# 開発環境（デフォルト設定を使用）
bundr get ps:/myapp/dev/DB_HOST

# 本番環境（CLIフラグで上書き）
bundr --profile prod --region ap-northeast-1 get ps:/myapp/prod/DB_HOST
```

### 環境変数で設定を固定

```bash
# .envrc (direnv) に記述
export BUNDR_AWS_REGION=ap-northeast-1
export BUNDR_AWS_PROFILE=myteam-dev
```
