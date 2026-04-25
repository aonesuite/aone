package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffold_GeneratesAllExpectedFilesPerLanguage(t *testing.T) {
	cases := map[string][]string{
		"go":         {"main.go", "go.mod", "Makefile", "Dockerfile", "aone.sandbox.toml"},
		"typescript": {"template.ts", "package.json", "Dockerfile", "aone.sandbox.toml"},
		"python":     {"template.py", "requirements.txt", "Dockerfile", "aone.sandbox.toml"},
	}

	for lang, want := range cases {
		t.Run(lang, func(t *testing.T) {
			dir := t.TempDir()
			if err := scaffold("demo-template", lang, dir); err != nil {
				t.Fatalf("scaffold(%s): %v", lang, err)
			}
			for _, name := range want {
				p := filepath.Join(dir, name)
				info, err := os.Stat(p)
				if err != nil {
					t.Fatalf("missing %s: %v", name, err)
				}
				if info.Size() == 0 {
					t.Fatalf("%s is empty", name)
				}
			}

			// aone.sandbox.toml must be the canonical name (not a legacy one)
			// and contain the project name passed in.
			toml, err := os.ReadFile(filepath.Join(dir, "aone.sandbox.toml"))
			if err != nil {
				t.Fatalf("read aone.sandbox.toml: %v", err)
			}
			if !strings.Contains(string(toml), "demo-template") {
				t.Fatalf("aone.sandbox.toml does not embed template name; got:\n%s", toml)
			}
		})
	}
}

func TestScaffold_UnsupportedLanguageReturnsError(t *testing.T) {
	err := scaffold("x", "rust", t.TempDir())
	if err == nil {
		t.Fatalf("expected error for unsupported language")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("error = %v, want to contain 'unsupported language'", err)
	}
}

func TestScaffold_CreatesMissingTargetDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "deep", "nested")
	if err := scaffold("p", "go", nested); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, "main.go")); err != nil {
		t.Fatalf("main.go not created in nested dir: %v", err)
	}
}
