package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadProject_NoFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	p, loc, err := LoadProject("", dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if p != nil || loc != nil {
		t.Fatalf("expected (nil, nil, nil); got (%+v, %+v)", p, loc)
	}
}

func TestLoadProject_OnlyReadsAoneProjectFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ProjectFileName), `template_id = "tpl-aone"`)
	writeFile(t, filepath.Join(dir, "other.toml"), `template_id = "tpl-other"`)

	p, loc, err := LoadProject("", dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if p.TemplateID != "tpl-aone" {
		t.Fatalf("TemplateID = %q, want tpl-aone", p.TemplateID)
	}
	if filepath.Base(loc.Path) != ProjectFileName {
		t.Fatalf("Path = %q, want suffix %q", loc.Path, ProjectFileName)
	}
}

func TestLoadProject_ExplicitPathMustExist(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.toml")
	_, _, err := LoadProject(missing, dir)
	if err == nil {
		t.Fatalf("expected error for missing explicit path")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("error = %v, want 'config file not found'", err)
	}
}

func TestLoadProject_ExplicitPathLoads(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.toml")
	writeFile(t, custom, `template_id = "tpl-x"`)

	p, loc, err := LoadProject(custom, "")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if p.TemplateID != "tpl-x" {
		t.Fatalf("TemplateID = %q, want tpl-x", p.TemplateID)
	}
	if loc.Path != mustAbs(t, custom) {
		t.Fatalf("Path = %q, want %q", loc.Path, custom)
	}
}

func TestLoadProject_InvalidTOMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ProjectFileName), `template_id = `) // dangling

	if _, _, err := LoadProject("", dir); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestSaveProject_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "sub", ProjectFileName)

	in := &Project{
		TemplateID:   "tpl-1",
		TemplateName: "demo",
		Dockerfile:   "Dockerfile",
		StartCmd:     "/start.sh",
		CPUCount:     4,
		MemoryMB:     2048,
		DiskSizeMB:   8192,
		Public:       boolPtr(true),
	}
	if err := SaveProject(in, dest); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	got, _, err := LoadProject(dest, "")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if got.TemplateID != in.TemplateID ||
		got.TemplateName != in.TemplateName ||
		got.Dockerfile != in.Dockerfile ||
		got.StartCmd != in.StartCmd ||
		got.ReadyCmd != in.ReadyCmd ||
		got.CPUCount != in.CPUCount ||
		got.MemoryMB != in.MemoryMB ||
		got.DiskSizeMB != in.DiskSizeMB ||
		got.Public == nil ||
		in.Public == nil ||
		*got.Public != *in.Public {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", got, in)
	}
	// No leftover temp files.
	entries, err := os.ReadDir(filepath.Dir(dest))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == filepath.Base(dest) {
			continue
		}
		t.Fatalf("unexpected leftover %q", e.Name())
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestDefaultProjectPath(t *testing.T) {
	got := DefaultProjectPath("")
	if got != filepath.Join(".", ProjectFileName) {
		t.Fatalf("DefaultProjectPath(\"\") = %q", got)
	}

	got = DefaultProjectPath("/tmp/proj")
	if got != filepath.Join("/tmp/proj", ProjectFileName) {
		t.Fatalf("DefaultProjectPath = %q", got)
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs %q: %v", p, err)
	}
	return abs
}
