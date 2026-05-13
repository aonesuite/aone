# Development

This repository contains the root `aone` CLI module and the nested Go SDK
module at `packages/go`.

## Requirements

- Go 1.26.2 or newer for the root CLI module.
- Go 1.22 or newer for the nested Go SDK module.
- `staticcheck` for `make staticcheck`.
- `buf`, `protoc-gen-go`, and `protoc-gen-connect-go` for sandbox envd proto
  generation.

## Workspace

Local development should use the checked-in `go.work` file:

```sh
go work sync
go test ./...
cd packages/go && go test ./...
```

The root module intentionally depends on `github.com/aonesuite/aone/packages/go
v0.0.0` with a local `replace` during development. Do not cut a root CLI release
from that module graph; use `docs/release.md` for the release flow.

## Common Commands

```sh
make build          # build ./bin/aone
make install        # go install the CLI
make fmt            # gofmt all Go files
make unittest       # root + Go SDK unit tests
make test           # root + Go SDK tests with failfast/count/timeout
make integrationtest
make staticcheck
```

Run SDK-only tests from the nested module when changing public SDK behavior:

```sh
cd packages/go
go test ./...
```

## Configuration During Development

The CLI resolves credentials from flags, environment variables, and
`${AONE_CONFIG_HOME:-~/.config/aone}/config.json`.

Use `AONE_CONFIG_HOME` in tests or local experiments when you do not want to
touch your real config:

```sh
AONE_CONFIG_HOME="$(mktemp -d)" go test ./cmd
```

Sandbox CLI commands also load a project-local `.env` file from the current
directory. Variables already set in the process environment win over values in
`.env`.

## Code Generation

Regenerate SDK clients after OpenAPI or proto changes:

```sh
make generate
```

This runs:

- `generate-aone` for `spec/openapi.yml` into `packages/go/internal/aoneapi`.
- `generate-sandbox` for `spec/sandbox/envd/*` into
  `packages/go/sandbox/internal/envdapi`.

Generated packages are intentionally internal. Public SDK packages should expose
focused APIs rather than exporting generated clients directly.
