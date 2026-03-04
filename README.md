# bundr

A CLI that unifies AWS Parameter Store and Secrets Manager.

[![Release](https://img.shields.io/github/v/release/youyo/bundr)](https://github.com/youyo/bundr/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/youyo/bundr)](https://goreportcard.com/report/github.com/youyo/bundr)

[日本語](README.ja.md)

## What it does

- Reads and writes to SSM Parameter Store (Standard and Advanced) and Secrets Manager through a single interface.
- Tags every managed parameter with `cli=bundr` for auditing and filtering.
- Syncs parameters between .env files, PS, SM, and stdio in any direction.
- Injects parameters into a subprocess environment without touching the shell.
- Caches parameter paths locally to speed up tab completion.

## Install

### GitHub Actions

```yaml
steps:
  - uses: youyo/bundr@v0.7
```

To pin to a specific version:

```yaml
steps:
  - uses: youyo/bundr@v0.7.0
    with:
      bundr-version: v0.7.0
```

### Homebrew (recommended)

```bash
brew install youyo/tap/bundr
```

### go install

```bash
go install github.com/youyo/bundr@latest
```

### Manual binary

Download from the [Releases page](https://github.com/youyo/bundr/releases):

```bash
# One-liner (Linux/macOS)
curl -sSfL https://raw.githubusercontent.com/youyo/bundr/main/scripts/install.sh | bash

# Or manually:
# macOS (Apple Silicon)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_darwin_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# macOS (Intel)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_darwin_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (x86_64)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_linux_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (ARM64)
curl -sSfL https://github.com/youyo/bundr/releases/latest/download/bundr_$(curl -sSf https://api.github.com/repos/youyo/bundr/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')_linux_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/
```

## Quick start

```bash
# 1. Store a value
bundr put ps:/myapp/db_host --value localhost

# 2. Get a value
bundr get ps:/myapp/db_host

# 3. List parameters under a prefix
bundr ls ps:/myapp/

# 4. Sync parameters to a .env file
bundr sync --from ps:/myapp/ --to .env

# 5. Run a command with parameters injected
bundr exec --from ps:/myapp/ -- node app.js

# Store a sensitive value as SecureString
bundr put ps:/myapp/api_key --value s3cr3t --secure
```

## Ref syntax

| Ref | Backend | Notes |
|-----|---------|-------|
| `ps:/path/to/key` | SSM Parameter Store | Standard tier by default; use `--tier advanced` for up to 8 KB |
| `parameterstore:/path/to/key` | SSM Parameter Store | Full-name alias for `ps:` |
| `sm:secret-id` | Secrets Manager | Versioned secrets |
| `secretsmanager:secret-id` | Secrets Manager | Full-name alias for `sm:` |

Both shorthand (`ps:`, `sm:`) and full-name (`parameterstore:`, `secretsmanager:`) prefixes are accepted in all commands.

## Recipes

### put

Store a value:

```bash
bundr put ps:/app/db_host --value localhost
```

Store to Secrets Manager:

```bash
bundr put sm:myapp/api-key --value s3cr3t
```

Encrypt with a specific KMS key (Advanced tier):

```bash
bundr put ps:/app/token --value s3cr3t --tier advanced --kms-key-id alias/my-key
```

Store a sensitive value as SSM SecureString:

```bash
bundr put ps:/app/api_key --value s3cr3t --secure
```

### get

Print a value:

```bash
bundr get ps:/app/db_host
```

Capture in a shell variable:

```bash
DB_HOST=$(bundr get ps:/app/db_host)
```

Print the raw stored value, ignoring the store-mode tag:

```bash
bundr get ps:/app/db_port --raw
```

Fetch all parameters under a prefix as JSON (use trailing `/`):

```bash
bundr get ps:/app/
# {"db_host":"localhost","db_port":"5432"}
```

### ls

List all parameter paths under a prefix:

```bash
bundr ls ps:/app/
bundr ls sm:myapp/          # Secrets Manager prefix
bundr ls sm:                 # all secrets
```

List recursively (include all nested paths):

```bash
bundr ls ps:/app/ --recursive
bundr ls sm:myapp/ --recursive
```

Count parameters:

```bash
bundr ls ps:/app/ | wc -l
```

### sync

Sync parameters between .env files, Parameter Store, Secrets Manager, and stdio:

```bash
# .env → PS (JSON bulk)
bundr sync --from .env --to ps:/app/config
# → ps:/app/config = {"DB_HOST":"localhost","DB_PORT":"5432"}

# .env → PS (flat expansion)
bundr sync --from .env --to ps:/app/
# → ps:/app/db_host = localhost
# → ps:/app/db_port = 5432

# .env → SM (JSON bulk)
bundr sync --from .env --to sm:myapp-prod

# PS (JSON value) → stdout (.env format with expansion)
bundr sync --from ps:/app/config --to -
# → DB_HOST=localhost

# PS prefix → stdout (.env format)
bundr sync --from ps:/app/ --to -

# Output raw value without expansion
bundr sync --from ps:/app/config --to - --raw
# → {"DB_HOST":"localhost"}

# Output in export format (suitable for eval)
bundr sync --from ps:/app/ --to - --format export
# → export DB_HOST=localhost
# → export DB_PORT=5432

# Load parameters into the current shell
eval $(bundr sync --from ps:/app/ --to - --format export)

# PS → SM (copy)
bundr sync --from ps:/app/config --to sm:backup

# stdin → PS
cat .env | bundr sync --from - --to ps:/app/config

# SM → .env file
bundr sync --from sm:prod --to .env
```

### exec

Runs a command with parameters injected as environment variables. The subprocess inherits the current environment plus the fetched parameters. Later `--from` entries take precedence over earlier ones.

Single prefix:

```bash
bundr exec --from ps:/app/ -- node server.js
```

Multiple prefixes — later entries override earlier ones:

```bash
bundr exec --from ps:/common/ --from ps:/app/prod/ -- python main.py
```

Inspect what gets injected:

```bash
bundr exec --from ps:/app/ -- env | grep DB
```

Use in a GitHub Actions workflow:

```yaml
- name: Run with AWS parameters
  run: bundr exec --from ps:/myapp/prod/ -- ./deploy.sh
  env:
    AWS_REGION: ap-northeast-1
    AWS_ROLE_ARN: arn:aws:iam::123456789012:role/MyRole
```

### completion

Print and immediately activate completion for the current shell session:

```bash
eval "$(bundr completion zsh)"
eval "$(bundr completion bash)"
bundr completion fish | source
```

To persist across sessions, add to your shell startup file:

```bash
# ~/.zshrc
eval "$(bundr completion zsh)"

# ~/.bashrc
eval "$(bundr completion bash)"

# ~/.config/fish/config.fish
bundr completion fish | source
```

Tab completion navigates the parameter hierarchy one level at a time:

```bash
bundr get ps:/<TAB>            # ps:/app/  ps:/config/
bundr get ps:/app/<TAB>        # ps:/app/prod/  ps:/app/stg/
bundr get ps:/app/prod/<TAB>   # ps:/app/prod/DB_HOST  ps:/app/prod/DB_PORT
```

### cache

bundr caches parameter paths locally to make tab completion fast. The cache refreshes in the background automatically during completion.

Refresh the cache manually after adding new parameters:

```bash
bundr cache refresh                  # refresh all backends
bundr cache refresh ps:/app/         # refresh a specific Parameter Store prefix
bundr cache refresh sm:              # refresh all Secrets Manager secrets
```

Clear the local cache completely:

```bash
bundr cache clear
```

Full cache reset workflow:

```bash
bundr cache clear && bundr cache refresh ps:/
```

## Command reference

### Global flags

| Flag | Env var | Description |
|------|---------|-------------|
| `--region` | `AWS_REGION`, `BUNDR_AWS_REGION` | AWS region |
| `--profile` | `AWS_PROFILE`, `BUNDR_AWS_PROFILE` | AWS profile name |
| `--kms-key-id` | `BUNDR_KMS_KEY_ID`, `BUNDR_AWS_KMS_KEY_ID` | KMS key ID or ARN |

### bundr put

```
bundr put <ref> --value <string> [flags]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--value` | Yes | Value to store |
| `--kms-key-id` | No | KMS key ID or ARN for encryption |
| `--secure` | No | Use SecureString type (SSM Parameter Store only) |

### bundr get

```
bundr get <ref> [--raw|--json|--describe] [flags]
```

| Flag | Description |
|------|-------------|
| `--raw` | Print the stored value without JSON decoding |
| `--json` | Print the JSON-encoded value |
| `--describe` | Print parameter metadata as JSON |

Use a trailing `/` to fetch all parameters under a prefix as JSON:

```
bundr get ps:/app/
```

### bundr sync

```
bundr sync -f <source> -t <dest> [--raw] [--format dotenv|export]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-f`, `--from` | | Source (file path, `-`, `ps:/path`, `ps:/prefix/`, `sm:id`) |
| `-t`, `--to` | | Destination (file path, `-`, `ps:/path`, `ps:/prefix/`, `sm:id`) |
| `--raw` | false | Output raw value without expanding JSON (file/stdout only) |
| `--format` | `dotenv` | Output format for file/stdout: `dotenv` (`KEY=VALUE`) or `export` (`export KEY=VALUE`) |

`--to` trailing `/` controls storage mode:

| Destination | Behavior |
|-------------|----------|
| `ps:/path` | JSON bulk save |
| `ps:/prefix/` | Flat expansion (keys lowercased, each key as individual parameter) |
| `sm:id` | JSON bulk save |

### bundr ls

```
bundr ls <prefix> [--recursive]
```

Outputs one ref per line (e.g. `ps:/app/db_host`).

### bundr exec

```
bundr exec [--from <prefix>]... [flags] -- <command> [args...]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-f`, `--from` | | Source prefix; may be repeated; later entries win |
| `--no-flatten` | false | Disable JSON key flattening |
| `--upper` | true | Uppercase variable names |
| `--flatten-delim` | `_` | Delimiter for flattened keys |
| `--array-mode` | `join` | `join`, `index`, or `json` |
| `--array-join-delim` | `,` | Delimiter for `join` mode |

### bundr completion

```
bundr completion bash|zsh|fish
```

### bundr cache

```
bundr cache refresh [prefix]
bundr cache clear
```

## Advanced topics

### Tab completion

Enable tab completion to navigate parameter hierarchies interactively:

```bash
eval "$(bundr completion zsh)"   # add to ~/.zshrc
eval "$(bundr completion bash)"  # add to ~/.bashrc
```

The completion engine caches parameter paths locally. Run `bundr cache refresh` to pre-populate the cache before first use:

```bash
bundr cache refresh ps:/          # cache all Parameter Store paths
bundr cache refresh sm:           # cache all Secrets Manager paths
```

**Known limitations:**
- Tab completion requires the `bundr` binary to be in `$PATH` under the name `bundr`. A local build (e.g. `./bundr`) will not trigger registered completion functions.
- When using short-lived credentials (aws-vault, AWS SSO), the credential may expire before the background cache process runs. Pre-populate the cache while credentials are active with `bundr cache refresh`.

## Configuration

### Priority order

Settings are applied in this order (later sources override earlier ones):

1. `~/.config/bundr/config.toml` — global defaults
2. `.bundr.toml` — project-level settings (in current directory)
3. `AWS_REGION`, `AWS_PROFILE` — standard AWS environment variables
4. `BUNDR_AWS_REGION`, `BUNDR_AWS_PROFILE` — bundr-specific env vars (override `AWS_*`)
5. `--region`, `--profile`, `--kms-key-id` CLI flags — highest priority

### config.toml

`~/.config/bundr/config.toml` and `.bundr.toml` use the same format:

```toml
[aws]
region = "ap-northeast-1"
profile = "my-profile"
kms_key_id = "alias/my-key"
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `AWS_REGION` | AWS region (standard) |
| `AWS_PROFILE` | AWS profile name (standard) |
| `BUNDR_AWS_REGION` | AWS region (overrides `AWS_REGION`) |
| `BUNDR_AWS_PROFILE` | AWS profile name (overrides `AWS_PROFILE`) |
| `BUNDR_KMS_KEY_ID` | KMS key ID or ARN |
| `BUNDR_AWS_KMS_KEY_ID` | Alias for `BUNDR_KMS_KEY_ID` |

## AWS authentication

bundr uses the standard AWS SDK v2 credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. AWS profiles (`~/.aws/credentials`, `~/.aws/config`)
3. IAM instance roles (EC2, ECS, Lambda, etc.)

Override the profile for a single command:

```bash
bundr ls ps:/app/ --profile my-profile
```

Override via environment variable:

```bash
AWS_PROFILE=my-profile bundr ls ps:/app/
```

Set a default in the project config:

```toml
# .bundr.toml
[aws]
profile = "my-profile"
```

## Tag schema

bundr tags every managed parameter:

| Tag | Value | Purpose |
|-----|-------|---------|
| `cli` | `bundr` | Identifies bundr-managed resources |
| `cli-store-mode` | `raw` or `json` | Controls decoding on `get` |
| `cli-schema` | `v1` | Schema version |

## License

MIT

---

[日本語](README.ja.md)
