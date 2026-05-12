package log

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

// TestResolveLevel_PrecedencePicks verifies the precedence rules: explicit
// AONE_LOG_LEVEL must trump -v/-vv, which must trump AONE_DEBUG, which
// must trump --debug, which must trump the default.
func TestResolveLevel_PrecedencePicks(t *testing.T) {
	cases := []struct {
		name string
		opts ResolveOptions
		want slog.Level
	}{
		{
			name: "default is warn",
			opts: ResolveOptions{Env: emptyEnv},
			want: slog.LevelWarn,
		},
		{
			name: "--debug enables debug",
			opts: ResolveOptions{DebugFlag: true, Env: emptyEnv},
			want: slog.LevelDebug,
		},
		{
			name: "AONE_DEBUG=1 enables debug",
			opts: ResolveOptions{Env: envMap(map[string]string{"AONE_DEBUG": "1"})},
			want: slog.LevelDebug,
		},
		{
			name: "AONE_DEBUG=2 enables trace",
			opts: ResolveOptions{Env: envMap(map[string]string{"AONE_DEBUG": "2"})},
			want: LevelTrace,
		},
		{
			name: "-v enables debug",
			opts: ResolveOptions{Verbosity: 1, Env: emptyEnv},
			want: slog.LevelDebug,
		},
		{
			name: "-vv enables trace",
			opts: ResolveOptions{Verbosity: 2, Env: emptyEnv},
			want: LevelTrace,
		},
		{
			name: "explicit AONE_LOG_LEVEL=error wins over verbosity",
			opts: ResolveOptions{Verbosity: 2, Env: envMap(map[string]string{"AONE_LOG_LEVEL": "error"})},
			want: slog.LevelError,
		},
		{
			name: "explicit AONE_LOG_LEVEL=info wins over AONE_DEBUG",
			opts: ResolveOptions{Env: envMap(map[string]string{
				"AONE_LOG_LEVEL": "info",
				"AONE_DEBUG":     "1",
			})},
			want: slog.LevelInfo,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := slog.Level(ResolveLevel(tc.opts))
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestInit_WritesToProvidedStderrAtDebug ensures Init wires the slog
// handler at the requested level and routes records to the supplied
// writer. The default invocation (no flags, no env) must not emit
// anything — that's the contract for clean stdout/stderr in scripted use.
func TestInit_WritesToProvidedStderrAtDebug(t *testing.T) {
	var buf bytes.Buffer
	Init(InitOptions{
		ResolveOptions: ResolveOptions{DebugFlag: true, Env: emptyEnv},
		Stderr:         &buf,
	})
	t.Cleanup(resetGlobalLogger)

	Debug("hello", "k", "v")
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("debug log not captured: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "k=v") {
		t.Fatalf("debug log missing attrs: %q", buf.String())
	}
}

// TestInit_SilentByDefault confirms that with no flags and an empty env
// the package emits nothing — stderr must stay pristine for normal use.
func TestInit_SilentByDefault(t *testing.T) {
	var buf bytes.Buffer
	Init(InitOptions{ResolveOptions: ResolveOptions{Env: emptyEnv}, Stderr: &buf})
	t.Cleanup(resetGlobalLogger)

	Debug("should not appear")
	Info("should not appear either")
	if buf.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %q", buf.String())
	}
}

// TestInit_JSONFormat checks that AONE_LOG_FORMAT=json swaps the handler
// to JSON output. Useful when log lines need to be machine-parsed.
func TestInit_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	Init(InitOptions{
		ResolveOptions: ResolveOptions{DebugFlag: true, Env: envMap(map[string]string{"AONE_LOG_FORMAT": "json"})},
		Stderr:         &buf,
	})
	t.Cleanup(resetGlobalLogger)

	Debug("hello", "k", "v")
	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, `"msg":"hello"`) {
		t.Fatalf("expected JSON log, got: %q", out)
	}
}

// TestRedactHeaders_MasksSensitive verifies the header redactor keeps
// non-sensitive headers verbatim but masks API keys, cookies, and auth
// tokens. The input must not be mutated.
func TestRedactHeaders_MasksSensitive(t *testing.T) {
	in := http.Header{}
	in.Set("X-API-Key", "abcd1234efgh5678")
	in.Set("Authorization", "Bearer secret-token-12345")
	in.Set("Cookie", "session=abcdef123456")
	in.Set("Content-Type", "application/json")

	out := RedactHeaders(in)

	if got := out.Get("X-Api-Key"); got == "abcd1234efgh5678" || !strings.Contains(got, "*") {
		t.Fatalf("X-API-Key not masked: %q", got)
	}
	if got := out.Get("Authorization"); got == "Bearer secret-token-12345" || !strings.Contains(got, "*") {
		t.Fatalf("Authorization not masked: %q", got)
	}
	if got := out.Get("Cookie"); got == "session=abcdef123456" || !strings.Contains(got, "*") {
		t.Fatalf("Cookie not masked: %q", got)
	}
	if got := out.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type should be untouched, got %q", got)
	}

	// Source unchanged.
	if in.Get("X-Api-Key") != "abcd1234efgh5678" {
		t.Fatalf("input mutated")
	}
}

// TestRedactBody_JSONFieldsMasked checks that known-sensitive fields in
// JSON bodies become "***" while other fields pass through. Catches the
// most common case of accidentally logging credentials in request bodies.
func TestRedactBody_JSONFieldsMasked(t *testing.T) {
	body := []byte(`{"apiKey":"k-123","name":"demo","nested":{"password":"p","ok":true}}`)
	got := RedactBody(body, "application/json")

	if strings.Contains(got, "k-123") {
		t.Fatalf("apiKey leaked: %q", got)
	}
	if strings.Contains(got, `"p"`) && !strings.Contains(got, `"***"`) {
		t.Fatalf("password leaked: %q", got)
	}
	if !strings.Contains(got, `"name":"demo"`) {
		t.Fatalf("non-sensitive field dropped: %q", got)
	}
	if !strings.Contains(got, `"ok":true`) {
		t.Fatalf("boolean field dropped: %q", got)
	}
}

// TestRedactBody_NonJSONTruncates ensures non-JSON payloads still come
// through (so we don't completely hide unexpected content types) but are
// truncated when oversized.
func TestRedactBody_NonJSONTruncates(t *testing.T) {
	big := strings.Repeat("x", maxBodyLogBytes+200)
	got := RedactBody([]byte(big), "text/plain")
	if len(got) > maxBodyLogBytes+64 {
		t.Fatalf("body not truncated; len=%d", len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("expected truncation marker: %q", got[len(got)-32:])
	}
}

// TestRedactBody_EmptyReturnsEmpty covers the zero-length fast path so we
// don't accidentally log "{}" for empty bodies.
func TestRedactBody_EmptyReturnsEmpty(t *testing.T) {
	if got := RedactBody(nil, ""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

// TestRedactURL_MasksSignedQueryParams is the regression test for the
// signed-file-URL leak: control-plane returns presigned URLs with a
// `signature` query that's a live access credential until its expiration
// passes. Pasting a debug log into a ticket cannot be allowed to leak it.
func TestRedactURL_MasksSignedQueryParams(t *testing.T) {
	in := "https://example.com/files/abc?signature=DEADBEEF1234&signature_expiration=1700000000&name=demo"
	got := RedactURL(in)

	if strings.Contains(got, "DEADBEEF1234") {
		t.Fatalf("signature leaked: %q", got)
	}
	if strings.Contains(got, "1700000000") {
		t.Fatalf("signature_expiration leaked: %q", got)
	}
	if !strings.Contains(got, "name=demo") {
		t.Fatalf("non-sensitive query dropped: %q", got)
	}
	// Path and host must still be present — the whole point of DEBUG logs
	// is knowing which endpoint was hit.
	if !strings.Contains(got, "example.com/files/abc") {
		t.Fatalf("path/host lost: %q", got)
	}
}

// TestRedactURL_MasksUserinfo guards the rarer but equally dangerous case
// of credentials embedded in the URL's userinfo section.
func TestRedactURL_MasksUserinfo(t *testing.T) {
	got := RedactURL("https://user:hunter2@example.com/path")
	if strings.Contains(got, "hunter2") {
		t.Fatalf("password leaked: %q", got)
	}
}

// TestRedactURL_HandlesMisc covers edge cases: empty input, plain URL
// without sensitive params, and a malformed URL (we don't echo it back
// because the operator can't tell at a glance whether it's safe).
func TestRedactURL_HandlesMisc(t *testing.T) {
	if got := RedactURL(""); got != "" {
		t.Fatalf("empty -> %q", got)
	}
	if got := RedactURL("https://example.com/list?limit=5"); got != "https://example.com/list?limit=5" {
		t.Fatalf("non-sensitive URL altered: %q", got)
	}
	// url.Parse is extremely permissive — we use a NUL byte to reliably
	// trip its error path.
	if got := RedactURL("https://example.com/\x00bad"); got != "<unparseable-url>" {
		t.Fatalf("malformed URL passed through: %q", got)
	}
}

// TestRedactURL_UnparseableQueryFullyRedacted is the regression for the
// case where url.Parse succeeds but url.ParseQuery fails (bad percent-
// escape). The query part contains the leaked secret, so the earlier
// best-effort branch that echoed RawQuery back was unsafe.
func TestRedactURL_UnparseableQueryFullyRedacted(t *testing.T) {
	// `%zz` is not a valid percent-escape; ParseQuery returns an error
	// while url.Parse still produces a *URL with RawQuery set.
	in := "https://example.com/files/abc?signature=SECRET%zz&name=demo"
	got := RedactURL(in)

	if strings.Contains(got, "SECRET") {
		t.Fatalf("signature leaked on parse failure: %q", got)
	}
	if !strings.Contains(got, "redacted-unparseable-query") {
		t.Fatalf("missing redaction marker: %q", got)
	}
	// Path/host must still be present so the operator knows which
	// endpoint was hit.
	if !strings.Contains(got, "example.com/files/abc") {
		t.Fatalf("path/host lost: %q", got)
	}
}

// TestRedactURL_MasksFragmentTokens covers the OAuth-implicit-grant case:
// `#access_token=...&token_type=Bearer&expires_in=3600`. Fragments don't
// hit the wire, but they end up in browser address bars, screenshots,
// and copy-pasted URLs — anywhere a credential we'd otherwise mask in
// the query string can slip past unredacted.
func TestRedactURL_MasksFragmentTokens(t *testing.T) {
	in := "https://callback.example.com/cb#access_token=LIVE-OAUTH-TOKEN&token_type=Bearer&state=ok"
	got := RedactURL(in)

	if strings.Contains(got, "LIVE-OAUTH-TOKEN") {
		t.Fatalf("fragment access_token leaked: %q", got)
	}
	// Non-sensitive fragment fields are preserved so the log still tells
	// you what kind of callback it was.
	if !strings.Contains(got, "state=ok") {
		t.Fatalf("non-sensitive fragment field dropped: %q", got)
	}
	if !strings.Contains(got, "callback.example.com/cb") {
		t.Fatalf("path/host lost: %q", got)
	}
}

// TestRedactURL_UnparseableFragmentFullyRedacted mirrors the query-side
// fallback. url.Parse itself is stricter about fragments than queries
// (it may reject bad percent-escapes outright), but either way the
// secret must not appear in the output.
func TestRedactURL_UnparseableFragmentFullyRedacted(t *testing.T) {
	in := "https://example.com/cb#access_token=LIVE%zz&x=1"
	got := RedactURL(in)
	if strings.Contains(got, "LIVE") {
		t.Fatalf("fragment token leaked: %q", got)
	}
	// Acceptable outcomes: full unparseable-URL placeholder, or just
	// the fragment redacted. We don't pin which since net/url's
	// validation rules can change across Go versions.
	if !strings.Contains(got, "unparseable") {
		t.Fatalf("expected an unparseable marker: %q", got)
	}
}

// emptyEnv is a Getenv stub that returns "" for every key. Used to make
// ResolveLevel tests independent of the host environment.
func emptyEnv(string) string { return "" }

// envMap returns a Getenv stub backed by an in-memory map.
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// resetGlobalLogger restores the package-wide logger to its silent zero
// state so tests don't bleed into each other.
func resetGlobalLogger() {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.logger = slog.New(discardHandler{})
	global.level = Level(slog.LevelError + 1)
}
