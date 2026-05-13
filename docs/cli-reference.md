# CLI Reference

Run `aone <command> --help` for the canonical flag list. This page summarizes
the current command groups, aliases, and commonly used flags.

## Global Flags

| Flag | Purpose |
|---|---|
| `-v`, `--verbose` | Increase log verbosity. Use `-v` for debug and `-vv` for trace. |
| `--debug` | Enable debug logging, equivalent to `-v` / `AONE_DEBUG=1`. |
| `--version` | Print the CLI version. |

Debug logs go to stderr. Command output stays on stdout.

## Auth

| Command | Purpose |
|---|---|
| `aone auth login` | Save an API key to the local credential store. |
| `aone auth info` | Show the active API key and endpoint sources. |
| `aone auth logout` | Remove the saved API key. |
| `aone auth configure` | Edit saved API key or endpoint. |

Useful flags:

```sh
aone auth login --api-key ak_xxx --endpoint https://api.example.test
aone auth login --api-key ak_xxx --no-verify
aone auth configure --api-key ak_new
aone auth configure --endpoint https://api.example.test
```

## Sandboxes

`aone sandbox` has the alias `aone sbx`.

| Command | Alias | Purpose |
|---|---|---|
| `aone sandbox list` | `ls` | List sandboxes. |
| `aone sandbox create [template]` | `cr` | Create a sandbox and connect to its terminal. |
| `aone sandbox connect <sandboxID>` | `cn` | Connect to an existing sandbox terminal. |
| `aone sandbox info <sandboxID>` | `in` | Show sandbox details. |
| `aone sandbox kill [sandboxIDs...]` | `kl` | Kill one or more sandboxes. |
| `aone sandbox exec <sandboxID> -- <command...>` | `ex` | Execute a command in a sandbox. |
| `aone sandbox logs <sandboxID>` | `lg` | View sandbox logs. |
| `aone sandbox metrics <sandboxID>` | `mt` | View sandbox resource metrics. |

Common `list` flags:

```sh
aone sandbox list --state running,paused --limit 20 --format json
```

Common `create` flags:

```sh
aone sandbox create my-template \
  --detach \
  --timeout 3600 \
  --env-var KEY=VALUE \
  --metadata owner=platform,service=api \
  --allow-out api.example.com:443 \
  --deny-out 169.254.169.254
```

`sandbox create` also accepts `--config` and `--path` to locate
`aone.sandbox.toml`. When no template argument is provided, it reads
`template_id` from the project config and falls back to `base` when none is
available.

Common `exec` flags:

```sh
aone sandbox exec sbx_xxx -- pwd
aone sandbox exec sbx_xxx --cwd /workspace --user root --env FOO=bar -- printenv FOO
aone sandbox exec sbx_xxx --background -- sleep 60
```

Common log and metrics flags:

```sh
aone sandbox logs sbx_xxx --follow --level INFO --format json
aone sandbox metrics sbx_xxx --follow --start 1735689600 --end 1735693200
```

## Templates

`aone sandbox template` has the alias `aone sandbox tpl`.

| Command | Alias | Purpose |
|---|---|---|
| `aone sandbox template init` | `it` | Initialize a template project. |
| `aone sandbox template create <template-name>` | `ct` | Build a Dockerfile as a sandbox template. |
| `aone sandbox template list` | `ls` | List templates. |
| `aone sandbox template get <templateID>` | `gt` | Get template details. |
| `aone sandbox template delete [templateIDs...]` | `dl` | Delete templates. |
| `aone sandbox template publish [templateIDs...]` | `pb` | Publish templates. |
| `aone sandbox template unpublish [templateIDs...]` | `upb` | Unpublish templates. |
| `aone sandbox template builds <templateID> <buildID>` | `bds` | View build status. |
| `aone sandbox template logs <templateID> <buildID>` | `blg` | View build logs. |
| `aone sandbox template migrate` | | Convert a Dockerfile to SDK-native template code. |

Create a template from a Dockerfile:

```sh
aone sandbox template create my-app \
  --dockerfile Dockerfile \
  --path . \
  --cmd "/usr/local/bin/my-app" \
  --ready-cmd "curl -fsS http://localhost:8080/health" \
  --cpu-count 2 \
  --memory-mb 2048 \
  --disk-size-mb 10240
```

List and inspect templates:

```sh
aone sandbox template list --name my-app --build-status ready --format json
aone sandbox template get tpl_xxx
```

View build logs:

```sh
aone sandbox template logs tpl_xxx build_xxx --level INFO --limit 100
```

Migrate a Dockerfile:

```sh
aone sandbox template migrate --language go --path .
aone sandbox template migrate --language typescript --dockerfile ./Dockerfile
```

The lower-level `aone sandbox template build` command exists but is hidden from
normal help output. Prefer `template create` for Dockerfile projects.

## Text To Speech

| Command | Alias | Purpose |
|---|---|---|
| `aone tts voices` | `voice` | List available TTS voices. |
| `aone tts speech` | | Synthesize text to speech. |

Examples:

```sh
aone tts voices --format json

aone tts speech \
  --text "Hello from Aone." \
  --voice voice-1 \
  --audio-format mp3 \
  --speed 1.0 \
  --json
```
