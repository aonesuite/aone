# Aone SDK packages

`packages/` contains independently published SDK packages for AoneSuite.

Current packages:

- `go/sandbox`: Go SDK for sandbox, template, volume, filesystem, command, PTY, and git operations.

Planned packages should keep their own manifest, tests, generation scripts, and README in their package directory. Shared OpenAPI and proto inputs live in `../spec/`; root-level Makefile targets should only orchestrate package-local build, test, and code generation commands.

For the nested Go SDK module, release tags must use the module path prefix, for example `packages/go/sandbox/v0.1.0`. The root CLI may use a local `replace` during development, but CLI releases should depend on a real Go SDK version and pass `make releasecheck`.
