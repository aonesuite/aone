# Release model

This repository is a Go CLI plus multi-language SDK monorepo. The root module builds the `aone` CLI. SDKs under `packages/` are independently published packages.

## Go SDK

The Go sandbox SDK lives at `packages/go/sandbox` and has its own module path:

```text
github.com/aonesuite/aone/packages/go/sandbox
```

Because it is a nested Go module, its Git tags must include the module directory prefix:

```text
packages/go/sandbox/v0.1.0
packages/go/sandbox/v0.1.1
```

Before a root CLI release, publish or select a real Go SDK version and update the root `go.mod` to require that version instead of `v0.0.0`.

## Root CLI

The root CLI can use `go.work` and a local `replace` while developing inside the monorepo. That is a development convenience only.

Do not release the root CLI while either of these is true:

- `go.mod` requires `github.com/aonesuite/aone/packages/go/sandbox v0.0.0`
- `go.mod` contains `replace github.com/aonesuite/aone/packages/go/sandbox => ./packages/go/sandbox`

Run this before creating a CLI tag:

```sh
make releasecheck
```

The release check intentionally runs with `GOWORK=off` so it verifies the module graph that remote users get through commands like:

```sh
go install github.com/aonesuite/aone@latest
```

## Development

Local development should use the checked-in `go.work` file:

```sh
go test ./...
cd packages/go/sandbox && go test ./...
```

When the Go SDK has a real published version, the preferred steady state is:

- root `go.mod` requires the released SDK version
- root `go.mod` has no local SDK `replace`
- `go.work` provides local workspace linking for contributors
