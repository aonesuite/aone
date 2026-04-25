package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestHome_RespectsEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	got, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if got != dir {
		t.Fatalf("Home() = %q, want %q", got, dir)
	}
}

func TestHome_FallsBackToUserHome(t *testing.T) {
	t.Setenv(EnvConfigHome, "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir unavailable: %v", err)
	}
	want := filepath.Join(home, ".config", "aone")

	got, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestPath_JoinsHomeAndFileName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join(dir, configFileName)
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	f, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f == nil {
		t.Fatalf("Load returned nil File on missing config")
	}
	if f.APIKey != "" || f.Endpoint != "" || f.LastLoginAt != nil {
		t.Fatalf("expected zero-value File, got %+v", f)
	}
}

func TestLoad_EmptyFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	if err := os.WriteFile(filepath.Join(dir, configFileName), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	f, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f.APIKey != "" || f.Endpoint != "" {
		t.Fatalf("expected empty File, got %+v", f)
	}
}

func TestLoad_InvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(); err == nil {
		t.Fatalf("expected parse error for invalid JSON")
	}
}

func TestSave_WritesAtomicallyWith0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	in := &File{
		Endpoint:    "https://example.test",
		APIKey:      "sk-secret",
		LastLoginAt: &now,
	}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(dir, configFileName)

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("perm = %o, want 0600", perm)
		}
	}

	// Roundtrip via Load.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if got.APIKey != in.APIKey || got.Endpoint != in.Endpoint {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", got, in)
	}
	if got.LastLoginAt == nil || !got.LastLoginAt.Equal(now) {
		t.Fatalf("LastLoginAt mismatch: got %v want %v", got.LastLoginAt, now)
	}

	// File should be valid JSON with a trailing newline.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("expected trailing newline; got %q", raw)
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}
}

func TestSave_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	if err := Save(&File{APIKey: "k"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == configFileName {
			continue
		}
		t.Fatalf("unexpected leftover entry %q", e.Name())
	}
}

func TestUpdate_AppliesMutationAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	if err := Save(&File{APIKey: "old", Endpoint: "https://a"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	err := Update(func(f *File) error {
		f.APIKey = "new"
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != "new" {
		t.Fatalf("APIKey = %q, want %q", got.APIKey, "new")
	}
	if got.Endpoint != "https://a" {
		t.Fatalf("Endpoint = %q, want preserved value %q", got.Endpoint, "https://a")
	}
}

func TestUpdate_MutatorErrorAbortsWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	if err := Save(&File{APIKey: "keep"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	wantErr := os.ErrPermission // any sentinel is fine
	err := Update(func(f *File) error {
		f.APIKey = "should-not-persist"
		return wantErr
	})
	if err == nil {
		t.Fatalf("expected error from Update")
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != "keep" {
		t.Fatalf("APIKey = %q, want unchanged %q", got.APIKey, "keep")
	}
}
