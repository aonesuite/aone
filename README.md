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

A legacy `e2b.toml` is read (with a warning) when `aone.sandbox.toml` is
missing, so existing projects can migrate incrementally.

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

- `--debug` — equivalent to `AONE_DEBUG=1`; enables verbose SDK logging.
- `--version` / `-v` — print the CLI version.
- `--format pretty|json` — supported on `list`, `info`, `logs`, `metrics`.

## Environment variables

| Variable | Purpose |
|---|---|
| `AONE_API_KEY` | API key (overrides config file) |
| `AONE_SANDBOX_API_URL` | Control-plane endpoint |
| `AONE_DEBUG` | `1` / `true` enables SDK debug logs |
| `AONE_CONFIG_HOME` | Override `~/.config/aone` (test isolation) |
