# Bundr - Comprehensive Technical Specification

Version: 1.1 Generated: 2026-02-27T03:21:21.521660Z

------------------------------------------------------------------------

# 1. Purpose

Bundr is a single-binary Go CLI that abstracts AWS Parameter Store
(Standard/Advanced) and AWS Secrets Manager into a unified configuration
interface.

It enables:

-   Raw and JSON storage modes
-   Tag-driven metadata portability
-   Multi-parameter bundling into structured JSON
-   Flattened environment export
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
  ------ ------------------------------
  ps     SSM Parameter Store Standard
  psa    SSM Parameter Store Advanced
  sm     AWS Secrets Manager

Reference syntax:

-   ps:/path/to/key
-   psa:/path/to/key
-   sm:secret-id-or-name

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

cli=bundr cli-store-mode=raw\|json cli-schema=v1

Optional: cli-flatten=on\|off cli-owner=`<user>`{=html}

Tags are authoritative for behavior reconstruction.

------------------------------------------------------------------------

# 6. Commands

## 6.1 put

bundr put `<ref>`{=html} --value `<string>`{=html} --store raw\|json
--value-type string\|secure --kms-key-id `<id>`{=html} --tags key=value

Rules: - If json mode: scalar must be JSON encoded. - SecureString
supported for ps/psa. - Secrets Manager always encrypted.

------------------------------------------------------------------------

## 6.2 get

bundr get `<ref>`{=html} bundr get `<ref>`{=html} --raw bundr get
`<ref>`{=html} --json

Default: - If cli-store-mode=json → decode JSON. - If raw → return
literal string.

------------------------------------------------------------------------

## 6.3 export

bundr export --from \<prefix\|ref\> --format shell\|dotenv\|direnv
--no-flatten (optional) --array-mode join\|index\|json
--array-join-delim "," --flatten-delim "\_" --upper (default true)

Default: - flatten enabled - uppercase keys

Output must be safe for:

eval "\$(bundr export --from ps:/app/prod/)"

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

\[ "a", "b" \] → A,B \[ {"host":"x"}, {"host":"y"} \] →
SERVERS_0\_HOST=x → SERVERS_1\_HOST=y

------------------------------------------------------------------------

# 8. jsonize Command

bundr jsonize `<target-ref>`{=html} --frompath ps:/prefix/ --store json
--value-type string\|secure --force

Behavior:

-   Fetch parameters under prefix
-   Construct nested JSON based on path
-   Store as single JSON value at target

Example:

/app/prod/DB_HOST=localhost /app/prod/DB_PORT=5432

→

{ "db": { "host": "localhost", "port": 5432 } }

------------------------------------------------------------------------

# 9. Configuration Hierarchy

Priority order:

1.  CLI flags
2.  Environment variables
3.  Project config (.bundr.toml)
4.  Global config (\~/.config/bundr/config.toml)

Must support TOML.

------------------------------------------------------------------------

# 10. Cache System

Location: \~/.cache/bundr/

Contents: - key metadata - tags - hierarchy index

NEVER store secret values.

TTL default: 30 seconds.

Completion flow:

1.  Read cache
2.  Return cached result immediately
3.  If TTL expired → spawn background refresh process
4.  Use file lock for concurrency control

------------------------------------------------------------------------

# 11. Completion

bundr \_\_complete `<shell>`{=html} `<args>`{=html}

Must support: - prefix path completion - secret name completion - JSON
key completion (optional, via tag metadata)

Completion must never block on API call.

------------------------------------------------------------------------

# 12. Security Requirements

-   No plaintext secrets written to disk
-   stdin/file input for sensitive values
-   SecureString supported
-   KMS optional configuration

------------------------------------------------------------------------

# 13. Project Structure (Go)

cmd/ internal/ backend/ aws/ config/ cache/ flatten/ completion/ tags/

Dependencies:

-   AWS SDK v2
-   Kong (CLI parsing)
-   Viper (config)
-   Single static build

------------------------------------------------------------------------

# 14. Non-Goals

-   Not a secret rotation manager
-   Not a full config server
-   Not a daemonized service

------------------------------------------------------------------------

# 15. Future Extensions

-   Pluggable backends
-   AppConfig support
-   Remote cache provider
-   Policy validation

------------------------------------------------------------------------

# End of Comprehensive Specification
