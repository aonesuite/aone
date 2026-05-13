# Authentication And Debugging

The CLI and Go SDK packages share the same Aone API key and endpoint
conventions.

## Credential Storage

`aone auth` writes user-level credentials to:

```text
${AONE_CONFIG_HOME:-~/.config/aone}/config.json
```

The config file is written with mode `0600`.

Save credentials:

```sh
aone auth login --api-key ak_xxx
```

`auth login` verifies the key by making a small sandbox list request. Use
`--no-verify` to save the key without a network check:

```sh
aone auth login --api-key ak_xxx --no-verify
```

Save a custom endpoint:

```sh
aone auth login --api-key ak_xxx --endpoint https://api.example.test
```

Inspect the active credential source:

```sh
aone auth info
```

Clear the saved API key:

```sh
aone auth logout
```

`auth logout` preserves a saved endpoint so staging or private deployments do
not have to be configured again after key rotation.

Update values non-interactively:

```sh
aone auth configure --api-key ak_new
aone auth configure --endpoint https://api.example.test
```

Without flags, `auth configure` prompts for values.

## Resolution Precedence

Regular CLI commands resolve credentials in this order:

```text
env AONE_API_KEY   >  config file
env AONE_API_URL   >  config file  >  https://api.aonesuite.com
```

`auth login --api-key/--endpoint` and `auth configure --api-key/--endpoint`
write values to the config file. They are not global temporary override flags
for sandbox or TTS commands.

For one-off CLI overrides, set environment variables:

```sh
AONE_API_KEY=ak_xxx AONE_API_URL=https://api.example.test aone sandbox list
```

The Go SDK packages have an additional caller-controlled layer:
`Config.APIKey` and `Config.Endpoint` are used before environment-variable
fallbacks.

`AONE_CONFIG_HOME` changes the directory that contains `config.json`. This is
useful for tests and local experiments:

```sh
AONE_CONFIG_HOME="$(mktemp -d)" aone auth login --api-key ak_test --no-verify
```

Sandbox CLI commands also load a `.env` file from the current directory.
Existing process environment variables win over `.env` values.

## Debug Logs

Normal command output is written to stdout. Debug logs are written to stderr so
JSON and shell pipelines stay clean.

Enable debug logging:

```sh
aone -v sandbox list
aone --debug sandbox list
```

Enable trace logging, including redacted request and response headers/bodies:

```sh
aone -vv sandbox list
```

Environment controls:

| Variable | Purpose |
|---|---|
| `AONE_DEBUG=1` or `AONE_DEBUG=debug` | Enable debug logs |
| `AONE_DEBUG=2` or `AONE_DEBUG=trace` | Enable trace logs |
| `AONE_LOG_LEVEL=trace|debug|info|warn|error` | Pin an explicit level |
| `AONE_LOG_FORMAT=json` | Emit JSON log records |
| `AONE_LOG_FILE=/tmp/aone.log` | Write logs to a file instead of stderr |

`AONE_LOG_LEVEL` has the highest priority. For example,
`AONE_LOG_LEVEL=error aone -vv sandbox list` still logs only errors.

Log files are opened with mode `0600`.

## Redaction

Debug and trace logs redact known sensitive values before writing:

- Authentication headers such as `Authorization`, `X-API-Key`, cookies, and
  proxy authorization.
- Sensitive URL query or fragment parameters such as `token`, `access_token`,
  `api_key`, and cloud-provider signature parameters.
- JSON fields such as `apiKey`, `api_key`, `password`, `secret`, `token`,
  `credential`, and `credentials`.

Trace body dumps are capped to avoid flooding logs.
