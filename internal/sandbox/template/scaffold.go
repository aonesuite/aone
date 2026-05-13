package template

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

//go:embed templates/go/*.tmpl templates/typescript/*.tmpl templates/python/*.tmpl
var templateFS embed.FS

// scaffoldData holds template rendering context.
type scaffoldData struct {
	Name            string
	LanguageLabel   string
	Async           bool
	BuildScriptName string
}

// languageFiles maps language names to their template directory and output files.
var languageFiles = map[string][]struct {
	tmpl   string // template file path within embed FS
	output string // output file name
}{
	"go": {
		{tmpl: "templates/go/main.go.tmpl", output: "main.go"},
		{tmpl: "templates/go/go.mod.tmpl", output: "go.mod"},
		{tmpl: "templates/go/Makefile.tmpl", output: "Makefile"},
		{tmpl: "templates/go/Dockerfile.tmpl", output: "Dockerfile"},
		{tmpl: "templates/go/aone.sandbox.toml.tmpl", output: "aone.sandbox.toml"},
		{tmpl: "templates/go/README.md.tmpl", output: "README.md"},
	},
	"typescript": {
		{tmpl: "templates/typescript/template.ts.tmpl", output: "template.ts"},
		{tmpl: "templates/typescript/build.dev.ts.tmpl", output: "build.dev.ts"},
		{tmpl: "templates/typescript/build.prod.ts.tmpl", output: "build.prod.ts"},
		{tmpl: "templates/typescript/package.json.tmpl", output: "package.json"},
		{tmpl: "templates/typescript/Dockerfile.tmpl", output: "Dockerfile"},
		{tmpl: "templates/typescript/aone.sandbox.toml.tmpl", output: "aone.sandbox.toml"},
		{tmpl: "templates/typescript/README.md.tmpl", output: "README.md"},
	},
	"python": {
		{tmpl: "templates/python/template.py.tmpl", output: "template.py"},
		{tmpl: "templates/python/requirements.txt.tmpl", output: "requirements.txt"},
		{tmpl: "templates/python/Dockerfile.tmpl", output: "Dockerfile"},
		{tmpl: "templates/python/aone.sandbox.toml.tmpl", output: "aone.sandbox.toml"},
		{tmpl: "templates/python/build_dev.py.tmpl", output: "build_dev.py"},
		{tmpl: "templates/python/build_prod.py.tmpl", output: "build_prod.py"},
		{tmpl: "templates/python/README.md.tmpl", output: "README.md"},
		{tmpl: "templates/python/Makefile.tmpl", output: "Makefile"},
	},
	"python-sync": {
		{tmpl: "templates/python/template.py.tmpl", output: "template.py"},
		{tmpl: "templates/python/requirements.txt.tmpl", output: "requirements.txt"},
		{tmpl: "templates/python/Dockerfile.tmpl", output: "Dockerfile"},
		{tmpl: "templates/python/aone.sandbox.toml.tmpl", output: "aone.sandbox.toml"},
		{tmpl: "templates/python/build_dev.py.tmpl", output: "build_dev.py"},
		{tmpl: "templates/python/build_prod.py.tmpl", output: "build_prod.py"},
		{tmpl: "templates/python/README.md.tmpl", output: "README.md"},
		{tmpl: "templates/python/Makefile.tmpl", output: "Makefile"},
	},
	"python-async": {
		{tmpl: "templates/python/template.py.tmpl", output: "template.py"},
		{tmpl: "templates/python/requirements.txt.tmpl", output: "requirements.txt"},
		{tmpl: "templates/python/Dockerfile.tmpl", output: "Dockerfile"},
		{tmpl: "templates/python/aone.sandbox.toml.tmpl", output: "aone.sandbox.toml"},
		{tmpl: "templates/python/build_dev.py.tmpl", output: "build_dev.py"},
		{tmpl: "templates/python/build_prod.py.tmpl", output: "build_prod.py"},
		{tmpl: "templates/python/README.md.tmpl", output: "README.md"},
		{tmpl: "templates/python/Makefile.tmpl", output: "Makefile"},
	},
}

// scaffold generates project files for the given language in the target directory.
func scaffold(name, language, targetDir string) error {
	files, ok := languageFiles[language]
	if !ok {
		return fmt.Errorf("unsupported language: %s", language)
	}

	data := scaffoldData{
		Name:            name,
		LanguageLabel:   scaffoldLanguageLabel(language),
		Async:           language == "python-async",
		BuildScriptName: scaffoldBuildScriptName(language),
	}

	// Create target directory if needed
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	for _, f := range files {
		tmplContent, rErr := templateFS.ReadFile(f.tmpl)
		if rErr != nil {
			return fmt.Errorf("read template %s: %w", f.tmpl, rErr)
		}

		tmpl, pErr := template.New(f.output).Parse(string(tmplContent))
		if pErr != nil {
			return fmt.Errorf("parse template %s: %w", f.tmpl, pErr)
		}

		var buf bytes.Buffer
		if eErr := tmpl.Execute(&buf, data); eErr != nil {
			return fmt.Errorf("execute template %s: %w", f.tmpl, eErr)
		}

		outPath := filepath.Join(targetDir, f.output)
		if wErr := os.WriteFile(outPath, buf.Bytes(), 0o644); wErr != nil {
			return fmt.Errorf("write %s: %w", outPath, wErr)
		}
		sbClient.PrintSuccess("  Created %s", outPath)
	}

	return nil
}

func scaffoldLanguageLabel(language string) string {
	switch language {
	case "python-async":
		return "Python (async)"
	case "python", "python-sync":
		return "Python (sync)"
	case "typescript":
		return "TypeScript"
	case "go":
		return "Go"
	default:
		return language
	}
}

func scaffoldBuildScriptName(language string) string {
	if language == "python-async" {
		return "AsyncTemplate.build"
	}
	return "Template.build"
}
