# Bundr - Comprehensive Technical Specification

Version: 1.1 Generated: 2026-02-27T03:21:21.521660Z
Updated: 2026-03-04 (v0.7.0)

------------------------------------------------------------------------

# 1. Purpose

Bundr is a single-binary Go CLI that abstracts AWS Parameter Store
(Standard/Advanced) and AWS Secrets Manager into a unified configuration
interface.

It enables:

-   Raw and JSON storage modes
-   Tag-driven metadata portability
-   Bidirectional sync between .env files, stdin/stdout, and AWS backends
-   Cache-backed, background-refresh tab completion
-   Deterministic behavior across environments

This document is intended for implementation by a coding agent.

------------------------------------------------------------------------

# 2. Core Design Principles

1.  JSON-first for application use
2.  Raw mode for infrastructure compatibility (CloudFormation)
3.  Tags as portable source of truth
4.  No local persistence of secret values
5.  Deterministic flatten behavior
6.  Single static Go binary
7.  Zero runtime daemon dependency
8.  Stale-While-Revalidate cache strategy

------------------------------------------------------------------------

# 3. Backends

Supported backends:

  Type   Meaning
  ------ ----------------------------------------
  ps     SSM Parameter Store (Standard/Advanced)
  sm     AWS Secrets Manager

Reference syntax:

-   `ps:/path/to/key` — Standard tier (default)
-   `ps:/path/to/key` with `--tier advanced` — Advanced tier
-   `sm:secret-id-or-name`

------------------------------------------------------------------------

# 4. Storage Modes

Each key must support:

  Mode   Description
  ------ ------------------------------
  raw    Plain string stored directly
  json   Value stored as valid JSON

## Behavior

-   json mode MUST JSON-encode scalars.
-   raw mode MUST store literal string unchanged.
-   get default behavior MUST inspect tag `cli-store-mode`.

------------------------------------------------------------------------

# 5. AWS Tag Schema

All managed keys MUST include:

cli=bundr cli-store-mode=raw|json cli-schema=v1

Optional: cli-flatten=on|off cli-owner=`<user>`

Tags are authoritative for behavior reconstruction.

------------------------------------------------------------------------

# 6. Commands

## 6.1 put

bundr put `<ref>` --value `<string>` --value-type string|secure --kms-key-id `<id>` --tags key=value

Rules:
- Value is always stored in raw mode.
- SecureString supported for ps:.
- Secrets Manager always encrypted.

------------------------------------------------------------------------

## 6.2 get

bundr get `<ref>`
bundr get `<ref>` --raw
bundr get `<ref>` --json
bundr get `<ref>/`

Default:
- If cli-store-mode=json → decode JSON.
- If raw → return literal string.
- Trailing `/` on ref → collect parameters under prefix and output as JSON.

------------------------------------------------------------------------

## 6.3 sync

bundr sync -f `<source>` -t `<destination>` [--raw]

Bidirectional sync between .env files, stdin/stdout, and AWS backends.

### Source/Destination types

| Type | Syntax | Example |
|------|--------|---------|
| File path | any path | `.env`, `/tmp/params.env` |
| Stdin/Stdout | `-` | `-` |
| PS single key | `ps:/path` | `ps:/app/prod/config` |
| PS prefix | `ps:/prefix/` (trailing `/`) | `ps:/app/prod/` |
| SM secret | `sm:id` | `sm:myapp/prod` |

### Read behavior (--from)

| Source type | Behavior |
|-------------|----------|
| File / stdin | Parse as .env format (`KEY=value`) |
| PS prefix (`ps:/prefix/`) | Fetch all parameters under prefix; key = uppercase relative path, `/` → `_` |
| PS/SM single ref | Fetch value; if JSON object → expand to entries; otherwise single entry with basename as key |

### Write behavior (--to)

| Destination type | Behavior |
|------------------|----------|
| File / stdout | Write .env format; JSON values are expanded unless `--raw` |
| PS prefix (`ps:/prefix/`) | Write each entry as individual parameter (key lowercased) with raw store mode |
| PS/SM single ref | Marshal all entries to JSON object and store with json store mode |

### --raw flag

Only effective when destination is file or stdout.
When set, JSON values are not expanded — output as-is.

------------------------------------------------------------------------

## 6.4 ls

bundr ls `<prefix>` [--recursive] [--describe]

List parameters under a prefix. Output is full ref format (e.g. `ps:/app/db_host`).

------------------------------------------------------------------------

## 6.5 exec

bundr exec --from `<prefix>`... -- `<command>` [args...]

Execute a command with environment variables populated from AWS parameters.
Multiple `--from` prefixes supported; later prefixes override earlier ones.

------------------------------------------------------------------------

# 7. Flatten Specification

## Objects

Nested objects flatten using delimiter `_`.

{"db": {"host": "x"}} → DB_HOST=x

## Arrays

Rules:

1.  If all elements are strings and NOT JSON → join
2.  If strings parse as JSON → parse and recurse
3.  If contains objects/arrays → index expansion
4.  Mixed types → index expansion

Example:

[ "a", "b" ] → A,B
[ {"host":"x"}, {"host":"y"} ] → SERVERS_0_HOST=x, SERVERS_1_HOST=y

------------------------------------------------------------------------

# 8. Configuration Hierarchy

Priority order:

1.  CLI flags
2.  Environment variables
3.  Project config (.bundr.toml)
4.  Global config (~/.config/bundr/config.toml)

Must support TOML.

------------------------------------------------------------------------

# 9. Cache System

Location: ~/.cache/bundr/

Contents:
- key metadata
- tags
- hierarchy index

NEVER store secret values.

TTL default: 30 seconds.

Completion flow:

1.  Read cache
2.  Return cached result immediately
3.  If TTL expired → spawn background refresh process
4.  Use file lock for concurrency control

------------------------------------------------------------------------

# 10. Completion

bundr completion bash|zsh|fish

Must support:
- prefix path completion
- secret name completion

Completion must never block on API call.

------------------------------------------------------------------------

# 11. Security Requirements

-   No plaintext secrets written to disk
-   stdin/file input for sensitive values
-   SecureString supported
-   KMS optional configuration

------------------------------------------------------------------------

# 12. Project Structure (Go)

cmd/ internal/ backend/ config/ cache/ flatten/ dotenv/ tags/

Dependencies:

-   AWS SDK v2
-   Kong (CLI parsing)
-   Viper (config)
-   Single static build

------------------------------------------------------------------------

# 13. Non-Goals

-   Not a secret rotation manager
-   Not a full config server
-   Not a daemonized service

------------------------------------------------------------------------

# 14. Future Extensions

-   Pluggable backends
-   AppConfig support
-   Remote cache provider
-   Policy validation

------------------------------------------------------------------------

# End of Comprehensive Specification
