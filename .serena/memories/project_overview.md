# Bundr — プロジェクト概要

## 目的
AWS Parameter Store (Standard/Advanced) と Secrets Manager を単一インターフェースで統一する Go CLI ツール。
タグをポータブルなメタデータとして使用し、JSON/raw 保存モードをサポートする。

## モジュール
- Go module: `github.com/youyo/bundr`
- Go バージョン: 1.25.7

## 主要コマンド
- `bundr put <ref> --value <string> --store raw|json` — パラメータ/シークレット保存
- `bundr get <ref> [--raw|--json]` — パラメータ/シークレット取得
- `bundr export --from <prefix> --format shell|dotenv|direnv` — prefix 配下を環境変数形式でエクスポート (M2)
- `bundr jsonize <target-ref> --frompath <prefix>` — prefix 配下を JSON 化して保存 (M3)
- `bundr __complete <shell>` — シェル補完 (M4)

## Ref 構文
- `ps:/path/to/key` — SSM Parameter Store Standard
- `psa:/path/to/key` — SSM Parameter Store Advanced
- `sm:secret-id-or-name` — Secrets Manager

## タグスキーマ (必須)
```
cli=bundr
cli-store-mode=raw|json
cli-schema=v1
```

## マイルストーン
| M | 内容 | 状態 |
|---|------|------|
| M1 | プロジェクト骨格 + put/get 実装 | ✅ 完了 |
| M2 | export + flatten ロジック | 計画済み (plans/bundr-m02-export-flatten.md) |
| M3 | jsonize コマンド | 未着手 |
| M4 | completion + cache システム | 未着手 |
| M5 | 設定階層 + CI/CD + goreleaser | 未着手 |
