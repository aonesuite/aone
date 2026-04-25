package instance

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/packages/go/sandbox/sandboxtest"
)

// captureStdout redirects os.Stdout while fn runs and returns the captured
// bytes. Tests that exercise commands with the mock server need this because
// the commands print success/info to stdout via fmt.Println / table writers.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String()
}

// sawRequest reports whether the mock server received a request matching the
// given method+path exactly.
func sawRequest(srv *sandboxtest.Server, method, path string) bool {
	for _, req := range srv.Requests() {
		if req.Method == method && req.Path == path {
			return true
		}
	}
	return false
}

// withMock wires the mock control plane into the credential resolver chain so
// any *NewSandboxClient call inside fn targets the mock server.
func withMock(t *testing.T) *sandboxtest.Server {
	t.Helper()
	t.Setenv(config.EnvConfigHome, t.TempDir())
	srv := sandboxtest.NewServer(t)
	t.Setenv(config.EnvAPIKey, "test-key")
	t.Setenv(config.EnvEndpoint, srv.URL())
	return srv
}

func TestInfo_JSONHappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Info(InfoInfo{SandboxID: "sbx-test", Format: "json"})
	})
	// JSON output should contain the sandbox ID and template ID from the
	// default mock GET /sandboxes/{id} response.
	if !strings.Contains(out, "sbx-test") {
		t.Fatalf("stdout missing sandbox id: %q", out)
	}
	if !strings.Contains(out, "tpl-test") {
		t.Fatalf("stdout missing template id: %q", out)
	}
}

func TestInfo_PrettyHappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Info(InfoInfo{SandboxID: "sbx-test"})
	})
	for _, want := range []string{"Sandbox ID", "Template ID", "State"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q: %q", want, out)
		}
	}
}

func TestInfo_ServerError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"code":500,"message":"boom"}`)
	})
	stderr := captureStderr(t, func() {
		// stdout is captured to keep the test output quiet; we do not assert
		// on it because Connect succeeds before GetInfo fails.
		_ = captureStdout(t, func() {
			Info(InfoInfo{SandboxID: "sbx-test"})
		})
	})
	if !strings.Contains(stderr, "get sandbox info failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestList_HappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		List(ListInfo{Format: "json"})
	})
	if !strings.Contains(out, "sbx-test") {
		t.Fatalf("stdout missing sandbox id: %q", out)
	}
}

func TestList_TableEmpty(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/v2/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	out := captureStdout(t, func() {
		List(ListInfo{})
	})
	if !strings.Contains(out, "No sandboxes found") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestKill_SingleID(t *testing.T) {
	srv := withMock(t)
	// Drain stdout so the colorized success line doesn't pollute test output.
	_ = captureStdout(t, func() {
		Kill(KillInfo{SandboxIDs: []string{"sbx-test"}})
	})
	// PrintSuccess uses fatih/color which caches os.Stdout at init, so we
	// verify success via the server-side request log instead of stdout.
	if !sawRequest(srv, "DELETE", "/sandboxes/sbx-test") {
		t.Fatalf("expected DELETE /sandboxes/sbx-test; got %+v", srv.Requests())
	}
}

func TestKill_NoSandboxesNoOp(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Kill(KillInfo{}) // no IDs, no --all
	})
	if !strings.Contains(out, "No sandboxes to kill") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestKill_AllListsThenKills(t *testing.T) {
	srv := withMock(t)
	// Override list to return two sandboxes.
	srv.Handle("GET", "/v2/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"sandboxID":"a","templateID":"t","clientID":"c","envdVersion":"0","cpuCount":1,"memoryMB":1,"diskSizeMB":1,"startedAt":"2025-01-01T00:00:00Z","endAt":"2025-01-01T00:00:00Z","state":"running"},
			{"sandboxID":"b","templateID":"t","clientID":"c","envdVersion":"0","cpuCount":1,"memoryMB":1,"diskSizeMB":1,"startedAt":"2025-01-01T00:00:00Z","endAt":"2025-01-01T00:00:00Z","state":"running"}
		]`)
	})
	_ = captureStdout(t, func() {
		Kill(KillInfo{All: true})
	})
	if !sawRequest(srv, "DELETE", "/sandboxes/a") || !sawRequest(srv, "DELETE", "/sandboxes/b") {
		t.Fatalf("expected DELETE for a and b; got %+v", srv.Requests())
	}
}

func TestPause_SingleID(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Pause(PauseInfo{SandboxIDs: []string{"sbx-test"}})
	})
	if !sawRequest(srv, "POST", "/sandboxes/sbx-test/pause") {
		t.Fatalf("expected POST /sandboxes/sbx-test/pause; got %+v", srv.Requests())
	}
}

func TestPause_PauseFails(t *testing.T) {
	srv := withMock(t)
	srv.Handle("POST", "/sandboxes/{id}/pause", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Pause(PauseInfo{SandboxIDs: []string{"sbx-test"}})
		})
	})
	if !strings.Contains(stderr, "pause sandbox sbx-test failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestResume_SingleID(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Resume(ResumeInfo{SandboxIDs: []string{"sbx-test"}})
	})
	if !sawRequest(srv, "POST", "/sandboxes/sbx-test/resume") {
		t.Fatalf("expected POST /sandboxes/sbx-test/resume; got %+v", srv.Requests())
	}
}

func TestMetrics_JSONOneShot(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Metrics(MetricsInfo{SandboxID: "sbx-test", Format: "json"})
	})
	if !strings.Contains(out, "CPUUsedPct") {
		t.Fatalf("stdout missing metrics fields: %q", out)
	}
}

func TestMetrics_PrettyOneShotEmpty(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/sandboxes/{id}/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	out := captureStdout(t, func() {
		Metrics(MetricsInfo{SandboxID: "sbx-test"})
	})
	if !strings.Contains(out, "No metrics available") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestMetrics_RequiresSandboxID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		Metrics(MetricsInfo{})
	})
	if !strings.Contains(stderr, "sandbox ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}
