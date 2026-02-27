# Bundr — コードスタイルと規約

## 言語・スタイル
- Go 標準スタイル（gofmt）
- golangci-lint でリント
- コメントは英語（実装者向けコード）、ユーザー向けドキュメントは日本語可

## 命名規則
- パッケージ名: 小文字 (backend, config, tags, flatten)
- インターフェース: 末尾に `er` を付けない場合も多い（`Backend`, `SSMClient`）
- テスト: `Test{関数名}_{シナリオ}` 形式
- モック: `Mock{型名}` または `mock{Type}` (小文字はパッケージ内限定)
- 定数: `Tag{Name}` (tags パッケージ), `StoreMode{Name}`

## TDD スタイル (Red → Green → Refactor)
1. 失敗するテストを先に書く
2. テストが通る最小限の実装
3. テストを変えずにリファクタリング
- AWS は必ず mock interface で隔離してテスト
- テーブル駆動テスト (TableDriven) を多用

## エラー処理
- エラーは `fmt.Errorf("context: %w", err)` でラッピング
- cmd 層: `"put command failed: %w"` など階層をプレフィックスで表現

## ファイル構成
- `*_test.go` は同じパッケージに配置
- インターフェースは `interface.go` に集約
- モックは `mock.go` に集約

## Go モジュール管理
- `go mod tidy` でクリーンに保つ
- ベンダリングなし (go.sum のみ管理)

## コミットメッセージ (Conventional Commits)
- `feat:` 新機能
- `fix:` バグ修正
- `test:` テスト追加
- `docs:` ドキュメント
- `refactor:` リファクタリング
- 日本語で記述
