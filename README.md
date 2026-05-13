# aone

`aone` is the AoneSuite command-line tool and SDK monorepo. The root module
builds the CLI for managing sandboxes, templates, and account
credentials against the Aone API. `packages/go` contains the Go SDK packages
for sandbox automation and text-to-speech.

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

Local development uses Go workspaces:

```sh
go test ./...
cd packages/go && go test ./...
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
flag --endpoint  >  env AONE_API_URL  >  config file  >  built-in default
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
aone sandbox     list | create | connect | info | kill | exec | logs |
                 metrics
aone sandbox template   init | create | list | get | delete | publish |
                        unpublish | builds | migrate
```

Common flags:

- `-v` / `-vv` — debug / trace logs to stderr (network calls, config resolution, redacted headers + bodies). Stdout stays clean so pipelines like `aone sandbox list -f json | jq` keep working.
- `--debug` — alias of `-v`; also sets `AONE_DEBUG=1` so SDK-level debug paths fire.
- `--version` — print the CLI version.
- `--format pretty|json` — supported on `list`, `info`, `logs`, `metrics`.

Common aliases:

- `aone sandbox` can be shortened to `aone sbx`.
- `aone sandbox template` can be shortened to `aone sandbox tpl`.
- Frequently used subcommands also have short aliases, for example `list` /
  `ls`, `create` / `cr`, `connect` / `cn`, `exec` / `ex`, and `logs` / `lg`.

`aone sandbox template create <template-name>` is the primary template build
surface. The lower-level `template build` command still exists internally but
is hidden from normal help output.

`aone sandbox template migrate` converts an existing Dockerfile into
SDK-native template code for Go, TypeScript, or Python.

## Go SDK

The Go SDK lives in the nested module:

```text
github.com/aonesuite/aone/packages/go
```

Current public packages:

| Package | Purpose |
|---|---|
| `github.com/aonesuite/aone/packages/go/sandbox` | Create and operate sandboxes, commands, files, PTY sessions, Git repositories, templates, logs, and metrics. |
| `github.com/aonesuite/aone/packages/go/tts` | List TTS voices and synthesize text into audio. |

Sandbox example:

```go
import "github.com/aonesuite/aone/packages/go/sandbox"

client, err := sandbox.NewClient(&sandbox.Config{
	APIKey: os.Getenv("AONE_API_KEY"),
})
if err != nil {
	return err
}

sb, _, err := client.CreateAndWait(ctx, sandbox.CreateParams{
	TemplateID: "base",
}, sandbox.WithPollInterval(2*time.Second))
if err != nil {
	return err
}
defer sb.Kill(ctx)
```

TTS example:

```go
import "github.com/aonesuite/aone/packages/go/tts"

client, err := tts.NewClient(&tts.Config{
	APIKey: os.Getenv("AONE_API_KEY"),
})
if err != nil {
	return err
}

voices, err := client.ListVoices(ctx)
if err != nil {
	return err
}
if len(voices) == 0 {
	return fmt.Errorf("no TTS voices available")
}

audio, err := client.Synthesize(ctx, tts.SynthesizeParams{
	Text:  "Hello from Aone.",
	Voice: voices[0].ID,
})
if err != nil {
	return err
}
fmt.Println(audio.AudioURL)
```

Both SDK packages read `AONE_API_KEY` when `Config.APIKey` is empty and
`AONE_API_URL` when `Config.Endpoint` is empty. The default endpoint is
`https://api.aonesuite.com`.

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
| `AONE_API_URL` | Aone API endpoint |
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
| `packages/go/` | Go SDK module; `sandbox/` and `tts/` are public packages, and `internal/` holds shared generated clients |
| `packages/` | Home for language SDK packages such as future JS and Python SDKs |
| `spec/` | Shared OpenAPI and proto specifications used by SDK code generation |

See `docs/release.md` for the Go SDK and CLI versioning rules.
