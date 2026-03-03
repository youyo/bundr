# HANDOVER.md

> 生成日時: 2026-03-03 14:06
> プロジェクト: bundr
> ブランチ: main

## 今回やったこと

- **`exec --env-file` 機能のプランニング（実装は見送り）**
  - 1Password CLI の `op run --env-file .env` のように、`.env` ファイル内の bundr ref を解決して `exec` コマンドで環境変数注入する機能を設計した
  - プランファイル: `plans/magical-sleeping-beacon.md` に設計を保存
  - ユーザーが「やっぱり実装しません」と判断し、プランのみ残した

## 決定事項（プランのみ・将来の実装時の参考）

- **コマンド形式**: `exec --env-file` フラグを既存 `exec` コマンドに追加
  - `bundr exec --env-file .env.bundr -- node server.js`
  - 複数指定可能（後者優先）
  - `--from` との併用可
- **通常値の扱い**: bundr ref でない値（`PORT=8080` 等）はそのまま環境変数にセット
- **SM ref の非対称性**: `--from sm:` は不可だが、`--env-file` 内の `KEY=sm:secret` は可能（個別 `Get()` 呼び出しのため）
- **マージ優先順**: `os.Environ()` < `--from` 展開 < `--env-file` 解決
- **インラインコメント非対応**: `KEY=VALUE # comment` の `# comment` は値として扱う
- **実装ファイル**（将来実装時の参考）:
  - `cmd/envfile.go` — `parseEnvFile`, `isBundrRef`, `resolveEnvFile`（新規作成）
  - `cmd/envfile_test.go` — 単体テスト（新規作成）
  - `cmd/exec.go` — `EnvFile []string` フラグ追加、`Run()` 修正
  - `cmd/exec_test.go` — `TestExecCmd_EnvFile` 追加

## 捨てた選択肢と理由

- **新規コマンド `inject` / `envsubst`**: 既存の `exec` との分断になる。`exec --env-file` の方がユーザビリティが高い
- **ファイルを位置引数で受け取る方式**: `exec` の `Args` と混在して紛らわしい
- **インラインコメント対応**: パスワード等の先頭/末尾空白が誤ってトリムされるリスクを避けるため非対応

## ハマりどころ

- 特になし（プランのみで実装未実施）

## 学び

- **`--from sm:` が禁止な理由**: `GetByPrefix` は SM が複数キー取得の概念を持たないため不可。一方 `--env-file` は個別 `Get()` を呼ぶので SM も自然に動作する
- **`exec_test.go` の `setupExecCmd`**: `opts ...func(*ExecCmd)` パターンを使っているため、`EnvFile` フィールド追加時の変更が最小限で済む

## 次にやること（優先度順）

- [ ] `exec --env-file` の実装（プランは `plans/magical-sleeping-beacon.md` 参照）
- [ ] `bundr exec --from sm:prefix` のサポート検討（低）
- [ ] 旧キャッシュ形式マイグレーションメッセージの検討（低）
- [ ] README に CI バッジ追加検討（低）
- [ ] `--describe` 出力に `--format table` オプション追加検討（低）

## 関連ファイル

- `plans/magical-sleeping-beacon.md` — `exec --env-file` 機能の完全な実装プラン（TDD 設計・テストケース一覧含む）
- `cmd/exec.go` — 将来の変更予定先（`ExecCmd` struct、`Run()` メソッド）
- `cmd/exec_test.go` — 将来のテスト追加予定先
- `internal/backend/interface.go` — `Backend.Get()` の呼び出しパターン参照元
