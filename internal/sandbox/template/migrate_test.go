package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/packages/go/sandbox"
)

const sampleDockerfile = `FROM ubuntu:22.04
WORKDIR /app
RUN apt-get update && apt-get install -y curl
COPY . /app
ENV FOO=bar
USER nobody
CMD ["./run.sh"]
`

func TestReadDockerfile_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "MyDockerfile")
	if err := os.WriteFile(p, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	content, got, err := readDockerfile(dir, p)
	if err != nil {
		t.Fatalf("readDockerfile: %v", err)
	}
	if got != p {
		t.Fatalf("path = %q, want %q", got, p)
	}
	if !strings.Contains(content, "FROM scratch") {
		t.Fatalf("content mismatch: %q", content)
	}
}

func TestReadDockerfile_PrefersAonePrefixedCandidate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aone.Dockerfile"), []byte("FROM aone:1\n"), 0o644); err != nil {
		t.Fatalf("write aone.Dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM other:1\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	content, got, err := readDockerfile(dir, "")
	if err != nil {
		t.Fatalf("readDockerfile: %v", err)
	}
	if filepath.Base(got) != "aone.Dockerfile" {
		t.Fatalf("expected aone.Dockerfile to win; got %s", got)
	}
	if !strings.Contains(content, "FROM aone:1") {
		t.Fatalf("content mismatch: %q", content)
	}
}

func TestReadDockerfile_FallsBackToPlainDockerfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, got, err := readDockerfile(dir, "")
	if err != nil {
		t.Fatalf("readDockerfile: %v", err)
	}
	if filepath.Base(got) != "Dockerfile" {
		t.Fatalf("expected Dockerfile; got %s", got)
	}
}

func TestReadDockerfile_NoneFoundReturnsError(t *testing.T) {
	if _, _, err := readDockerfile(t.TempDir(), ""); err == nil {
		t.Fatalf("expected error when no Dockerfile present")
	}
}

func TestReadDockerfile_ExplicitMissingReturnsError(t *testing.T) {
	if _, _, err := readDockerfile(t.TempDir(), "/no/such/file"); err == nil {
		t.Fatalf("expected error for missing explicit path")
	}
}

func TestReadDockerfile_ExplicitRelativeToRoot(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "docker")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(nested, "Customfile")
	if err := os.WriteFile(p, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, got, err := readDockerfile(dir, filepath.Join("docker", "Customfile"))
	if err != nil {
		t.Fatalf("readDockerfile: %v", err)
	}
	if got != p {
		t.Fatalf("path = %q, want %q", got, p)
	}
}

func TestMigrate_UsesProjectConfigForDockerfileAndName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine:3.20\nRUN echo hi\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	if err := config.SaveProject(&config.Project{
		TemplateName: "cfg-demo",
		Dockerfile:   "Dockerfile",
	}, filepath.Join(dir, config.ProjectFileName)); err != nil {
		t.Fatalf("save project: %v", err)
	}

	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Migrate(MigrateInfo{Path: dir, Language: "go"})
		})
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}

	out, err := os.ReadFile(filepath.Join(dir, "template.go"))
	if err != nil {
		t.Fatalf("read migrated template: %v", err)
	}
	if !strings.Contains(string(out), `func TemplateCfgDemo() *sandbox.TemplateBuilder`) {
		t.Fatalf("expected config-derived name in migrated template; got:\n%s", out)
	}
}

func TestIsSupportedLanguage(t *testing.T) {
	cases := map[string]bool{
		"go":         true,
		"typescript": true,
		"python":     true,
		"GO":         false, // case-sensitive
		"rust":       false,
		"":           false,
	}
	for in, want := range cases {
		if got := isSupportedLanguage(in); got != want {
			t.Errorf("isSupportedLanguage(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveLanguage_AcceptsValid(t *testing.T) {
	for _, lang := range supportedLanguages {
		got, err := resolveLanguage(lang)
		if err != nil {
			t.Errorf("resolveLanguage(%q): %v", lang, err)
		}
		if got != lang {
			t.Errorf("resolveLanguage(%q) = %q", lang, got)
		}
	}
}

func TestResolveLanguage_NormalizesCase(t *testing.T) {
	got, err := resolveLanguage("GO")
	if err != nil {
		t.Fatalf("resolveLanguage(GO): %v", err)
	}
	if got != "go" {
		t.Fatalf("expected lowercase 'go', got %q", got)
	}
}

func TestResolveLanguage_RejectsUnknown(t *testing.T) {
	_, err := resolveLanguage("rust")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveLanguage_NonInteractiveRequiresFlag(t *testing.T) {
	// In `go test` stdin is not a TTY, so the empty-language branch should
	// fail rather than try to launch the interactive prompt.
	_, err := resolveLanguage("")
	if err == nil {
		t.Fatalf("expected error in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "non-interactive") {
		t.Fatalf("error = %v, want 'non-interactive'", err)
	}
}

func TestExportedName(t *testing.T) {
	cases := map[string]string{
		"foo":          "Foo",
		"my-template":  "MyTemplate",
		"my_template":  "MyTemplate",
		"a-b-c":        "ABC",
		"AlreadyCased": "AlreadyCased",
		"":             "Migrated",
		"--":           "Migrated",
	}
	for in, want := range cases {
		if got := exportedName(in); got != want {
			t.Errorf("exportedName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateGoTemplate_ContainsExpectedStructure(t *testing.T) {
	r, err := sandbox.ConvertDockerfile(sampleDockerfile)
	if err != nil {
		t.Fatalf("ConvertDockerfile: %v", err)
	}
	out := generateGoTemplate("my-app", r)

	wantSubstrings := []string{
		`package main`,
		`"github.com/aonesuite/aone/packages/go/sandbox"`,
		`func TemplateMyApp() *sandbox.TemplateBuilder`,
		`FromImage("ubuntu:22.04")`,
		`SetWorkdir("/app")`,
		`SetUser("nobody")`,
		`SetEnvs(map[string]string{"FOO": "bar"})`,
		`SetStartCmd(`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("Go template missing %q; full:\n%s", s, out)
		}
	}
	// Generated templates should use only aone imports.
	if !strings.Contains(out, "github.com/aonesuite/aone/packages/go/sandbox") {
		t.Errorf("generated Go template missing aone import:\n%s", out)
	}
}

func TestGenerateTypeScriptTemplate_UsesAonesuiteImport(t *testing.T) {
	r, err := sandbox.ConvertDockerfile(sampleDockerfile)
	if err != nil {
		t.Fatalf("ConvertDockerfile: %v", err)
	}
	out := generateTypeScriptTemplate(r)

	wantSubstrings := []string{
		`import { Template } from '@aonesuite/sandbox'`,
		`Template()`,
		`.fromImage("ubuntu:22.04")`,
		`.setWorkdir("/app")`,
		`.setUser("nobody")`,
		`.setStartCmd(`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("TS template missing %q; full:\n%s", s, out)
		}
	}
	if !strings.Contains(out, "from '@aonesuite/sandbox'") {
		t.Errorf("TS template missing aone import:\n%s", out)
	}
}

func TestGeneratePythonTemplate_UsesAonesuiteImport(t *testing.T) {
	r, err := sandbox.ConvertDockerfile(sampleDockerfile)
	if err != nil {
		t.Fatalf("ConvertDockerfile: %v", err)
	}
	out := generatePythonTemplate(r)

	wantSubstrings := []string{
		`from aonesuite.sandbox import Template`,
		`Template()`,
		`.from_image("ubuntu:22.04")`,
		`.set_workdir("/app")`,
		`.set_user("nobody")`,
		`.set_start_cmd(`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("Python template missing %q; full:\n%s", s, out)
		}
	}
	if !strings.Contains(out, "from aonesuite.sandbox") {
		t.Errorf("Python template missing aone import:\n%s", out)
	}
}

func TestGenerateTemplate_NoStartCmdOmitsSection(t *testing.T) {
	noCmd := "FROM alpine\nRUN echo hi\n"
	r, err := sandbox.ConvertDockerfile(noCmd)
	if err != nil {
		t.Fatalf("ConvertDockerfile: %v", err)
	}
	if got := generateGoTemplate("x", r); strings.Contains(got, "SetStartCmd(") {
		t.Errorf("Go: SetStartCmd should be omitted when there's no CMD: %s", got)
	}
	if got := generateTypeScriptTemplate(r); strings.Contains(got, ".setStartCmd(") {
		t.Errorf("TS: .setStartCmd should be omitted when there's no CMD: %s", got)
	}
	if got := generatePythonTemplate(r); strings.Contains(got, ".set_start_cmd(") {
		t.Errorf("Python: .set_start_cmd should be omitted when there's no CMD: %s", got)
	}
}

func TestWriteMigratedTemplate_WritesPerLanguage(t *testing.T) {
	r, err := sandbox.ConvertDockerfile(sampleDockerfile)
	if err != nil {
		t.Fatalf("ConvertDockerfile: %v", err)
	}

	cases := map[string]string{
		"go":         "template.go",
		"typescript": "template.ts",
		"python":     "template.py",
	}
	for lang, file := range cases {
		t.Run(lang, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeMigratedTemplate(dir, "demo", lang, r); err != nil {
				t.Fatalf("writeMigratedTemplate: %v", err)
			}
			info, err := os.Stat(filepath.Join(dir, file))
			if err != nil {
				t.Fatalf("missing %s: %v", file, err)
			}
			if info.Size() == 0 {
				t.Fatalf("%s is empty", file)
			}
		})
	}
}

func TestWriteMigratedTemplate_UnsupportedLanguageReturnsError(t *testing.T) {
	r := &sandbox.DockerfileConvertResult{BaseImage: "x"}
	err := writeMigratedTemplate(t.TempDir(), "n", "rust", r)
	if err == nil {
		t.Fatalf("expected error for unsupported language")
	}
}
