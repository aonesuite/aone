// Package sandboxtest provides an httptest-based mock of the aone sandbox
// control plane. It is intended for use by CLI and library tests that exercise
// command paths invoking the SDK without contacting a real server.
//
// The Server starts an httptest.Server with default handlers covering the
// sandbox, volume, and template control-plane endpoints. Tests may override
// any route via Handle, replace the default response payload via setter
// helpers, or inspect captured requests for assertions.
//
// The default routes return minimally-valid JSON so that the SDK's response
// decoders succeed; tests that need richer payloads should override per-route.
package sandboxtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Server is a mock control-plane server backed by httptest.Server. It exposes
// the underlying URL via URL() and a registry of per-route handlers callers can
// override with Handle.
type Server struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	routes   map[string]http.HandlerFunc // key: "METHOD path-pattern"
	requests []RecordedRequest           // append-only request log
}

// RecordedRequest captures an inbound request for assertion in tests. The body
// is read fully and stored as a string; binary bodies are still safe to inspect
// via the Body field (Go strings are byte sequences).
type RecordedRequest struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// NewServer starts a mock control-plane server. The server is closed via
// t.Cleanup so callers do not need to defer Close.
func NewServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{t: t, routes: map[string]http.HandlerFunc{}}
	s.installDefaults()
	s.srv = httptest.NewServer(http.HandlerFunc(s.dispatch))
	t.Cleanup(s.srv.Close)
	return s
}

// URL returns the base URL of the mock server. Callers should pass this URL
// via t.Setenv(config.EnvEndpoint, srv.URL()) so the SDK targets it.
func (s *Server) URL() string { return s.srv.URL }

// Close terminates the server. Calling Close is optional because NewServer
// registers t.Cleanup; this is exposed for tests that want explicit control.
func (s *Server) Close() { s.srv.Close() }

// Handle registers (or replaces) the handler for "method pattern". The pattern
// uses the same {id}-style placeholders as the Match helper. Patterns are
// matched in registration order during dispatch.
func (s *Server) Handle(method, pattern string, h http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[routeKey(method, pattern)] = h
}

// Requests returns a copy of the recorded request log.
func (s *Server) Requests() []RecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecordedRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

// dispatch is the root handler. It records the request, then matches against
// registered routes and invokes the first match. A 404 with a JSON body is
// returned when nothing matches so the SDK surfaces a usable error.
func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	s.record(r)

	s.mu.Lock()
	// Snapshot the route table so per-request mutations from handlers don't
	// race with subsequent dispatches.
	keys := make([]string, 0, len(s.routes))
	for k := range s.routes {
		keys = append(keys, k)
	}
	routes := s.routes
	s.mu.Unlock()

	for _, k := range keys {
		method, pattern, ok := splitRouteKey(k)
		if !ok || method != r.Method {
			continue
		}
		if matchPath(pattern, r.URL.Path) {
			routes[k](w, r)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    404,
		"message": fmt.Sprintf("sandboxtest: no handler for %s %s", r.Method, r.URL.Path),
	})
}

// record stores a copy of the request for later inspection by tests.
func (s *Server) record(r *http.Request) {
	rec := RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Header: r.Header.Clone(),
	}
	if r.Body != nil {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		rec.Body = sb.String()
	}
	s.mu.Lock()
	s.requests = append(s.requests, rec)
	s.mu.Unlock()
}

// installDefaults wires the minimal set of routes the SDK touches in standard
// CLI command paths. Each handler returns just enough JSON to satisfy the
// generated response decoders. Per-test overrides can still replace any route.
func (s *Server) installDefaults() {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// --- sandboxes ---------------------------------------------------------
	s.routes[routeKey("POST", "/sandboxes")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{
			"sandboxID":          "sbx-test",
			"templateID":         "tpl-test",
			"clientID":           "client-test",
			"envdVersion":        "0.0.1",
			"domain":             "example.test",
			"trafficAccessToken": "traf-tok",
			"envdAccessToken":    "envd-tok",
		})
	}

	s.routes[routeKey("POST", "/sandboxes/{id}/connect")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"sandboxID":          pathID(r.URL.Path, "/sandboxes/", "/connect"),
			"templateID":         "tpl-test",
			"clientID":           "client-test",
			"envdVersion":        "0.0.1",
			"domain":             "example.test",
			"trafficAccessToken": "traf-tok",
			"envdAccessToken":    "envd-tok",
		})
	}

	sandboxDetail := func(id string) map[string]any {
		return map[string]any{
			"sandboxID":           id,
			"templateID":          "tpl-test",
			"clientID":            "client-test",
			"envdVersion":         "0.0.1",
			"envdAccessToken":     "envd-tok",
			"domain":              "example.test",
			"cpuCount":            int32(2),
			"memoryMB":            int32(1024),
			"diskSizeMB":          int32(8192),
			"startedAt":           now,
			"endAt":               now,
			"state":               "running",
			"allowInternetAccess": true,
		}
	}

	s.routes[routeKey("GET", "/sandboxes/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r.URL.Path, "/sandboxes/", "")
		writeJSON(w, http.StatusOK, sandboxDetail(id))
	}

	s.routes[routeKey("GET", "/v2/sandboxes")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{{
			"sandboxID":   "sbx-test",
			"templateID":  "tpl-test",
			"clientID":    "client-test",
			"envdVersion": "0.0.1",
			"cpuCount":    int32(2),
			"memoryMB":    int32(1024),
			"diskSizeMB":  int32(8192),
			"startedAt":   now,
			"endAt":       now,
			"state":       "running",
		}})
	}

	s.routes[routeKey("DELETE", "/sandboxes/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	s.routes[routeKey("POST", "/sandboxes/{id}/pause")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	s.routes[routeKey("POST", "/sandboxes/{id}/resume")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}

	s.routes[routeKey("POST", "/sandboxes/{id}/timeout")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	s.routes[routeKey("POST", "/sandboxes/{id}/refreshes")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	s.routes[routeKey("GET", "/sandboxes/{id}/metrics")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{{
			"timestamp":     now,
			"timestampUnix": time.Now().Unix(),
			"cpuCount":      int32(2),
			"cpuUsedPct":    float32(12.5),
			"memTotal":      int64(1024 * 1024 * 1024),
			"memUsed":       int64(256 * 1024 * 1024),
			"diskTotal":     int64(8 * 1024 * 1024 * 1024),
			"diskUsed":      int64(1024 * 1024 * 1024),
		}})
	}

	s.routes[routeKey("GET", "/sandboxes/{id}/logs")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"logs":       []map[string]any{{"line": "hello", "timestamp": now}},
			"logEntries": []map[string]any{{"level": "info", "message": "hello", "fields": map[string]string{}, "timestamp": now}},
		})
	}

	s.routes[routeKey("GET", "/sandboxes/metrics")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"sandboxes": []any{}})
	}

	// --- volumes -----------------------------------------------------------
	s.routes[routeKey("POST", "/volumes")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{
			"volumeID": "vol-test",
			"name":     "test-volume",
			"token":    "vol-tok",
		})
	}

	s.routes[routeKey("GET", "/volumes")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{{
			"volumeID": "vol-test",
			"name":     "test-volume",
			"token":    "vol-tok",
		}})
	}

	s.routes[routeKey("GET", "/volumes/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"volumeID": pathID(r.URL.Path, "/volumes/", ""),
			"name":     "test-volume",
			"token":    "vol-tok",
		})
	}

	s.routes[routeKey("DELETE", "/volumes/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	// volume content endpoints (envd-style; same server stands in for both)
	volEntry := func(path string) map[string]any {
		return map[string]any{
			"name":  pathBase(path),
			"path":  path,
			"type":  "file",
			"size":  int64(0),
			"mode":  uint32(0o644),
			"uid":   uint32(1000),
			"gid":   uint32(1000),
			"atime": now,
			"mtime": now,
			"ctime": now,
		}
	}

	s.routes[routeKey("GET", "/volumecontent/{id}/dir")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{volEntry(r.URL.Query().Get("path"))})
	}
	s.routes[routeKey("POST", "/volumecontent/{id}/dir")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, volEntry(r.URL.Query().Get("path")))
	}
	s.routes[routeKey("GET", "/volumecontent/{id}/path")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, volEntry(r.URL.Query().Get("path")))
	}
	s.routes[routeKey("PATCH", "/volumecontent/{id}/path")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, volEntry(r.URL.Query().Get("path")))
	}
	s.routes[routeKey("DELETE", "/volumecontent/{id}/path")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	s.routes[routeKey("GET", "/volumecontent/{id}/file")] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file-content"))
	}
	s.routes[routeKey("PUT", "/volumecontent/{id}/file")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, volEntry(r.URL.Query().Get("path")))
	}

	// --- templates ---------------------------------------------------------
	s.routes[routeKey("GET", "/templates")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{{
			"templateID":  "tpl-test",
			"buildID":     "build-1",
			"buildStatus": "ready",
			"buildCount":  int32(1),
			"cpuCount":    int32(2),
			"memoryMB":    int32(1024),
			"diskSizeMB":  int32(8192),
			"envdVersion": "0.0.1",
			"public":      false,
			"spawnCount":  int64(0),
			"aliases":     []string{},
			"names":       []string{"test"},
			"createdAt":   now,
			"updatedAt":   now,
		}})
	}
	s.routes[routeKey("POST", "/v3/templates")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"templateID": "tpl-new",
			"buildID":    "11111111-1111-1111-1111-111111111111",
			"aliases":    []string{},
		})
	}
	s.routes[routeKey("GET", "/templates/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"templateID":  pathID(r.URL.Path, "/templates/", ""),
			"buildID":     "build-1",
			"buildStatus": "ready",
			"buildCount":  int32(1),
			"cpuCount":    int32(2),
			"memoryMB":    int32(1024),
			"diskSizeMB":  int32(8192),
			"envdVersion": "0.0.1",
			"public":      false,
			"spawnCount":  int64(0),
			"aliases":     []string{},
			"names":       []string{"test"},
			"createdAt":   now,
			"updatedAt":   now,
			"builds":      []any{},
		})
	}
	s.routes[routeKey("DELETE", "/templates/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	s.routes[routeKey("PATCH", "/templates/{id}")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	s.routes[routeKey("GET", "/templates/aliases/{alias}")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"templateID": "tpl-test",
			"public":     false,
		})
	}
	s.routes[routeKey("GET", "/templates/{id}/tags")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	}
	s.routes[routeKey("POST", "/templates/tags")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"tags": []any{}})
	}
	s.routes[routeKey("DELETE", "/templates/tags")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	s.routes[routeKey("POST", "/v2/templates/{tid}/builds/{bid}")] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}
	s.routes[routeKey("GET", "/templates/{tid}/builds/{bid}/status")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"templateID": "tpl-test",
			"buildID":    "build-1",
			"status":     "ready",
			"logs":       []string{},
			"logEntries": []any{},
		})
	}
	s.routes[routeKey("GET", "/templates/{tid}/builds/{bid}/logs")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"logs": []any{}})
	}
	s.routes[routeKey("GET", "/templates/{tid}/files/{hash}")] = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{
			"present": true,
		})
	}
}

// --- helpers --------------------------------------------------------------

// writeJSON writes status + a JSON body with Content-Type set.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// routeKey encodes a method+pattern pair into a single map key.
func routeKey(method, pattern string) string {
	return strings.ToUpper(method) + " " + pattern
}

// splitRouteKey splits a route key back into its method and pattern halves.
func splitRouteKey(key string) (method, pattern string, ok bool) {
	idx := strings.IndexByte(key, ' ')
	if idx <= 0 {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}

// matchPath compares a path against a {placeholder} pattern. Each segment must
// match exactly unless the pattern segment is wrapped in braces.
func matchPath(pattern, path string) bool {
	pSegs := splitSegments(pattern)
	xSegs := splitSegments(path)
	if len(pSegs) != len(xSegs) {
		return false
	}
	for i, p := range pSegs {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			continue
		}
		if p != xSegs[i] {
			return false
		}
	}
	return true
}

// splitSegments splits a path on '/' and drops empty leading/trailing slots.
func splitSegments(p string) []string {
	out := strings.Split(p, "/")
	if len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// pathID extracts the ID slug from a path of the form prefix + id + suffix.
// It returns the slug between the two anchors, or empty when not found.
func pathID(path, prefix, suffix string) string {
	rest := strings.TrimPrefix(path, prefix)
	if suffix == "" {
		return rest
	}
	before, _, ok := strings.Cut(rest, suffix)
	if !ok {
		return rest
	}
	return before
}

// pathBase returns the last '/' separated segment.
func pathBase(p string) string {
	if p == "" {
		return ""
	}
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
