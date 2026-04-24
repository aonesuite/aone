package template

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
	"github.com/aonesuite/aone/packages/go/sandbox"
)

// MigrateInfo holds parameters for migrating a Dockerfile-based template to
// the SDK-native template definition.
type MigrateInfo struct {
	// Dockerfile is the path to the Dockerfile. When empty the default
	// candidates (e2b.Dockerfile, Dockerfile) are tried in Path.
	Dockerfile string

	// Path is the project root. Defaults to the current working directory.
	Path string

	// Language is the target SDK language: go, typescript, python.
	// When empty, an interactive prompt is shown for TTY callers.
	Language string

	// Name overrides the generated template name. Defaults to the directory
	// name of Path.
	Name string
}

// Migrate converts a Dockerfile into an SDK-native template source file.
func Migrate(info MigrateInfo) {
	root := info.Path
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			sbClient.PrintError("resolve working directory: %v", err)
			return
		}
		root = cwd
	}

	content, dockerfilePath, err := readDockerfile(root, info.Dockerfile)
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	converted, err := sandbox.ConvertDockerfile(content)
	if err != nil {
		sbClient.PrintError("parse Dockerfile: %v", err)
		return
	}

	language, err := resolveLanguage(info.Language)
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	name := info.Name
	if name == "" {
		name = filepath.Base(root)
	}

	fmt.Printf("Migrating %s to %s SDK template...\n", dockerfilePath, language)

	if err := writeMigratedTemplate(root, name, language, converted); err != nil {
		sbClient.PrintError("migrate failed: %v", err)
		return
	}

	sbClient.PrintSuccess("Template migrated successfully")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  Review the generated template file in %s\n", root)
	fmt.Printf("  Build with: aone sandbox template build --name %s --dockerfile %s\n", name, dockerfilePath)
}

// readDockerfile returns the Dockerfile content and resolved path. When the
// explicit path is empty, the default candidates are tried in root.
func readDockerfile(root, explicit string) (string, string, error) {
	if explicit != "" {
		b, err := os.ReadFile(explicit)
		if err != nil {
			return "", "", fmt.Errorf("read Dockerfile %s: %w", explicit, err)
		}
		return string(b), explicit, nil
	}
	for _, candidate := range []string{"e2b.Dockerfile", "Dockerfile"} {
		p := filepath.Join(root, candidate)
		b, err := os.ReadFile(p)
		if err == nil {
			return string(b), p, nil
		}
		if !os.IsNotExist(err) {
			return "", "", fmt.Errorf("read Dockerfile %s: %w", p, err)
		}
	}
	return "", "", fmt.Errorf("no Dockerfile found. Specify one with -d/--dockerfile")
}

// resolveLanguage validates the requested language or prompts interactively.
func resolveLanguage(requested string) (string, error) {
	if requested != "" {
		lang := strings.ToLower(requested)
		if !isSupportedLanguage(lang) {
			return "", fmt.Errorf("unsupported language %q (supported: %s)", requested, strings.Join(supportedLanguages, ", "))
		}
		return lang, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("--language is required in non-interactive mode (%s)", strings.Join(supportedLanguages, ", "))
	}
	var selected string
	options := make([]huh.Option[string], 0, len(supportedLanguages))
	for _, lang := range supportedLanguages {
		options = append(options, huh.NewOption(lang, lang))
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select target language for Template SDK").
			Options(options...).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("cancelled: %w", err)
	}
	return selected, nil
}

// isSupportedLanguage reports whether lang is in supportedLanguages.
func isSupportedLanguage(lang string) bool {
	return slices.Contains(supportedLanguages, lang)
}

// writeMigratedTemplate emits the generated template source file. Existing
// scaffolding files (go.mod, package.json, requirements.txt) are left alone.
func writeMigratedTemplate(root, name, language string, r *sandbox.DockerfileConvertResult) error {
	var (
		outputFile string
		body       string
	)
	switch language {
	case "go":
		outputFile = filepath.Join(root, "template.go")
		body = generateGoTemplate(name, r)
	case "typescript":
		outputFile = filepath.Join(root, "template.ts")
		body = generateTypeScriptTemplate(r)
	case "python":
		outputFile = filepath.Join(root, "template.py")
		body = generatePythonTemplate(r)
	default:
		return fmt.Errorf("unsupported language: %s", language)
	}
	if err := os.WriteFile(outputFile, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputFile, err)
	}
	sbClient.PrintSuccess("  Generated %s", outputFile)
	return nil
}

// stepMethods maps Dockerfile step types to builder method names in the
// target language.
type stepMethods struct {
	run, copy, workdir, user, env string
}

// renderSteps writes chained builder calls for the supported Dockerfile steps.
// Unknown / unsupported step types are skipped. `prefix` is emitted before
// each call (e.g. ".\n\t\t" for Go, "\n  " for TS). `sep` separates the
// method name from its argument list ("(" in every language we target).
func renderSteps(w io.Writer, steps []sandbox.TemplateStep, prefix string, m stepMethods) {
	for _, s := range steps {
		args := []string{}
		if s.Args != nil {
			args = *s.Args
		}
		switch strings.ToUpper(s.Type) {
		case "RUN":
			if len(args) > 0 && m.run != "" {
				fmt.Fprintf(w, "%s%s(%q)", prefix, m.run, strings.Join(args, " "))
			}
		case "COPY":
			if len(args) >= 2 && m.copy != "" {
				fmt.Fprintf(w, "%s%s(%q, %q)", prefix, m.copy, args[0], args[1])
			}
		case "WORKDIR":
			if len(args) > 0 && m.workdir != "" {
				fmt.Fprintf(w, "%s%s(%q)", prefix, m.workdir, args[0])
			}
		case "USER":
			if len(args) > 0 && m.user != "" {
				fmt.Fprintf(w, "%s%s(%q)", prefix, m.user, args[0])
			}
		case "ENV":
			if len(args) >= 2 && m.env != "" {
				fmt.Fprintf(w, "%s%s(%q, %q)", prefix, m.env, args[0], args[1])
			}
		}
	}
}

// generateGoTemplate renders a Go template definition using sandbox.TemplateBuilder.
func generateGoTemplate(name string, r *sandbox.DockerfileConvertResult) string {
	var b strings.Builder
	b.WriteString("// Code generated by `aone sandbox template migrate`. Edit as needed.\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n\t\"github.com/aonesuite/aone/packages/go/sandbox\"\n)\n\n")
	fmt.Fprintf(&b, "func Template%s() *sandbox.TemplateBuilder {\n", exportedName(name))
	b.WriteString("\tt := sandbox.NewTemplate().\n")
	fmt.Fprintf(&b, "\t\tFromImage(%q)", r.BaseImage)
	// Go uses SetEnvs(map[string]string{...}) instead of a two-arg method, so
	// ENV is handled explicitly after renderSteps.
	renderSteps(&b, r.Steps, ".\n\t\t", stepMethods{
		run:     "RunCmd",
		copy:    "Copy",
		workdir: "SetWorkdir",
		user:    "SetUser",
	})
	for _, s := range r.Steps {
		if strings.ToUpper(s.Type) != "ENV" || s.Args == nil || len(*s.Args) < 2 {
			continue
		}
		fmt.Fprintf(&b, ".\n\t\tSetEnvs(map[string]string{%q: %q})", (*s.Args)[0], (*s.Args)[1])
	}
	if r.StartCmd != "" {
		// WaitForTimeout is a sensible default; swap to WaitForPort / WaitForURL /
		// WaitForFile in the generated code if the template needs a real readiness probe.
		fmt.Fprintf(&b, ".\n\t\tSetStartCmd(%q, sandbox.WaitForTimeout(20))", r.StartCmd)
	}
	b.WriteString("\n\treturn t\n}\n")
	return b.String()
}

// generateTypeScriptTemplate renders a TypeScript template definition matching the e2b node SDK style.
func generateTypeScriptTemplate(r *sandbox.DockerfileConvertResult) string {
	var b strings.Builder
	b.WriteString("// Code generated by `aone sandbox template migrate`. Edit as needed.\n")
	b.WriteString("import { Template } from 'e2b'\n\n")
	fmt.Fprintf(&b, "export const template = Template()\n  .fromImage(%q)", r.BaseImage)
	renderSteps(&b, r.Steps, "\n  .", stepMethods{
		run:     "runCmd",
		copy:    "copy",
		workdir: "setWorkdir",
		user:    "setUser",
	})
	if r.StartCmd != "" {
		ready := r.ReadyCmd
		if ready == "" {
			ready = "sleep 20"
		}
		fmt.Fprintf(&b, "\n  .setStartCmd(%q, %q)", r.StartCmd, ready)
	}
	b.WriteString("\n")
	return b.String()
}

// generatePythonTemplate renders a Python template definition matching the e2b python SDK style.
func generatePythonTemplate(r *sandbox.DockerfileConvertResult) string {
	var b strings.Builder
	b.WriteString("# Code generated by `aone sandbox template migrate`. Edit as needed.\n")
	b.WriteString("from e2b import Template\n\n")
	fmt.Fprintf(&b, "template = (\n    Template()\n    .from_image(%q)", r.BaseImage)
	renderSteps(&b, r.Steps, "\n    .", stepMethods{
		run:     "run_cmd",
		copy:    "copy",
		workdir: "set_workdir",
		user:    "set_user",
	})
	if r.StartCmd != "" {
		ready := r.ReadyCmd
		if ready == "" {
			ready = "sleep 20"
		}
		fmt.Fprintf(&b, "\n    .set_start_cmd(%q, %q)", r.StartCmd, ready)
	}
	b.WriteString("\n)\n")
	return b.String()
}

// exportedName converts a template name (lowercase, possibly hyphenated) into
// an exported Go identifier by title-casing each segment.
func exportedName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	s := strings.Join(parts, "")
	if s == "" {
		return "Migrated"
	}
	return s
}
