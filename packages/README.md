# Aone SDK packages

`packages/` contains independently published SDK packages for AoneSuite. Each
language package owns its own module manifest, tests, generation config, and
README so SDKs can evolve without turning the root CLI module into the public
SDK surface.

Current packages:

- `go`: Go SDK module. Its current public packages are:
  - `sandbox`, covering sandbox, template, filesystem, command, PTY, and git operations.
  - `tts`, covering text-to-speech voice listing and synthesis.

Go users import the sandbox package directly:

```go
import "github.com/aonesuite/aone/packages/go/sandbox"
```

Text-to-speech users import the TTS package directly:

```go
import "github.com/aonesuite/aone/packages/go/tts"
```

The Go SDK keeps the complete generated OpenAPI client under
`go/internal/aoneapi` as a shared implementation detail. Public SDK packages
should expose focused, module-level APIs such as `go/sandbox`, `go/tts`, or
future `go/projects` instead of making the generated client the public surface.

## Go SDK quick start

Install the Go SDK module:

```sh
go get github.com/aonesuite/aone/packages/go@latest
```

Sandbox:

```go
import "github.com/aonesuite/aone/packages/go/sandbox"

client, err := sandbox.NewClient(&sandbox.Config{})
if err != nil {
	return err
}

sb, _, err := client.CreateAndWait(ctx, sandbox.CreateParams{
	TemplateID: "base",
})
if err != nil {
	return err
}
defer sb.Kill(ctx)
```

Text-to-speech:

```go
import "github.com/aonesuite/aone/packages/go/tts"

client, err := tts.NewClient(&tts.Config{})
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

Both packages read `AONE_API_KEY` and `AONE_API_URL` when the corresponding
config fields are empty.

Planned language SDKs should keep their own manifest, tests, generation
scripts, and README in their package directory. Shared OpenAPI and proto inputs
live in `../spec/`; root-level Makefile targets should only orchestrate
package-local build, test, and code generation commands.

For the nested Go SDK module, release tags must use the module path prefix, for
example `packages/go/v0.1.0`. The root CLI may use a local `replace` during
development, but CLI releases should depend on a real Go SDK version and pass
`make releasecheck`.
