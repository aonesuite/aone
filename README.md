# aone

`aone` is the AoneSuite command-line tool for managing sandboxes, templates,
volumes, and account credentials against the AoneSuite control plane.

## Install

```sh
go install github.com/aonesuite/aone@latest
```

Or build from source:

```sh
git clone https://github.com/aonesuite/aone
cd aone
go build -o aone .
```

## Quick start

```sh
# 1. Save credentials (writes ~/.config/aone/config.json with mode 0600)
aone auth login --api-key ak_xxx

# 2. Verify the active credential source
aone auth info

# 3. Scaffold a template project
aone sandbox template init --name my-app --language go --path ./my-app

# 4. Build the template (template_id is written back to aone.sandbox.toml)
cd my-app
aone sandbox template create my-app

# 5. Launch a sandbox from the project's template
aone sandbox create
```

## Authentication

Credentials are resolved with the following precedence:

```
flag --api-key   >  env AONE_API_KEY   >  ~/.config/aone/config.json
flag --endpoint  >  env AONE_SANDBOX_API_URL  >  config file  >  built-in default
```

| Subcommand | Purpose |
|---|---|
| `aone auth login --api-key <key>` | Save an API key locally. Pass `--no-verify` to skip the live check. |
| `aone auth info` | Print the active key (masked) and which layer (flag / env / config) won. |
| `aone auth logout` | Clear the saved API key while preserving any custom endpoint. |
| `aone auth configure` | Interactively edit endpoint / API key. |

`AONE_CONFIG_HOME` overrides the config directory, useful for tests so they
don't pollute the real `~/.config/aone`.

## Project configuration: `aone.sandbox.toml`

Template-related commands accept `-p/--path` (project root) and `--config`
(explicit file). When neither is given, the file in the current directory is
used. Fields:

```toml
template_id   = "tpl_xxx"   # written automatically after build
template_name = "my-app"
dockerfile    = "Dockerfile"
start_cmd     = ""
ready_cmd     = ""
cpu_count     = 0
memory_mb     = 0
```

Commands that consume the file: `template build`, `template create`,
`template delete`, `template publish` / `unpublish`, `sandbox create`.

## Command reference

Top-level groups (run `aone <cmd> --help` for full flag listings):

```
aone auth        login | logout | info | configure
aone sandbox     list | create | connect | exec | logs | metrics | info |
                 kill | pause | resume
aone sandbox template   init | create | delete | publish | unpublish |
                        list | get | builds | migrate
aone sandbox volume     list | create | info | delete | ls | cat | cp |
                        rm | mkdir
```

Common flags:

- `-v` / `-vv` — debug / trace logs to stderr (network calls, config resolution, redacted headers + bodies). Stdout stays clean so pipelines like `aone sandbox list -f json | jq` keep working.
- `--debug` — alias of `-v`; also sets `AONE_DEBUG=1` so SDK-level debug paths fire.
- `--version` — print the CLI version.
- `--format pretty|json` — supported on `list`, `info`, `logs`, `metrics`.

## Debug mode

When something goes wrong, re-run the command with `-v` (debug) or `-vv`
(trace). Output goes to **stderr** so pipelines aren't affected.

```bash
aone -v  sandbox list                # HTTP method/url/status/duration, config source
aone -vv sandbox list                # + redacted request/response headers and bodies
```

Triggers (any one works; higher precedence wins):

| Trigger | Resolved level |
|---|---|
| `AONE_LOG_LEVEL=trace\|debug\|info\|warn\|error` | as named |
| `-vv` / `AONE_DEBUG=2` | trace |
| `-v`  / `--debug` / `AONE_DEBUG=1` | debug |
| _(none)_ | silent (warnings/errors only) |

Other knobs:

- `AONE_LOG_FORMAT=json` — emit JSON records instead of human-readable text.
- `AONE_LOG_FILE=/tmp/aone.log` — write logs to this file (mode 0600) instead of stderr.

API keys, `Authorization`, cookies, and JSON fields like `apiKey` / `password` / `token` are masked before logging.

## Environment variables

| Variable | Purpose |
|---|---|
| `AONE_API_KEY` | API key (overrides config file) |
| `AONE_SANDBOX_API_URL` | Control-plane endpoint |
| `AONE_DEBUG` | `1`/`true` → debug logs; `2`/`trace` → trace logs |
| `AONE_LOG_LEVEL` | Explicit level (`trace`/`debug`/`info`/`warn`/`error`) — overrides `-v`/`AONE_DEBUG` |
| `AONE_LOG_FORMAT` | `json` switches log records to JSON; default is text |
| `AONE_LOG_FILE` | Write log records to this file (mode 0600) instead of stderr |
| `AONE_CONFIG_HOME` | Override `~/.config/aone` (test isolation) |

## Repository layout

This repository is organized as a CLI plus multi-language SDK monorepo:

| Path | Purpose |
|---|---|
| `cmd/` | Go CLI entrypoint and command wiring |
| `internal/` | CLI-only implementation details |
| `packages/go/sandbox/` | Go sandbox SDK module |
| `packages/` | Home for language SDK packages such as future JS and Python SDKs |
| `spec/` | Shared OpenAPI and proto specifications used by SDK code generation |

See `docs/release.md` for the Go SDK and CLI versioning rules.
