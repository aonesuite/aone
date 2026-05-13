# Aone SDK packages

`packages/` contains independently published SDK packages for AoneSuite.

Current packages:

- `go`: Go SDK module. Its current public packages are:
  - `sandbox`, covering sandbox, template, volume, filesystem, command, PTY, and git operations.
  - `tts`, covering text-to-speech voice listing and synthesis.

Go users import the sandbox package directly:

```go
import "github.com/aonesuite/aone/packages/go/sandbox"
```

Text-to-speech users import the TTS package directly:

```go
import "github.com/aonesuite/aone/packages/go/tts"
```

The Go SDK keeps the complete generated OpenAPI client under `go/internal/aoneapi` as a shared implementation detail. Public SDK packages should expose focused, module-level APIs such as `go/sandbox`, `go/tts`, or future `go/projects` instead of making the generated client the public surface.

Planned language SDKs should keep their own manifest, tests, generation scripts, and README in their package directory. Shared OpenAPI and proto inputs live in `../spec/`; root-level Makefile targets should only orchestrate package-local build, test, and code generation commands.

For the nested Go SDK module, release tags must use the module path prefix, for example `packages/go/v0.1.0`. The root CLI may use a local `replace` during development, but CLI releases should depend on a real Go SDK version and pass `make releasecheck`.
