# Bundr — 開発コマンド

## ビルド
```bash
go build -o bundr .
```

## テスト
```bash
# 全パッケージテスト
go test ./...

# キャッシュなし（確実に再実行）
go test -count=1 ./...

# 特定パッケージ
go test ./internal/backend/...
go test ./cmd/...

# verboseモード
go test -v ./...

# カバレッジ計測
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 依存管理
```bash
go mod tidy
go mod download
```

## リント
```bash
golangci-lint run
```

## 実行（ビルド後）
```bash
./bundr put ps:/app/prod/DB_HOST --value localhost --store raw
./bundr get ps:/app/prod/DB_HOST
./bundr export --from ps:/app/prod/ --format shell
```

## Git
```bash
git status
git log --oneline -10
git diff
git add <file>
git commit -m "feat: ..."
git push
```

## ファイル検索 (Darwin)
```bash
find . -name "*.go" -not -path "*/vendor/*"
grep -r "pattern" --include="*.go" .
ls -la
```
