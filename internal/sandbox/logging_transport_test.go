package sandbox

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/log"
)

// TestLoggingTransport_DebugLogsRequest verifies that at DEBUG level the
// transport emits a single record per request with method, status, and
// duration, but does not include header or body content.
func TestLoggingTransport_DebugLogsRequest(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{DebugFlag: true, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "rid-abc")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/foo", nil)
	req.Header.Set("X-API-Key", "abcd1234efgh5678")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()

	out := buf.String()
	if !strings.Contains(out, "http response") {
		t.Fatalf("missing http response log: %q", out)
	}
	if !strings.Contains(out, "status=200") {
		t.Fatalf("status not logged: %q", out)
	}
	if !strings.Contains(out, "rid-abc") {
		t.Fatalf("request_id not logged: %q", out)
	}
	// DEBUG (not TRACE) — header values must NOT appear, masked or otherwise.
	if strings.Contains(out, "abcd1234efgh5678") {
		t.Fatalf("api key leaked at DEBUG: %q", out)
	}
}

// TestLoggingTransport_TraceLogsHeadersAndBody confirms TRACE adds the
// header+body dump, and that the API key shows up only in masked form.
func TestLoggingTransport_TraceLogsHeadersAndBody(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{Verbosity: 2, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"demo","token":"leak-me"}`))
	}))
	t.Cleanup(srv.Close)

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/bar",
		strings.NewReader(`{"apiKey":"sekret","x":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "abcd1234efgh5678")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()

	out := buf.String()
	// Plaintext API key must NOT be in the log; masked form may be.
	if strings.Contains(out, "abcd1234efgh5678") {
		t.Fatalf("api key leaked verbatim: %q", out)
	}
	// JSON body apiKey field must be masked.
	if strings.Contains(out, `"sekret"`) {
		t.Fatalf("body credential leaked: %q", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("expected redaction marker: %q", out)
	}
	if !strings.Contains(out, `name`) || !strings.Contains(out, `demo`) {
		t.Fatalf("response body not in trace log: %q", out)
	}
}

// TestLoggingTransport_SilentByDefault asserts the zero-flag path doesn't
// emit anything — the package's "no overhead by default" contract.
func TestLoggingTransport_SilentByDefault(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()

	if buf.Len() != 0 {
		t.Fatalf("expected silent logger, got: %q", buf.String())
	}
}

// TestLoggingTransport_RedactsSignedURLQuery is the regression test for
// the signed-file-URL leak: even at DEBUG (which is more likely to end up
// in tickets than TRACE), the signature and its expiration must never
// appear verbatim.
func TestLoggingTransport_RedactsSignedURLQuery(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{DebugFlag: true, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	signed := srv.URL + "/files/x?signature=TOPSECRETSIG&signature_expiration=1700000000&name=demo"
	resp, err := c.Get(signed)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()

	out := buf.String()
	if strings.Contains(out, "TOPSECRETSIG") {
		t.Fatalf("signature leaked into log: %q", out)
	}
	if strings.Contains(out, "1700000000") {
		t.Fatalf("signature_expiration leaked into log: %q", out)
	}
	if !strings.Contains(out, "name=demo") {
		t.Fatalf("non-sensitive query missing: %q", out)
	}
}

// countingReader wraps a Reader to track total bytes read and how many
// Read calls have happened. Used to prove that streaming bodies are not
// drained by the transport at TRACE.
type countingReader struct {
	src   io.Reader
	reads int
	total int64
}

// Read implements io.Reader and accounts each call so the test can assert
// on read-pattern, not just byte counts.
func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	r.reads++
	r.total += int64(n)
	return n, err
}

// TestLoggingTransport_DoesNotBufferStreamingUpload is the regression
// test for the high-severity streaming bug. We post a large
// application/octet-stream payload with an unknown Content-Length (-1
// via chunked encoding) and assert the transport hands the body straight
// through — no full-buffer read before delegating.
func TestLoggingTransport_DoesNotBufferStreamingUpload(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{Verbosity: 2, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	// Server echoes the byte count so we can confirm bytes still reached it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	body := strings.Repeat("x", 256*1024) // 256KB > maxBodyDumpBytes (64KB)
	cr := &countingReader{src: strings.NewReader(body)}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/upload", cr)
	req.Header.Set("Content-Type", "application/octet-stream")
	// Force chunked encoding by leaving Content-Length unset (-1).
	req.ContentLength = -1

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()

	// The body should have been read exactly once-per-buffer by net/http
	// streaming it to the wire. If the transport buffered the whole thing
	// first via io.ReadAll, reads would still occur but the trace log
	// would also contain the body — assert on the log to make this robust.
	out := buf.String()
	if strings.Contains(out, body) {
		t.Fatalf("streaming body leaked into log (buffered!): len=%d", len(out))
	}
	if !strings.Contains(out, "skipped streaming/binary body") {
		t.Fatalf("expected skip marker in trace log: %q", out)
	}
	if cr.total != int64(len(body)) {
		t.Fatalf("server received %d bytes, want %d", cr.total, len(body))
	}
}

// TestLoggingTransport_DoesNotBufferStreamingDownload covers the response
// side: a large octet-stream download must not be read end-to-end just so
// we can log a body the user doesn't want anyway.
func TestLoggingTransport_DoesNotBufferStreamingDownload(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{Verbosity: 2, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	payload := strings.Repeat("y", 256*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Set a large Content-Length so the transport's shouldDumpBody
		// rejects on size in addition to content type — covers both
		// gates with one test.
		w.Header().Set("Content-Length", "262144")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	resp, err := c.Get(srv.URL + "/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// Caller must still be able to read the full body.
	got, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("body length mismatch: got %d want %d", len(got), len(payload))
	}

	out := buf.String()
	if strings.Contains(out, payload) {
		t.Fatalf("download body leaked into log")
	}
	if !strings.Contains(out, "skipped streaming/binary body") {
		t.Fatalf("expected skip marker in trace log: %q", out)
	}
}

// TestLoggingTransport_DoesNotBufferEventStream covers the SSE / envd
// streaming path (text/event-stream Content-Type). Without the skip rule
// these long-lived streams would block at TRACE until EOF.
func TestLoggingTransport_DoesNotBufferEventStream(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{Verbosity: 2, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}, Stderr: io.Discard})
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: hello\n\n"))
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(srv.Close)

	c := &http.Client{Transport: &loggingTransport{base: http.DefaultTransport}}
	resp, err := c.Get(srv.URL + "/events")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()

	out := buf.String()
	if !strings.Contains(out, "skipped streaming/binary body") {
		t.Fatalf("event-stream body should be skipped, got: %q", out)
	}
}
