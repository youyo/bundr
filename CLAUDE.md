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
├── cmd/
│   ├── root.go          # Kong CLI root (context, config init)
│   ├── put.go           # bundr put <ref> --value --store
│   └── get.go           # bundr get <ref> [--raw|--json]
└── internal/
    ├── backend/
    │   ├── interface.go  # Backend interface (Put/Get)
    │   ├── ps.go         # SSM Parameter Store (ps:, psa:)
    │   ├── sm.go         # Secrets Manager (sm:)
    │   └── mock.go       # Test mock (no AWS calls)
    ├── tags/
    │   └── tags.go       # Tag constants (cli=bundr, cli-store-mode, cli-schema)
    └── config/
        └── config.go     # Viper-based config (TOML + env vars)
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
}
```

### Ref Syntax

- `ps:/path/to/key` — SSM Parameter Store Standard
- `psa:/path/to/key` — SSM Parameter Store Advanced
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

| M | Scope |
|---|-------|
| M1 | `put` / `get` commands + project scaffold (current) |
| M2 | `export` + flatten engine |
| M3 | `jsonize` command |
| M4 | `__complete` + cache system (SWR, `~/.cache/bundr/`) |
| M5 | Config hierarchy + goreleaser CI/CD |

See `plans/bundr-m01-scaffold-core-commands.md` for M1 step-by-step plan.
