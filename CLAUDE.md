# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Bundr is a single-binary Go CLI that unifies AWS Parameter Store (Standard/Advanced) and Secrets Manager into a single interface. It uses tags as portable metadata and supports JSON/raw storage modes.

Go module: `github.com/youyo/bundr`

## Development Commands

```bash
# Initialize (first time)
go mod init github.com/youyo/bundr
go mod tidy

# Build
go build -o bundr ./...

# Test all packages
go test ./...

# Test single package
go test ./internal/backend/...

# Test with verbose output
go test -v ./...

# Test with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint (requires golangci-lint)
golangci-lint run
```

## Architecture

### Directory Structure

```
bundr/
├── main.go
├── main_test.go
├── cmd/
│   ├── root.go          # Kong CLI root (context, config init, BackendFactory)
│   ├── put.go           # bundr put <ref> --value [--secure]
│   ├── get.go           # bundr get <ref> [--raw|--json|--describe]
│   ├── ls.go            # bundr ls <prefix> [--recursive] [--describe]
│   ├── sync.go          # bundr sync -f <source> -t <dest> [--raw]
│   ├── exec.go          # bundr exec --from <prefix>... -- <cmd>
│   ├── vars.go          # buildVars() helper (shared by exec and sync)
│   ├── completion.go    # bundr completion bash|zsh|fish
│   ├── cache.go         # bundr cache refresh|clear
│   ├── predictor.go     # Tab completion predictors (cache + live)
│   └── json_util.go     # JSON output helper
└── internal/
    ├── backend/
    │   ├── interface.go  # Backend interface (Put/Get/GetByPrefix/Describe)
    │   ├── ref.go        # Ref parsing + BackendType/ValueType constants
    │   ├── ps.go         # SSM Parameter Store (ps:, psa:)
    │   ├── sm.go         # Secrets Manager (sm:)
    │   └── mock.go       # Test mock (no AWS calls)
    ├── cache/
    │   ├── cache.go      # FileStore: XDG-compliant, AWS identity-scoped, atomic writes
    │   └── lock.go       # File locking (syscall.Flock, Unix-only)
    ├── flatten/
    │   └── flatten.go    # JSON key flattening engine
    ├── jsonize/
    │   └── jsonize.go    # Parameter → nested JSON conversion
    ├── tags/
    │   └── tags.go       # Tag constants (cli=bundr, cli-store-mode, cli-schema)
    ├── dotenv/
    │   └── dotenv.go     # .env file read/write
    └── config/
        └── config.go     # Config hierarchy (TOML + env vars + CLI flags)
```

### Key Dependencies

- **CLI**: `github.com/alecthomas/kong` — declarative CLI, native completion support
- **Config**: `github.com/spf13/viper` — TOML + env var binding
- **AWS**: `github.com/aws/aws-sdk-go-v2` (ssm + secretsmanager packages)

### Backend Interface

All AWS interaction goes through the `Backend` interface in `internal/backend/interface.go`. This enables TDD — tests use `mock.go`, production uses `ps.go`/`sm.go`.

```go
type Backend interface {
    Put(ctx context.Context, ref string, opts PutOptions) error
    Get(ctx context.Context, ref string, opts GetOptions) (string, error)
    GetByPrefix(ctx context.Context, prefix string, opts GetByPrefixOptions) ([]ParameterEntry, error)
    Describe(ctx context.Context, ref string) (map[string]any, error)
}
```

### Ref Syntax

- `ps:/path/to/key` — SSM Parameter Store (use `--tier advanced` for Advanced)
- `sm:secret-id` — Secrets Manager

### Tag Schema (Required on All Managed Keys)

```
cli=bundr
cli-store-mode=raw|json
cli-schema=v1
```

### Storage Modes

- **raw**: value stored as-is
- **json**: scalars are JSON-encoded (`"hello"` → `"\"hello\""`)
- `get` reads `cli-store-mode` tag to decide decode behavior automatically

### Config Priority

CLI flags > env vars > `.bundr.toml` > `~/.config/bundr/config.toml`

## TDD Approach

Follow Red → Green → Refactor strictly:
1. Write failing test first (`internal/backend/mock.go` for AWS isolation)
2. Implement minimal code to pass
3. Refactor with tests green

Never call real AWS in unit tests. All AWS SDK interfaces must be mockable.

## Milestones

| M | Scope | Status |
|---|-------|--------|
| M1 | `put` / `get` commands + project scaffold | Done |
| M2 | `export` + flatten engine | Done |
| M3 | `jsonize` command | Done |
| M4 | Tab completion + cache system (SWR, `~/.cache/bundr/`) | Done |
| M5 | Config hierarchy + goreleaser CI/CD + Homebrew | Done |
| M6 | `exec`, `ls`, `completion` commands; `export` positional arg | Done |
| M7 | `exec` rename, `--describe` flag, GitHub Actions support | Done |
| M8 | `sync` コマンド追加・`export`/`jsonize`/`put --store` 廃止 | Done |
