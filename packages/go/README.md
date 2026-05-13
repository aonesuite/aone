# Aone Go SDK

The Go SDK module lives at:

```text
github.com/aonesuite/aone/packages/go
```

Install it with:

```sh
go get github.com/aonesuite/aone/packages/go@latest
```

The module currently supports Go 1.22 or newer.

## Packages

| Package | Purpose |
|---|---|
| `github.com/aonesuite/aone/packages/go/sandbox` | Create and operate sandboxes, commands, files, PTY sessions, Git repositories, templates, logs, and metrics. |
| `github.com/aonesuite/aone/packages/go/tts` | List TTS voices and synthesize text into audio. |

Both packages read `AONE_API_KEY` when `Config.APIKey` is empty and
`AONE_API_URL` when `Config.Endpoint` is empty. The default endpoint is
`https://api.aonesuite.com`.

## Sandbox

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

## Text To Speech

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

## Development

Run tests from the module directory:

```sh
go test ./...
```

The generated OpenAPI client is intentionally kept under `internal/aoneapi`.
Public packages should expose focused APIs instead of exporting the generated
client directly.
