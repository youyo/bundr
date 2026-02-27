# Roadmap: Bundr

## Meta
| 項目 | 値 |
|------|---|
| ゴール | AWS Parameter Store / Secrets Manager を統合する単一バイナリ Go CLI |
| 成功基準 | `bundr get`, `bundr put`, `bundr export` が実 AWS 環境で動作。単体テスト 80%+ coverage |
| 制約 | Go 単一バイナリ、AWS SDK v2 必須、Kong (CLI)、Viper (config) |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/bundr |
| 作成日 | 2026-02-27 |
| 最終更新 | 2026-02-27 13:10 |
| ステータス | 進行中 |

## Current Focus
- **マイルストーン**: M2 - export + flatten ロジック
- **直接の完了**: M1 実装完了 (43テスト全PASS、カバレッジ80%+)
- **次のアクション**: `/plan` で M2 詳細計画を作成

## Progress

### M1: プロジェクト骨格 + put/get コア実装 ✅ 完了
- [x] go.mod 初期化 (module: github.com/youyo/bundr)
- [x] ディレクトリ構造作成 (cmd/, internal/backend/, internal/config/, internal/tags/)
- [x] Backend インターフェース定義 + mock 実装
- [x] SSM Parameter Store バックエンド実装 (ps, psa)
- [x] Secrets Manager バックエンド実装 (sm)
- [x] `bundr put` コマンド実装 (raw/json mode, tag schema)
- [x] `bundr get` コマンド実装 (cli-store-mode tag 参照)
- [x] 単体テスト (TDD: mock backend で AWS 隔離) — 43テスト全PASS、カバレッジ80%+
- 📄 詳細: plans/bundr-m01-scaffold-core-commands.md
- 🌿 ブランチ: feat-m1-scaffold-core-commands (プッシュ済み)

### M2: export + flatten ロジック
- [ ] flatten エンジン実装 (objects: _ 区切り, arrays: join/index)
- [ ] `bundr export` コマンド (shell/dotenv/direnv format)
- [ ] flatten 単体テスト (エッジケース網羅)
- 📄 詳細: (M2 着手時に生成)

### M3: jsonize コマンド
- [ ] prefix 配下パラメータ収集ロジック
- [ ] パスベース nested JSON 構築
- [ ] `bundr jsonize` コマンド実装
- [ ] 単体テスト
- 📄 詳細: (M3 着手時に生成)

### M4: completion + cache システム
- [ ] キャッシュシステム (~/.cache/bundr/, SWR 戦略, ファイルロック)
- [ ] `bundr __complete` コマンド (prefix補完, secret名補完)
- [ ] シェル統合 (bash/zsh)
- 📄 詳細: (M4 着手時に生成)

### M5: 設定階層 + CI/CD + リリース
- [ ] 設定階層実装 (CLI flags > env vars > .bundr.toml > ~/.config/bundr/config.toml)
- [ ] GitHub Actions CI/CD
- [ ] goreleaser 設定 (静的バイナリ)
- [ ] ドキュメント整備
- 📄 詳細: (M5 着手時に生成)

## Blockers
なし

## Architecture Decisions
| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | CLI パーサー: Kong | spf13/cobra より宣言的、自動補完サポート | 2026-02-27 |
| 2 | 設定: Viper | TOML サポート + env var バインディング | 2026-02-27 |
| 3 | AWS Backend: インターフェース隔離 | TDD で AWS をモック可能にする | 2026-02-27 |

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-02-27 12:30 | 作成 | ロードマップ初版作成 (スペック v1.1 ベース) |
| 2026-02-27 13:10 | 完了 | M1 実装完了 (43テスト全PASS、カバレッジ80%+、ブランチ: feat-m1-scaffold-core-commands) |
