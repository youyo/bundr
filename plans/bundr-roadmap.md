# Roadmap: Bundr

## Meta
| 項目 | 値 |
|------|---|
| ゴール | AWS Parameter Store / Secrets Manager を統合する単一バイナリ Go CLI |
| 成功基準 | `bundr get`, `bundr put`, `bundr export` が実 AWS 環境で動作。単体テスト 80%+ coverage |
| 制約 | Go 単一バイナリ、AWS SDK v2 必須、Kong (CLI)、Viper (config) |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/bundr |
| 作成日 | 2026-02-27 |
| 最終更新 | 2026-03-04 |
| ステータス | 進行中 |

## Current Focus
- **最新リリース**: v0.7.0 (M8 完了 — sync コマンド追加、export/jsonize/put --store 廃止)
- **次のアクション**:
  - v0.7.1: lint 修正 (errcheck)
  - 将来: exec --env-file 実装の可能性

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

### M2: export + flatten ロジック ✅ 完了
- [x] flatten エンジン実装 (objects: _ 区切り, arrays: join/index)
- [x] `bundr export` コマンド (shell/dotenv/direnv format)
- [x] flatten 単体テスト (エッジケース網羅) — カバレッジ 80.8%
- 🌿 ブランチ: feat-m2-export-flatten (マージ済み)

### M3: jsonize コマンド ✅ 完了
- [x] prefix 配下パラメータ収集ロジック
- [x] パスベース nested JSON 構築
- [x] `bundr jsonize` コマンド実装
- [x] 単体テスト — カバレッジ 96%/87%/83%
- 🌿 ブランチ: feat-m3-jsonize (マージ済み)

### M4: completion + cache システム ✅ 完了
- [x] キャッシュシステム (~/.cache/bundr/, Always-Refresh-on-Read + 10秒スロットリング, syscall.Flock)
- [x] `bundr cache refresh` コマンド
- [x] `bundr install-completions` コマンド (bash/zsh)
- [x] kongplete v0.4.0 統合 + CachePredictor
- [x] BGLauncher インターフェース (ExecBGLauncher + MockBGLauncher)
- [x] COMP_* 環境変数フィルタリング
- [x] NoopStore フォールバック (non-fatal 初期化)
- [x] 全テスト PASS — cmd 86.9% / cache 87.3%
- 📄 詳細: plans/bundr-m04-completion-cache.md
- 🌿 ブランチ: feat-m4-completion-cache (PR #2)

### M5: 設定階層 + CI/CD + リリース ✅ 完了
- [x] 設定階層実装 (CLI flags > env vars > .bundr.toml > ~/.config/bundr/config.toml)
- [x] GitHub Actions CI/CD
- [x] goreleaser 設定 (静的バイナリ)
- [x] ドキュメント整備
- 📄 詳細: plans/bundr-m05-config-cicd-release.md

### M6: exec + ls + completion + export 位置引数化 ✅ 完了
- [x] `bundr export` 位置引数化 (`--from` 廃止)
- [x] `bundr ls` コマンド実装
- [x] `bundr completion` コマンド実装 (bash/zsh/fish)
- [x] `buildVars()` 抽出・共通化
- [x] `bundr run` コマンド実装
- 📄 詳細: plans/bundr-m06-run-command.md

### M7: exec リネーム + --describe + GitHub Actions 対応 ✅ 完了
- [x] `run` → `exec` リネーム
- [x] `--describe` フラグ実装 (get/ls)
- [x] AWS 環境変数優先順位整理
- [x] Homebrew 配布設定
- 📄 詳細: plans/bundr-m07-polish-ux.md

### M8: sync コマンド + Breaking Changes (v0.7.0) ✅ 完了
- [x] `bundr sync -f <from> -t <to> [--raw]` コマンド追加 (.env ↔ PS/SM 双方向同期)
- [x] `bundr export` コマンド廃止
- [x] `bundr jsonize` コマンド廃止
- [x] `bundr put --store` フラグ廃止 (常に raw 保存)

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
| 2026-02-27 | 完了 | M2 実装完了 (カバレッジ80.8%、ブランチ: feat-m2-export-flatten) |
| 2026-02-27 | 完了 | M3 実装完了 (カバレッジ96%/87%/83%、ブランチ: feat-m3-jsonize) |
| 2026-02-27 | 完了 | M4 実装完了 (cmd 86.9%/cache 87.3%、PR #2: feat-m4-completion-cache) |
| 2026-03-01 | 完了 | M5 実装完了 (設定階層 + CI/CD + goreleaser リリース) |
| 2026-03-01 | 完了 | M6 実装完了 (exec + ls + completion + export 位置引数化) |
| 2026-03-02 | 完了 | M7 実装完了 (exec リネーム + --describe + Homebrew 配布) |
| 2026-03-04 | 完了 | M8 実装完了 (v0.7.0: sync コマンド追加、export/jsonize/put --store 廃止) |
