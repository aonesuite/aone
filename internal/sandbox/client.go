package sandbox

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/internal/log"
	"github.com/aonesuite/aone/packages/go/sandbox"
	"github.com/subosito/gotenv"
)

// keepalivePingIntervalSec matches the upstream SDK's KEEPALIVE_PING_INTERVAL_SEC (50s).
// This header tells the envd server to send periodic keepalive pings on gRPC streams,
// preventing proxies/load balancers from closing idle connections.
const keepalivePingIntervalSec = "50"

// keepalivePingHeader is the HTTP header name for the keepalive ping interval.
const keepalivePingHeader = "Keepalive-Ping-Interval"

// keepaliveTransport wraps an http.RoundTripper to inject the Keepalive-Ping-Interval header.
type keepaliveTransport struct {
	base http.RoundTripper
}

// RoundTrip adds the keepalive header before delegating the request to the
// wrapped transport.
func (t *keepaliveTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(keepalivePingHeader, keepalivePingIntervalSec)
	return t.base.RoundTrip(req)
}

// loggingTransport wraps an http.RoundTripper to emit structured logs for
// each request. At DEBUG it records method, URL, status, duration, and
// request-id; at TRACE it additionally dumps redacted headers and bodies.
// Deliberately layered outside keepaliveTransport so the keepalive header
// shows up in the logged request — what the server sees is what we log.
type loggingTransport struct {
	base http.RoundTripper
}

// RoundTrip implements http.RoundTripper. Errors from the wrapped
// transport are logged at DEBUG and returned unchanged so callers handle
// them normally.
//
// Streaming and binary content is deliberately NOT buffered: file uploads
// (filesystem.WriteStream), downloads (filesystem.ReadStream), and envd
// connect streams (process/PTY) would otherwise lose their streaming
// semantics under TRACE — full payloads in RAM and no first-byte until
// EOF. shouldDumpBody decides per-request whether the dump is safe.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Short-circuit when nothing would be emitted: avoids reading bodies
	// and capturing headers for the default (no-flag) command path.
	if !log.Enabled(slogDebug) {
		return t.base.RoundTrip(req)
	}

	start := time.Now()
	safeURL := log.RedactURL(req.URL.String())
	traceEnabled := log.Enabled(log.LevelTrace)
	dumpReqBody := traceEnabled && shouldDumpBody(req.Header, req.ContentLength)

	// At TRACE, on payloads we deem safe to buffer, drain req.Body and
	// install a fresh reader before delegating. On read failure we surface
	// a wrapped error rather than letting the round tripper get a closed
	// body — the alternative was a silent corruption bug under TRACE.
	var reqBody []byte
	if dumpReqBody && req.Body != nil && req.Body != http.NoBody {
		buf, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("loggingTransport: read request body for trace dump: %w", err)
		}
		reqBody = buf
		req.Body = io.NopCloser(bytes.NewReader(buf))
	}

	if traceEnabled {
		log.Trace("http request",
			"method", req.Method,
			"url", safeURL,
			"headers", headerMap(log.RedactHeaders(req.Header)),
			"body", bodyForTrace(dumpReqBody, reqBody, req.Header),
		)
	}

	resp, err := t.base.RoundTrip(req)
	dur := time.Since(start)
	if err != nil {
		log.Debug("http request failed",
			"method", req.Method,
			"url", safeURL,
			"duration", dur.String(),
			"error", err.Error(),
		)
		return resp, err
	}

	requestID := resp.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = resp.Header.Get("X-Request-ID")
	}

	log.Debug("http response",
		"method", req.Method,
		"url", safeURL,
		"status", resp.StatusCode,
		"duration", dur.String(),
		"request_id", requestID,
	)

	dumpRespBody := traceEnabled && shouldDumpBody(resp.Header, resp.ContentLength)
	if traceEnabled {
		var respBody []byte
		if dumpRespBody && resp.Body != nil {
			buf, rerr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if rerr != nil {
				// Propagate the read error through the body reader so the
				// caller sees a real failure instead of a silently empty
				// response. We still log the trace line so the failure is
				// visible side-by-side with the request that caused it.
				resp.Body = io.NopCloser(errReader{err: rerr})
				log.Trace("http response body",
					"method", req.Method,
					"url", safeURL,
					"status", resp.StatusCode,
					"headers", headerMap(log.RedactHeaders(resp.Header)),
					"body", "<read error: "+rerr.Error()+">",
				)
				return resp, nil
			}
			respBody = buf
			resp.Body = io.NopCloser(bytes.NewReader(buf))
		}
		log.Trace("http response body",
			"method", req.Method,
			"url", safeURL,
			"status", resp.StatusCode,
			"headers", headerMap(log.RedactHeaders(resp.Header)),
			"body", bodyForTrace(dumpRespBody, respBody, resp.Header),
		)
	}

	return resp, nil
}

// bodyForTrace renders the body section of a TRACE log line. When the
// body was deliberately skipped (streaming/binary), the placeholder names
// the content type and length so the log line still tells the operator
// "we saw a payload, we just didn't capture it."
func bodyForTrace(dumped bool, body []byte, h http.Header) string {
	if dumped {
		return log.RedactBody(body, h.Get("Content-Type"))
	}
	ct := h.Get("Content-Type")
	if ct == "" {
		ct = "unknown"
	}
	length := h.Get("Content-Length")
	if length == "" {
		length = "?"
	}
	return "<skipped streaming/binary body content_type=" + ct + " length=" + length + ">"
}

// shouldDumpBody decides whether a body is safe to buffer for a TRACE
// dump. The rule of thumb: only payloads that are small and look like
// text. Streaming/binary content types short-circuit to false to keep
// uploads, downloads, and envd streams from being silently buffered.
//
// contentLength may be -1 (unknown / chunked); that's a strong signal of
// a stream, so we skip.
func shouldDumpBody(h http.Header, contentLength int64) bool {
	if contentLength < 0 {
		return false
	}
	// Cap at maxBodyDumpBytes — much larger than the per-line truncation
	// because we want to avoid even reading 100MB into memory just to
	// throw most of it away.
	if contentLength > maxBodyDumpBytes {
		return false
	}
	ct := strings.ToLower(h.Get("Content-Type"))
	if ct == "" {
		// Empty Content-Type usually means an empty body (GET, 204…) so
		// dumping is harmless and useful.
		return true
	}
	for _, prefix := range streamingContentTypePrefixes {
		if strings.HasPrefix(ct, prefix) {
			return false
		}
	}
	for _, substr := range streamingContentTypeSubstrings {
		if strings.Contains(ct, substr) {
			return false
		}
	}
	return true
}

// maxBodyDumpBytes caps how large a Content-Length we'll buffer for a
// TRACE dump. 64KB is generous for control-plane JSON responses but small
// enough that buffering one is never a memory problem.
const maxBodyDumpBytes int64 = 64 * 1024

// streamingContentTypePrefixes are Content-Type prefixes we should never
// buffer: binary uploads, server-streaming responses, media.
var streamingContentTypePrefixes = []string{
	"application/octet-stream",
	"application/grpc",
	"application/connect",
	"application/x-ndjson",
	"multipart/",
	"audio/",
	"video/",
	"image/",
}

// streamingContentTypeSubstrings catches odd combinations the prefix list
// misses (e.g. `text/event-stream`, vendor-specific +proto media types).
var streamingContentTypeSubstrings = []string{
	"event-stream",
	"+proto",
}

// errReader returns err on every Read so a TRACE-time body read failure
// propagates to the caller exactly as if the response itself had failed.
type errReader struct{ err error }

// Read implements io.Reader.
func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// slogDebug aliases slog.LevelDebug for the loggingTransport hot path.
const slogDebug = slog.LevelDebug

// headerMap turns http.Header into a flat map[string]string so slog's
// text handler renders it on a single line. Multiple values per key are
// joined with ", "; that matches what the server sees on the wire.
func headerMap(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vs := range h {
		out[k] = strings.Join(vs, ", ")
	}
	return out
}

// loadDotEnv loads variables from the .env file in the current directory.
// Only variables not already set in the OS environment are loaded (OS takes priority).
// Missing or unreadable .env files are silently ignored.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	env, err := gotenv.StrictParse(f)
	if err != nil {
		return
	}
	for key, value := range env {
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, strings.TrimSpace(value))
		}
	}
}

// NewSandboxClient creates a new sandbox client by applying the standard
// credential precedence chain (flag > env > config file > default).
//
// Callers that need to honor CLI flags should use NewSandboxClientWithResolver
// directly; this helper resolves with no flag overrides for the common path.
func NewSandboxClient() (*sandbox.Client, error) {
	return NewSandboxClientWithResolver(config.Resolver{})
}

// NewSandboxClientWithResolver builds a client using the given Resolver,
// surfacing flag-level overrides on top of env + config file layers.
func NewSandboxClientWithResolver(r config.Resolver) (*sandbox.Client, error) {
	// Honor a project-local .env so users don't have to export every time
	// they switch repos. OS env still wins if both are set.
	loadDotEnv()

	resolved, err := r.Resolve()
	if err != nil {
		return nil, err
	}
	if resolved.APIKey == "" {
		return nil, fmt.Errorf("API key not configured: run `aone auth login --api-key <key>` or set %s", config.EnvAPIKey)
	}

	return sandbox.NewClient(&sandbox.Config{
		APIKey:   resolved.APIKey,
		Endpoint: resolved.Endpoint,
		HTTPClient: &http.Client{
			// loggingTransport wraps keepaliveTransport so the keepalive
			// header injected by the latter is included in any TRACE-level
			// header dump — i.e. logs show exactly what the server sees.
			Transport: &loggingTransport{
				base: &keepaliveTransport{base: http.DefaultTransport},
			},
		},
	})
}

// MaskAPIKey returns a redacted form of the API key suitable for display in
// `aone auth info`: keep a 4-char prefix and a 4-char suffix, replace the
// middle with `****`. Short keys collapse to all asterisks.
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", 4) + key[len(key)-4:]
}
