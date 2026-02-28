# bundr

A CLI for AWS Parameter Store and Secrets Manager.

[![Release](https://img.shields.io/github/v/release/youyo/bundr)](https://github.com/youyo/bundr/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/youyo/bundr)](https://goreportcard.com/report/github.com/youyo/bundr)

[日本語](README.ja.md)

## What it does

- Reads and writes to SSM Parameter Store (Standard and Advanced) and Secrets Manager through a single interface.
- Tags every managed parameter with `cli=bundr` for auditing and filtering.
- Exports parameters as environment variables (`shell`, `dotenv`, `direnv`).
- Injects parameters into a subprocess environment without touching the shell.
- Caches parameter paths locally to speed up tab completion.

## Install

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
# macOS (Apple Silicon)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Darwin_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Darwin_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Linux_amd64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/

# Linux (ARM64)
curl -L https://github.com/youyo/bundr/releases/latest/download/bundr_Linux_arm64.tar.gz | tar xz
sudo mv bundr /usr/local/bin/
```

## Quick start

```bash
# 1. Store a value
bundr put ps:/myapp/db_host --value localhost --store raw

# 2. Get a value
bundr get ps:/myapp/db_host

# 3. List parameters under a prefix
bundr ls ps:/myapp/

# 4. Export as environment variables
eval "$(bundr export ps:/myapp/ --format shell)"

# 5. Run a command with parameters injected
bundr exec --from ps:/myapp/ -- node app.js
```

## Ref syntax

| Ref | Backend | Notes |
|-----|---------|-------|
| `ps:/path/to/key` | SSM Parameter Store (Standard) | Up to 4 KB |
| `psa:/path/to/key` | SSM Parameter Store (Advanced) | Up to 8 KB |
| `sm:secret-id` | Secrets Manager | Versioned secrets |

## Recipes

### put

Store a string in raw mode:

```bash
bundr put ps:/app/db_host --value localhost --store raw
```

Store a value that should be treated as a JSON scalar:

```bash
bundr put ps:/app/db_port --value 5432 --store json
bundr put ps:/app/debug --value true --store json
```

Store to Secrets Manager:

```bash
bundr put sm:myapp/api-key --value s3cr3t --store raw
```

Encrypt with a specific KMS key:

```bash
bundr put psa:/app/token --value s3cr3t --store raw --kms-key-id alias/my-key
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

### ls

List all parameter paths under a prefix:

```bash
bundr ls ps:/app/
```

List only the immediate children (no recursion):

```bash
bundr ls ps:/app/ --no-recursive
```

Count parameters:

```bash
bundr ls ps:/app/ | wc -l
```

### export

Load parameters into the current shell:

```bash
eval "$(bundr export ps:/app/ --format shell)"
```

Write a `.env` file:

```bash
bundr export ps:/app/ --format dotenv > .env
```

Write a direnv `.envrc`:

```bash
bundr export ps:/app/ --format direnv > .envrc
```

Use in a CI/CD pipeline (GitHub Actions):

```yaml
- name: Load parameters
  run: eval "$(bundr export ps:/myapp/prod/ --format shell)"
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

### jsonize

Reads all parameters under a prefix and stores them as a single JSON object in Secrets Manager:

```bash
bundr jsonize sm:myapp-config --frompath ps:/app/
```

Use `--force` to overwrite an existing secret:

```bash
bundr jsonize sm:myapp-config --frompath ps:/app/ --force
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

### cache

bundr caches parameter paths locally to make tab completion fast. The cache refreshes in the background automatically during completion.

Refresh the cache manually after adding new parameters:

```bash
bundr cache refresh
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
bundr put <ref> --value <string> --store raw|json [flags]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--value` | Yes | Value to store |
| `--store` | Yes | `raw` stores as-is; `json` encodes scalars as JSON |
| `--kms-key-id` | No | KMS key ID or ARN for encryption |

### bundr get

```
bundr get <ref> [--raw|--json] [flags]
```

| Flag | Description |
|------|-------------|
| `--raw` | Print the stored value without JSON decoding |
| `--json` | Print the JSON-encoded value |

### bundr export

```
bundr export <prefix> --format shell|dotenv|direnv [flags]
```

| Format | Output |
|--------|--------|
| `shell` | `export KEY=value` |
| `dotenv` | `KEY=value` |
| `direnv` | `export KEY=value` |

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | | Output format (required) |
| `--no-recursive` | false | List only immediate children |
| `--upper` | true | Uppercase variable names |
| `--flatten-delim` | `_` | Delimiter for flattened keys |

### bundr ls

```
bundr ls <prefix> [--no-recursive]
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

### bundr jsonize

```
bundr jsonize <target-ref> --frompath <prefix> [--force]
```

| Flag | Description |
|------|-------------|
| `--frompath` | SSM prefix to read from |
| `--force` | Overwrite if the target already exists |

### bundr completion

```
bundr completion bash|zsh|fish
```

### bundr cache

```
bundr cache refresh
```

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
| `cli-store-mode` | `raw` or `json` | Controls decoding on `get` and `export` |
| `cli-schema` | `v1` | Schema version |

## License

MIT

---

[日本語](README.ja.md)
