package instance

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/config"
)

// captureStderr redirects os.Stderr while fn runs, returning what was written.
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

// isolateConfig points the user-config home and credential env vars at a temp
// dir, ensuring the test cannot read or pollute the real ~/.config/aone.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigHome, dir)
	t.Setenv(config.EnvAPIKey, "")
	t.Setenv(config.EnvEndpoint, "")
	return dir
}

func TestCreate_NoTemplateAndNoProjectConfigFallsBackToBase(t *testing.T) {
	isolateConfig(t)
	projectDir := t.TempDir() // empty: no aone.sandbox.toml

	out := captureStderr(t, func() {
		Create(CreateInfo{Path: projectDir})
	})

	if strings.Contains(out, "template ID is required") {
		t.Fatalf("stderr = %q, should fall back to base template instead of failing missing-template validation", out)
	}
	if !strings.Contains(out, "API key not configured") {
		t.Fatalf("expected API-key error after base fallback; stderr = %q", out)
	}
}

func TestCreate_FallsBackToProjectConfigTemplateID(t *testing.T) {
	// With no API key set, the fallback path should resolve TemplateID from
	// aone.sandbox.toml and then fail with the API-key error rather than the
	// missing-template-id error.
	isolateConfig(t)
	projectDir := t.TempDir()
	cfg := filepath.Join(projectDir, config.ProjectFileName)
	if err := os.WriteFile(cfg, []byte(`template_id = "tpl-from-config"`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	out := captureStderr(t, func() {
		Create(CreateInfo{Path: projectDir})
	})

	if strings.Contains(out, "template ID is required") {
		t.Fatalf("project-config fallback failed; stderr = %q", out)
	}
	if !strings.Contains(out, "API key not configured") {
		t.Fatalf("expected API-key error after fallback; stderr = %q", out)
	}
}

func TestCreate_ExplicitTemplateBeatsProjectConfig(t *testing.T) {
	isolateConfig(t)
	projectDir := t.TempDir()
	cfg := filepath.Join(projectDir, config.ProjectFileName)
	if err := os.WriteFile(cfg, []byte(`template_id = "tpl-from-config"`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Explicit TemplateID prevents the LoadProject fallback from being needed
	// — without an API key we still see the API-key error, never the
	// missing-template-id one.
	out := captureStderr(t, func() {
		Create(CreateInfo{TemplateID: "tpl-explicit", Path: projectDir})
	})

	if strings.Contains(out, "template ID is required") {
		t.Fatalf("unexpected missing-template error: %q", out)
	}
	if !strings.Contains(out, "API key not configured") {
		t.Fatalf("expected API-key error; stderr = %q", out)
	}
}

func TestInfo_EmptySandboxIDErrors(t *testing.T) {
	isolateConfig(t)

	out := captureStderr(t, func() {
		Info(InfoInfo{})
	})

	if !strings.Contains(out, "sandbox ID is required") {
		t.Fatalf("stderr = %q", out)
	}
}
