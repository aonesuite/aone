package template

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// validNamePattern validates template names: lowercase alphanumeric, starting with a-z or 0-9.
var validNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// supportedLanguages are the accepted language names for init. Python sync
// and async currently share the same scaffold template in aone.
var supportedLanguages = []string{"go", "typescript", "python", "python-sync", "python-async"}

// InitInfo holds parameters for initializing a template project.
type InitInfo struct {
	Name     string // Template project name
	Language string // Programming language
	Path     string // Parent/root directory for the generated project
}

// Init initializes a new template project with scaffolded files.
// When parameters are not provided, uses interactive prompts.
func Init(info InitInfo) {
	name := info.Name
	language := strings.ToLower(info.Language)
	path := info.Path

	// Interactive prompts if args are missing
	if name == "" || language == "" {
		// Require TTY for interactive mode
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			sbClient.PrintError("--name and --language are required in non-interactive mode")
			return
		}

		var fields []huh.Field

		if name == "" {
			fields = append(fields,
				huh.NewInput().
					Title("Template name").
					Description("Lowercase alphanumeric, hyphens and underscores allowed").
					Value(&name).
					Validate(func(s string) error {
						if !validNamePattern.MatchString(s) {
							return fmt.Errorf("name must match pattern: [a-z0-9][a-z0-9_-]*")
						}
						return nil
					}),
			)
		}

		if language == "" {
			promptLanguages := []string{"go", "typescript", "python-sync", "python-async"}
			langOptions := make([]huh.Option[string], 0, len(promptLanguages))
			for _, lang := range promptLanguages {
				langOptions = append(langOptions, huh.NewOption(lang, lang))
			}
			fields = append(fields,
				huh.NewSelect[string]().
					Title("Programming language").
					Options(langOptions...).
					Value(&language),
			)
		}

		if len(fields) > 0 {
			form := huh.NewForm(huh.NewGroup(fields...))
			if fErr := form.Run(); fErr != nil {
				sbClient.PrintError("cancelled: %v", fErr)
				return
			}
		}
	}

	// Validate name
	if !validNamePattern.MatchString(name) {
		sbClient.PrintError("invalid template name %q (must match: [a-z0-9][a-z0-9_-]*)", name)
		return
	}

	// Validate language
	scaffoldLanguage, ok := normalizeInitLanguage(language)
	if !ok {
		sbClient.PrintError("unsupported language %q (supported: %s)", language, strings.Join(supportedLanguages, ", "))
		return
	}

	if path == "" {
		path = "."
	}
	targetDir := filepath.Join(path, name)

	fmt.Printf("Initializing %s template %q in %s...\n", language, name, targetDir)
	if err := scaffold(name, scaffoldLanguage, targetDir); err != nil {
		sbClient.PrintError("scaffold failed: %v", err)
		return
	}
	sbClient.PrintSuccess("Template %s initialized successfully!", name)
}

func normalizeInitLanguage(language string) (string, bool) {
	switch language {
	case "go", "typescript", "python-sync", "python-async":
		return language, true
	case "python":
		return "python-sync", true
	default:
		return "", false
	}
}
