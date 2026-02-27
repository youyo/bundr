# Bundr — タスク完了チェックリスト

## コード変更後に必ず実行

1. **テスト実行**
   ```bash
   go test -count=1 ./...
   ```
   全パッケージが PASS であること

2. **ビルド確認**
   ```bash
   go build -o bundr .
   ```
   エラーなしでビルドできること

3. **カバレッジ確認**（任意、PR前推奨）
   ```bash
   go test -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out | grep total
   ```
   80%+ を維持すること

4. **リント**（golangci-lint がある場合）
   ```bash
   golangci-lint run
   ```

5. **依存整合**
   ```bash
   go mod tidy
   ```

## M2 実装時の追加チェック
- `GetByPrefix` のページネーション（NextToken）が正しく動作するか
- `ParseRef()` → `ref.Path` を `GetByPrefix` に渡しているか（ref 文字列直渡し禁止）
- StoreMode="json" の場合のみ JSON デコードしてから flatten に渡しているか
- `eval "$(bundr export ...)"` で安全なエスケープができているか
- 出力の決定論的な順序保証（sort.Strings）があるか
