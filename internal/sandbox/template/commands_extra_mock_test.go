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
	if !sawRequest(srv, "GET", "/templates/tpl-1/builds/build-1/status") {
		t.Fatalf("status route not hit; got %+v", srv.Requests())
	}
}

// TestBuilds_ServerError covers the error branch when the API returns 500.
func TestBuilds_ServerError(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/templates/{tid}/builds/{bid}/status", func(w http.ResponseWriter, r *http.Request) {
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

// TestInit_PathActsAsRootDir aligns with e2b's init semantics: --path points
// at the parent/root directory, and the template name becomes a child folder.
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

// TestBuild_FromDockerfile_CacheHit drives the COPY path where
// GetTemplateFiles reports the archive as already present (default mock).
// The build still proceeds to /v2/templates/.../builds/... but no upload
// happens.
func TestBuild_FromDockerfile_CacheHit(t *testing.T) {
	srv := withMock(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"),
		[]byte("FROM alpine:3.20\nCOPY . /app\n"), 0o644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}
	// A real source file so ComputeFilesHash has content to hash.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	out := captureStdout(t, func() {
		Build(BuildInfo{
			Name:       "demo",
			Dockerfile: filepath.Join(dir, "Dockerfile"),
			Path:       dir,
		})
	})
	if !strings.Contains(out, "already uploaded (cached)") {
		t.Fatalf("expected cache-hit message; out = %q", out)
	}
	if !sawRequest(srv, "POST", "/v2/templates/tpl-new/builds/11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected start build call; got %+v", srv.Requests())
	}
}

// TestBuild_FromDockerfile_UploadsOnCacheMiss switches the files endpoint to
// return present=false and a fake URL pointing back at the mock; the build
// should then PUT to that URL via CollectAndUpload.
func TestBuild_FromDockerfile_UploadsOnCacheMiss(t *testing.T) {
	srv := withMock(t)

	// Stand up an "upload" endpoint on the same mock server. CollectAndUpload
	// PUTs the gzipped tar there; we just need a 200 response.
	uploadHits := 0
	srv.Handle("PUT", "/upload-target", func(w http.ResponseWriter, r *http.Request) {
		uploadHits++
		w.WriteHeader(http.StatusOK)
	})
	uploadURL := srv.URL() + "/upload-target"
	srv.Handle("GET", "/templates/{tid}/files/{hash}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"present":false,"url":"` + uploadURL + `"}`))
	})

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"),
		[]byte("FROM alpine:3.20\nCOPY . /app\n"), 0o644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	out := captureStdout(t, func() {
		Build(BuildInfo{
			Name:       "demo",
			Dockerfile: filepath.Join(dir, "Dockerfile"),
			Path:       dir,
		})
	})
	if !strings.Contains(out, "Uploading files for COPY") {
		t.Fatalf("expected upload message; out = %q", out)
	}
	if uploadHits == 0 {
		t.Fatalf("upload PUT never happened; requests = %+v", srv.Requests())
	}
}

// TestBuild_FromDockerfile_UploadFailureSurfaces verifies that a non-2xx
// response from the upload URL bubbles back as an error from Build.
func TestBuild_FromDockerfile_UploadFailureSurfaces(t *testing.T) {
	srv := withMock(t)
	srv.Handle("PUT", "/upload-target", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	uploadURL := srv.URL() + "/upload-target"
	srv.Handle("GET", "/templates/{tid}/files/{hash}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"present":false,"url":"` + uploadURL + `"}`))
	})

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"),
		[]byte("FROM alpine:3.20\nCOPY . /app\n"), 0o644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Build(BuildInfo{
				Name:       "demo",
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Path:       dir,
			})
		})
	})
	if !strings.Contains(stderr, "upload files") && !strings.Contains(stderr, "build from Dockerfile") {
		t.Fatalf("stderr should mention upload failure; got %q", stderr)
	}
}

// TestGet_RendersBuildsTable exercises the optional builds-rendering branch
// in Get by overriding the template GET to include a builds slice.
func TestGet_RendersBuildsTable(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-x",
			"buildID":"build-1","buildStatus":"ready","buildCount":1,
			"cpuCount":2,"memoryMB":1024,"diskSizeMB":8192,
			"envdVersion":"0.0.1","public":false,"spawnCount":0,
			"aliases":[],"names":["x"],
			"createdAt":"2025-01-01T00:00:00Z","updatedAt":"2025-01-01T00:00:00Z",
			"builds":[{
				"buildID":"33333333-3333-3333-3333-333333333333","cpuCount":2,"memoryMB":1024,"status":"ready",
				"createdAt":"2025-01-01T00:00:00Z","updatedAt":"2025-01-01T00:00:00Z"
			}]
		}`))
	})
	out := captureStdout(t, func() {
		Get(GetInfo{TemplateID: "tpl-x"})
	})
	if !strings.Contains(out, "BUILD ID") || !strings.Contains(out, "33333333") {
		t.Fatalf("stdout missing builds table: %q", out)
	}
}
