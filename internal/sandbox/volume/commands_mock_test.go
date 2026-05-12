package volume

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
// bytes. Used because volume commands print user-facing details to stdout.
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

// withMock installs a fresh mock control plane and points the credential
// resolver chain at it.
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

func TestCreate_HappyPath(t *testing.T) {
	srv := withMock(t)
	out := captureStdout(t, func() {
		Create(CreateInfo{Name: "test-volume"})
	})
	if !strings.Contains(out, "vol-test") || !strings.Contains(out, "test-volume") {
		t.Fatalf("stdout = %q", out)
	}
	if !sawRequest(srv, "POST", "/api/v1/sbx/volumes") {
		t.Fatalf("expected POST /volumes; got %+v", srv.Requests())
	}
}

func TestCreate_JSONOutput(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Create(CreateInfo{Name: "vname", Format: "json"})
	})
	if !strings.Contains(out, `"volumeID"`) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestCreate_APIError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("POST", "/api/v1/sbx/volumes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"code":500,"message":"boom"}`)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() { Create(CreateInfo{Name: "v"}) })
	})
	if !strings.Contains(stderr, "create volume failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestList_HappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		List(ListInfo{Format: "json"})
	})
	if !strings.Contains(out, "vol-test") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestList_EmptyTable(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/api/v1/sbx/volumes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	out := captureStdout(t, func() { List(ListInfo{}) })
	if !strings.Contains(out, "No volumes found") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestInfo_HappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Info(InfoInfo{VolumeID: "vol-test"})
	})
	if !strings.Contains(out, "vol-test") || !strings.Contains(out, "test-volume") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestInfo_JSON(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Info(InfoInfo{VolumeID: "vol-test", Format: "json"})
	})
	if !strings.Contains(out, `"volumeID"`) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestLs_DefaultPath(t *testing.T) {
	srv := withMock(t)
	out := captureStdout(t, func() {
		Ls(LsInfo{VolumeID: "vol-test"})
	})
	// Default path "/" should produce a single mock entry whose path matches.
	// We only assert headers because the default mock entry has empty name.
	if !strings.Contains(out, "TYPE") {
		t.Fatalf("stdout = %q", out)
	}
	if !sawRequest(srv, "GET", "/volumecontent/vol-test/dir") {
		t.Fatalf("expected GET /volumecontent/vol-test/dir; got %+v", srv.Requests())
	}
}

func TestLs_JSONEmpty(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/volumecontent/{id}/dir", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	out := captureStdout(t, func() {
		Ls(LsInfo{VolumeID: "vol-test", Format: "json"})
	})
	if !strings.Contains(out, "[]") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestMkdir_HappyPath(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Mkdir(MkdirInfo{VolumeID: "vol-test", Path: "/work/new"})
	})
	if !sawRequest(srv, "POST", "/volumecontent/vol-test/dir") {
		t.Fatalf("expected POST /volumecontent/vol-test/dir; got %+v", srv.Requests())
	}
}

func TestRm_HappyPath(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Rm(RmInfo{VolumeID: "vol-test", Path: "/work/x"})
	})
	if !sawRequest(srv, "DELETE", "/volumecontent/vol-test/path") {
		t.Fatalf("expected DELETE /volumecontent/vol-test/path; got %+v", srv.Requests())
	}
}

func TestCat_HappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Cat(CatInfo{VolumeID: "vol-test", Path: "/etc/hosts"})
	})
	// Default mock returns "file-content" body.
	if !strings.Contains(out, "file-content") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestCat_ReadError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/volumecontent/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"code":404,"message":"missing"}`)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Cat(CatInfo{VolumeID: "vol-test", Path: "/missing"})
		})
	})
	if !strings.Contains(stderr, "read file failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestDelete_YesFlagSkipsPrompt(t *testing.T) {
	srv := withMock(t)
	_ = captureStdout(t, func() {
		Delete(DeleteInfo{VolumeIDs: []string{"vol-1", "vol-2"}, Yes: true})
	})
	if !sawRequest(srv, "DELETE", "/api/v1/sbx/volumes/vol-1") || !sawRequest(srv, "DELETE", "/api/v1/sbx/volumes/vol-2") {
		t.Fatalf("expected DELETE for both volumes; got %+v", srv.Requests())
	}
}

func TestDelete_NotFoundWarn(t *testing.T) {
	srv := withMock(t)
	srv.Handle("DELETE", "/api/v1/sbx/volumes/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Delete(DeleteInfo{VolumeIDs: []string{"vol-missing"}, Yes: true})
		})
	})
	if !strings.Contains(stderr, "vol-missing not found") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestDelete_NoIDsRequiresFlag(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() { Delete(DeleteInfo{Yes: true}) })
	})
	if !strings.Contains(stderr, "at least one volume ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}
