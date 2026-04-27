package template

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/packages/go/sandbox/sandboxtest"
)

// captureStdout redirects os.Stdout while fn runs and returns the captured
// bytes for assertion.
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

// captureStderr mirrors captureStdout for stderr, used to assert PrintError
// output without redirecting test runner diagnostics.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	os.Stderr = orig
	return buf.String()
}

// withMock wires the credential resolver chain at a fresh mock control plane.
func withMock(t *testing.T) *sandboxtest.Server {
	t.Helper()
	t.Setenv(config.EnvConfigHome, t.TempDir())
	srv := sandboxtest.NewServer(t)
	t.Setenv(config.EnvAPIKey, "test-key")
	t.Setenv(config.EnvEndpoint, srv.URL())
	return srv
}

// sawRequest reports whether any recorded request matches method+path exactly.
func sawRequest(srv *sandboxtest.Server, method, path string) bool {
	for _, req := range srv.Requests() {
		if req.Method == method && req.Path == path {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func TestList_HappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		List(ListInfo{Format: "json"})
	})
	if !strings.Contains(out, "tpl-test") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestList_TableEmpty(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/templates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	out := captureStdout(t, func() {
		List(ListInfo{})
	})
	if !strings.Contains(out, "No templates found") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestList_ServerError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/templates", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() { List(ListInfo{}) })
	})
	if !strings.Contains(stderr, "list templates failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// get
// ---------------------------------------------------------------------------

func TestGet_RequiresTemplateID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		Get(GetInfo{})
	})
	if !strings.Contains(stderr, "template ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestGet_HappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Get(GetInfo{TemplateID: "tpl-test"})
	})
	if !strings.Contains(out, "Template ID:") || !strings.Contains(out, "tpl-test") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestGet_ServerError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() { Get(GetInfo{TemplateID: "missing"}) })
	})
	if !strings.Contains(stderr, "get template failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

func TestDelete_NoIDsRequiresFlag(t *testing.T) {
	withMock(t)
	// Empty Path so no project config is found, no implicit fallback.
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Delete(DeleteInfo{Yes: true, Path: t.TempDir()})
		})
	})
	if !strings.Contains(stderr, "at least one template ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestDelete_YesFlagSkipsPrompt(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Delete(DeleteInfo{TemplateIDs: []string{"a", "b"}, Yes: true, Path: t.TempDir()})
	})
	if !sawRequest(srv, "DELETE", "/templates/a") || !sawRequest(srv, "DELETE", "/templates/b") {
		t.Fatalf("expected DELETE for a and b; got %+v", srv.Requests())
	}
}

func TestDelete_ServerErrorContinues(t *testing.T) {
	srv := withMock(t)
	srv.Handle("DELETE", "/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Delete(DeleteInfo{TemplateIDs: []string{"x"}, Yes: true, Path: t.TempDir()})
		})
	})
	if !strings.Contains(stderr, "delete template x failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// publish
// ---------------------------------------------------------------------------

func TestPublish_HappyPath(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Publish(PublishInfo{TemplateIDs: []string{"tpl-1"}, Yes: true, Public: true, Path: t.TempDir()})
	})
	if !sawRequest(srv, "PATCH", "/templates/tpl-1") {
		t.Fatalf("expected PATCH /templates/tpl-1; got %+v", srv.Requests())
	}
}

func TestPublish_UnpublishUsesSameRoute(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Publish(PublishInfo{TemplateIDs: []string{"tpl-1"}, Yes: true, Public: false, Path: t.TempDir()})
	})
	// We confirm the route was hit; the body distinguishes publish vs
	// unpublish but we don't decode it here.
	if !sawRequest(srv, "PATCH", "/templates/tpl-1") {
		t.Fatalf("expected PATCH /templates/tpl-1; got %+v", srv.Requests())
	}
}

func TestPublish_NoIDsRequiresFlag(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Publish(PublishInfo{Yes: true, Public: true, Path: t.TempDir()})
		})
	})
	if !strings.Contains(stderr, "at least one template ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestPublish_ServerErrorContinues(t *testing.T) {
	srv := withMock(t)
	srv.Handle("PATCH", "/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Publish(PublishInfo{TemplateIDs: []string{"tpl-x"}, Yes: true, Public: true, Path: t.TempDir()})
		})
	})
	if !strings.Contains(stderr, "publish template tpl-x failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// create
// ---------------------------------------------------------------------------

func TestCreate_RequiresName(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		Create(BuildInfo{})
	})
	if !strings.Contains(stderr, "template name is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreate_AutoDetectsDockerfile(t *testing.T) {
	srv := withMock(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine:3.20\nRUN echo hi\n"), 0o644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}

	_ = captureStdout(t, func() {
		Create(BuildInfo{Name: "demo", Path: dir})
	})
	if !sawRequest(srv, "POST", "/v3/templates") {
		t.Fatalf("expected POST /v3/templates; got %+v", srv.Requests())
	}
	if !sawRequest(srv, "POST", "/v2/templates/tpl-new/builds/11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected start build call; got %+v", srv.Requests())
	}
}
