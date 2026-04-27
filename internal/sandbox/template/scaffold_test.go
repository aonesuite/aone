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
		"typescript": {"template.ts", "package.json", "Dockerfile", "aone.sandbox.toml", "build.dev.ts", "build.prod.ts", "README.md"},
		"python":     {"template.py", "requirements.txt", "Dockerfile", "aone.sandbox.toml", "build_dev.py", "build_prod.py", "README.md", "Makefile"},
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

func TestScaffold_PythonAsyncIncludesAsyncHints(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold("demo-template", "python-async", dir); err != nil {
		t.Fatalf("scaffold(python-async): %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	for _, want := range []string{"Python (async)", "Installing Dependencies", "Building the Template", "Using the Template in a Sandbox"} {
		if !strings.Contains(string(readme), want) {
			t.Fatalf("README.md should contain %q; got:\n%s", want, readme)
		}
	}

	makefile, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	for _, want := range []string{"e2b:build:dev", "e2b:build:prod"} {
		if !strings.Contains(string(makefile), want) {
			t.Fatalf("Makefile should contain %q; got:\n%s", want, makefile)
		}
	}
}

func TestScaffold_PythonReadmeAndMakefileLookLikeTemplateProject(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold("demo-template", "python-sync", dir); err != nil {
		t.Fatalf("scaffold(python-sync): %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	for _, want := range []string{"Prerequisites", "Installing Dependencies", "make e2b:build:dev", "Sandbox.create('demo-template')"} {
		if !strings.Contains(string(readme), want) {
			t.Fatalf("README.md should contain %q; got:\n%s", want, readme)
		}
	}

	makefile, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	for _, want := range []string{"e2b:build:dev", "e2b:build:prod"} {
		if !strings.Contains(string(makefile), want) {
			t.Fatalf("Makefile should contain %q; got:\n%s", want, makefile)
		}
	}
}

func TestScaffold_TypeScriptReadmeLooksLikeTemplateProject(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold("demo-template", "typescript", dir); err != nil {
		t.Fatalf("scaffold(typescript): %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	for _, want := range []string{"Prerequisites", "Installing Dependencies", "npm run e2b:build:dev", "Sandbox.create('demo-template')"} {
		if !strings.Contains(string(readme), want) {
			t.Fatalf("README.md should contain %q; got:\n%s", want, readme)
		}
	}
}

func TestScaffold_TypeScriptAddsBuildScriptsAndReadme(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold("demo-template", "typescript", dir); err != nil {
		t.Fatalf("scaffold(typescript): %v", err)
	}

	pkgJSON, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	for _, want := range []string{`"e2b:build:dev"`, `"e2b:build:prod"`, `build.dev.ts`, `build.prod.ts`} {
		if !strings.Contains(string(pkgJSON), want) {
			t.Fatalf("package.json missing %q; got:\n%s", want, pkgJSON)
		}
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "TypeScript") {
		t.Fatalf("README.md should mention TypeScript mode; got:\n%s", readme)
	}

	templateFile, err := os.ReadFile(filepath.Join(dir, "template.ts"))
	if err != nil {
		t.Fatalf("read template.ts: %v", err)
	}
	if !strings.Contains(string(templateFile), `import { Template } from '@aonesuite/sandbox'`) {
		t.Fatalf("template.ts should use @aonesuite/sandbox Template; got:\n%s", templateFile)
	}

	buildDevFile, err := os.ReadFile(filepath.Join(dir, "build.dev.ts"))
	if err != nil {
		t.Fatalf("read build.dev.ts: %v", err)
	}
	if !strings.Contains(string(buildDevFile), `import { template } from './template'`) {
		t.Fatalf("build.dev.ts should import template; got:\n%s", buildDevFile)
	}
}

func TestScaffold_PythonSyncUsesSyncTemplateEntrypoints(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold("demo-template", "python-sync", dir); err != nil {
		t.Fatalf("scaffold(python-sync): %v", err)
	}

	templateFile, err := os.ReadFile(filepath.Join(dir, "template.py"))
	if err != nil {
		t.Fatalf("read template.py: %v", err)
	}
	if !strings.Contains(string(templateFile), "from aonesuite.sandbox import Template") {
		t.Fatalf("template.py should import sync Template; got:\n%s", templateFile)
	}

	buildDevFile, err := os.ReadFile(filepath.Join(dir, "build_dev.py"))
	if err != nil {
		t.Fatalf("read build_dev.py: %v", err)
	}
	if !strings.Contains(string(buildDevFile), "Template.build") {
		t.Fatalf("build_dev.py should use Template.build; got:\n%s", buildDevFile)
	}
}

func TestScaffold_PythonAsyncUsesAsyncTemplateEntrypoints(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold("demo-template", "python-async", dir); err != nil {
		t.Fatalf("scaffold(python-async): %v", err)
	}

	templateFile, err := os.ReadFile(filepath.Join(dir, "template.py"))
	if err != nil {
		t.Fatalf("read template.py: %v", err)
	}
	if !strings.Contains(string(templateFile), "from aonesuite.sandbox import AsyncTemplate") {
		t.Fatalf("template.py should import AsyncTemplate; got:\n%s", templateFile)
	}

	buildDevFile, err := os.ReadFile(filepath.Join(dir, "build_dev.py"))
	if err != nil {
		t.Fatalf("read build_dev.py: %v", err)
	}
	if !strings.Contains(string(buildDevFile), "AsyncTemplate.build") {
		t.Fatalf("build_dev.py should use AsyncTemplate.build; got:\n%s", buildDevFile)
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
