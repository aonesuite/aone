# Changelog

## Unreleased

This section records user-visible changes currently on `main`.

### Added

- Added `aone tts voices` and `aone tts speech` for text-to-speech APIs.
- Added the public Go SDK package `github.com/aonesuite/aone/packages/go/tts`.
- Added template migration with `aone sandbox template migrate`, which converts
  Dockerfiles into SDK-native template code for Go, TypeScript, or Python.
- Added `aone sandbox template create <template-name>` as the primary Dockerfile
  template build command.
- Added `make releasecheck` to block root CLI releases that still depend on the
  local development Go SDK module graph.

### Changed

- The Go SDK module now lives at `github.com/aonesuite/aone/packages/go`.
  Application imports stay package-specific, such as
  `github.com/aonesuite/aone/packages/go/sandbox` and
  `github.com/aonesuite/aone/packages/go/tts`.
- The root CLI module uses `go.work` and a local `replace` during development.
  Root CLI releases must first depend on a real `packages/go/vX.Y.Z` SDK tag.
- Sandbox and template CLI commands now align more closely with the current SDK
  and API contracts. Use `aone <command> --help` for exact flags.
- `aone sandbox template build` remains available as a hidden lower-level
  command; `aone sandbox template create <template-name>` is the normal user
  entrypoint.

### Removed

- Removed legacy volume, pause/resume, and snapshot command/API surfaces that no
  longer match the current sandbox SDK contracts.

### Migration Notes

- If a project required the old nested sandbox module directly, update the
  requirement to `github.com/aonesuite/aone/packages/go vX.Y.Z`. Keep source
  imports on `github.com/aonesuite/aone/packages/go/sandbox`.
- Before tagging a root CLI release, publish or select a real Go SDK tag such as
  `packages/go/v0.1.0`, update root `go.mod`, remove the local SDK `replace`,
  and run `make releasecheck`.
