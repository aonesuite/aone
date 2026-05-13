# Text To Speech

The `aone tts` command group and `packages/go/tts` SDK package use the shared
Aone API endpoint and authentication settings.

## Authentication

TTS commands use the same environment and config-file credentials as the rest
of the CLI:

```text
env AONE_API_KEY   >  ~/.config/aone/config.json
env AONE_API_URL   >  config file  >  built-in default
```

For normal CLI usage:

```sh
aone auth login --api-key ak_xxx
```

For a one-off CLI invocation, set environment variables:

```sh
AONE_API_KEY=ak_xxx AONE_API_URL=https://api.example.test aone tts voices
```

## List Voices

```sh
aone tts voices
```

Output columns:

- `VOICE ID`
- `NAME`
- `LANGUAGE`
- `GENDER`
- `SCENARIO`

Use JSON output for scripts:

```sh
aone tts voices --format json
```

`voices` also has the alias `voice`.

## Synthesize Speech

```sh
aone tts speech \
  --text "Hello from Aone." \
  --voice voice-1 \
  --audio-format mp3 \
  --speed 1.0
```

`--text` and `--voice` are required. `--audio-format` and `--speed` are optional
and passed through to the API.

By default, the command prints the generated audio URL and duration when the API
returns one:

```text
Audio URL:   https://...
Duration ms: 1234
```

Use JSON output when you need the raw response:

```sh
aone tts speech --text "Hello" --voice voice-1 --json
```

## Go SDK

Install the SDK module:

```sh
go get github.com/aonesuite/aone/packages/go@latest
```

List voices:

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
```

Synthesize text:

```go
format := "mp3"
speed := float32(1.0)

audio, err := client.Synthesize(ctx, tts.SynthesizeParams{
	Text:   "Hello from Aone.",
	Voice:  "voice-1",
	Format: &format,
	Speed:  &speed,
})
if err != nil {
	return err
}
fmt.Println(audio.AudioURL)
```

`tts.NewClient` uses `Config.APIKey` and `Config.Endpoint` when provided. When
those fields are empty, it reads `AONE_API_KEY` and `AONE_API_URL`. The default
endpoint is `https://api.aonesuite.com`.

Use `Config.HTTPClient` for a custom client, and `Config.RequestTimeout` to
apply a default timeout when the request context has no deadline.
