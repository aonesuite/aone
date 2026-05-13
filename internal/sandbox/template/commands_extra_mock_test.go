package template

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuilds_RequiresTemplateID covers the early validation in Builds when
// neither template ID nor build ID is provided.
func TestBuilds_RequiresTemplateID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		Builds(BuildsInfo{})
	})
	if !strings.Contains(stderr, "template ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestBuilds_RequiresBuildID covers the second-level validation: template ID
// is present but build ID is empty.
func TestBuilds_RequiresBuildID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		Builds(BuildsInfo{TemplateID: "tpl-1"})
	})
	if !strings.Contains(stderr, "build ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestBuilds_HappyPath drives the success path: GET status returns the
// default mock payload and the function prints template id + status.
func TestBuilds_HappyPath(t *testing.T) {
	srv := withMock(t)
	out := captureStdout(t, func() {
		Builds(BuildsInfo{TemplateID: "tpl-1", BuildID: "build-1"})
	})
	if !strings.Contains(out, "Status:") {
		t.Fatalf("stdout = %q", out)
	}
	if !sawRequest(srv, "GET", "/api/v1/sbx/templates/tpl-1/builds/build-1/status") {
		t.Fatalf("status route not hit; got %+v", srv.Requests())
	}
}

// TestBuilds_ServerError covers the error branch when the API returns 500.
func TestBuilds_ServerError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/api/v1/sbx/templates/{tid}/builds/{bid}/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Builds(BuildsInfo{TemplateID: "tpl-1", BuildID: "b"})
		})
	})
	if !strings.Contains(stderr, "get build status failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestInit_NonInteractiveRequiresFlags exercises the early validation that
// rejects missing flags when stdin isn't a TTY (always true in tests).
func TestInit_NonInteractiveRequiresFlags(t *testing.T) {
	stderr := captureStderr(t, func() {
		Init(InitInfo{})
	})
	if !strings.Contains(stderr, "--name and --language are required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestInit_InvalidName covers the regex validation for template names.
func TestInit_InvalidName(t *testing.T) {
	stderr := captureStderr(t, func() {
		Init(InitInfo{Name: "Bad Name!", Language: "go"})
	})
	if !strings.Contains(stderr, "invalid template name") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestInit_UnsupportedLanguage covers the language allow-list check.
func TestInit_UnsupportedLanguage(t *testing.T) {
	stderr := captureStderr(t, func() {
		Init(InitInfo{Name: "demo", Language: "rust"})
	})
	if !strings.Contains(stderr, "unsupported language") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestInit_ScaffoldsGo runs Init with a valid go scaffold target. We don't
// inspect every generated file — just confirm the directory and a couple of
// well-known outputs exist after the call.
func TestInit_ScaffoldsGo(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "demo")
	_ = captureStdout(t, func() {
		Init(InitInfo{Name: "demo", Language: "go", Path: root})
	})
	for _, name := range []string{"main.go", "go.mod", "Dockerfile", "aone.sandbox.toml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s in scaffold output: %v", name, err)
		}
	}
}

// TestInit_PathActsAsRootDir verifies --path points at the parent/root
// directory, and the template name becomes a child folder.
func TestInit_PathActsAsRootDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "demo")

	_ = captureStdout(t, func() {
		Init(InitInfo{Name: "demo", Language: "go", Path: root})
	})

	for _, name := range []string{"main.go", "go.mod", "Dockerfile", "aone.sandbox.toml"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Errorf("expected %s in scaffold output under named child dir: %v", name, err)
		}
	}
}

func TestInit_AcceptsPythonSyncAlias(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "demo")

	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Init(InitInfo{Name: "demo", Language: "python-sync", Path: root})
		})
	})
	if strings.Contains(stderr, "unsupported language") {
		t.Fatalf("stderr = %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(target, "template.py")); err != nil {
		t.Fatalf("expected python scaffold under alias language: %v", err)
	}
}

func TestBuild_FromDockerfile_CopyContentSentToCreateTemplate(t *testing.T) {
	srv := withMock(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"),
		[]byte("FROM alpine:3.20\nCOPY . /app\n"), 0o644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	_ = captureStdout(t, func() {
		Build(BuildInfo{
			Name:       "demo",
			Dockerfile: filepath.Join(dir, "Dockerfile"),
			Path:       dir,
		})
	})
	reqs := srv.RequestsFor("POST", "/api/v1/sbx/templates")
	if len(reqs) != 1 {
		t.Fatalf("expected create-template request; got %+v", srv.Requests())
	}
	if !strings.Contains(reqs[0].Body, "COPY . /app") {
		t.Fatalf("expected Dockerfile COPY content in request body: %q", reqs[0].Body)
	}
}

func TestGet_RendersTemplateDetail(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/api/v1/sbx/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"template_id":"tpl-x",
			"build_id":"33333333-3333-3333-3333-333333333333",
			"build_status":"ready",
			"cpu_count":2,"memory_mb":1024,"disk_size_mb":8192,
			"envd_version":"0.0.1","public":false,"source":"user",
			"editable":true,"deletable":true,
			"aliases":[],"names":["x"],
			"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"
		}`))
	})
	out := captureStdout(t, func() {
		Get(GetInfo{TemplateID: "tpl-x"})
	})
	for _, want := range []string{"Build ID:", "33333333", "Names:", "Source:", "Editable:", "Deletable:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q: %q", want, out)
		}
	}
}
