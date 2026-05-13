package sandbox

import (
	"fmt"
	"strings"

	"github.com/aonesuite/aone/packages/go/sandbox/dockerfile"
)

// DockerfileConvertResult holds Dockerfile conversion output in sandbox format.
// Callers that want the low-level step representation (e.g. to render a
// TypeScript or Python template) can use this directly. Most callers should
// use FromDockerfile which returns a pre-populated TemplateBuilder.
type DockerfileConvertResult struct {
	// BaseImage is the image name from the FROM instruction.
	BaseImage string

	// Steps is the list of build steps converted from Dockerfile instructions.
	Steps []TemplateStep

	// StartCmd is the startup command extracted from CMD or ENTRYPOINT.
	StartCmd string

	// ReadyCmd defaults to "sleep 20" when CMD or ENTRYPOINT exists, matching
	// the template build behavior.
	ReadyCmd string

	// Warnings contains non-fatal parse issues (e.g. multi-stage builds).
	Warnings []string
}

// ConvertDockerfile parses Dockerfile content and converts instructions to
// TemplateStep values. It mirrors the template build-system behavior:
//   - prepend USER root and WORKDIR / steps
//   - append USER user when no USER instruction is present
//   - append WORKDIR /home/user when no WORKDIR instruction is present
func ConvertDockerfile(content string) (*DockerfileConvertResult, error) {
	parsed, err := dockerfile.Parse(content)
	if err != nil {
		return nil, err
	}

	result := &DockerfileConvertResult{Warnings: parsed.Warnings}
	steps := []TemplateStep{
		makeDockerStep("USER", "root"),
		makeDockerStep("WORKDIR", "/"),
	}
	hasUser := false
	hasWorkdir := false
	fromCount := 0

	for _, inst := range parsed.Instructions {
		switch inst.Name {
		case "FROM":
			fromCount++
			if fromCount > 1 {
				result.Warnings = append(result.Warnings,
					"multi-stage build detected; using the last FROM stage as the runtime base image")
			}
			result.BaseImage = extractDockerImage(inst.Args)

		case "RUN":
			if inst.Args == "" {
				return nil, fmt.Errorf("line %d: empty RUN instruction", inst.Line)
			}
			args := []string{inst.Args}
			steps = append(steps, TemplateStep{Type: "RUN", Args: &args})

		case "COPY", "ADD":
			user, src, dest, cErr := parseDockerCopyArgs(inst.Args, inst.Flags)
			if cErr != nil {
				return nil, fmt.Errorf("line %d: invalid %s instruction: %w", inst.Line, inst.Name, cErr)
			}
			args := []string{src, dest, user, ""}
			steps = append(steps, TemplateStep{Type: "COPY", Args: &args})

		case "WORKDIR":
			if inst.Args == "" {
				return nil, fmt.Errorf("line %d: empty WORKDIR instruction", inst.Line)
			}
			hasWorkdir = true
			args := []string{inst.Args}
			steps = append(steps, TemplateStep{Type: "WORKDIR", Args: &args})

		case "USER":
			if inst.Args == "" {
				return nil, fmt.Errorf("line %d: empty USER instruction", inst.Line)
			}
			hasUser = true
			args := []string{inst.Args}
			steps = append(steps, TemplateStep{Type: "USER", Args: &args})

		case "ENV":
			envArgs, eErr := dockerfile.ParseEnvValues(inst.Args, parsed.EscapeToken)
			if eErr != nil {
				return nil, fmt.Errorf("line %d: invalid ENV instruction: %w", inst.Line, eErr)
			}
			steps = append(steps, TemplateStep{Type: "ENV", Args: &envArgs})

		case "ARG":
			argArgs, hasDefault := parseDockerArgValues(inst.Args)
			if hasDefault {
				steps = append(steps, TemplateStep{Type: "ENV", Args: &argArgs})
			}

		case "CMD", "ENTRYPOINT":
			result.StartCmd = dockerfile.ParseCommand(inst.Args)
			result.ReadyCmd = "sleep 20"
		}
	}

	if !hasUser {
		steps = append(steps, makeDockerStep("USER", "user"))
	}
	if !hasWorkdir {
		steps = append(steps, makeDockerStep("WORKDIR", "/home/user"))
	}
	if result.BaseImage == "" {
		return nil, fmt.Errorf("no FROM instruction found in Dockerfile")
	}

	result.Steps = steps
	return result, nil
}

// FromDockerfile parses Dockerfile content and returns a TemplateBuilder
// prepopulated with the base image, steps, and start command. Any remaining
// customization (readyCmd, setEnvs, etc.) can be chained on the returned
// builder.
func FromDockerfile(content string) (*TemplateBuilder, error) {
	result, err := ConvertDockerfile(content)
	if err != nil {
		return nil, err
	}
	b := NewTemplate().FromImage(result.BaseImage)
	applyConvertedSteps(b, result.Steps)
	if result.StartCmd != "" {
		b.SetStartCmd(result.StartCmd, WaitForTimeout(20))
	}
	return b, nil
}

// applyConvertedSteps replays converted Dockerfile steps onto a TemplateBuilder
// using the builder's native methods. Keeping this adapter here (rather than
// exposing raw TemplateStep append) lets the builder preserve invariants it
// enforces elsewhere.
func applyConvertedSteps(b *TemplateBuilder, steps []TemplateStep) {
	envs := map[string]string{}
	for _, s := range steps {
		var args []string
		if s.Args != nil {
			args = *s.Args
		}
		switch strings.ToUpper(s.Type) {
		case "RUN":
			if len(args) > 0 {
				b.RunCmd(args[0])
			}
		case "COPY":
			if len(args) >= 2 {
				b.Copy(args[0], args[1])
			}
		case "WORKDIR":
			if len(args) > 0 {
				b.SetWorkdir(args[0])
			}
		case "USER":
			if len(args) > 0 {
				b.SetUser(args[0])
			}
		case "ENV":
			if len(args) >= 2 {
				envs[args[0]] = args[1]
			}
		}
	}
	if len(envs) > 0 {
		b.SetEnvs(envs)
	}
}

// extractDockerImage extracts the image name from FROM arguments and ignores AS aliases.
func extractDockerImage(args string) string {
	for _, f := range strings.Fields(args) {
		if strings.HasPrefix(f, "--") {
			continue
		}
		if strings.ToUpper(f) == "AS" {
			break
		}
		return f
	}
	return args
}

// parseDockerCopyArgs extracts user, source, and destination from COPY/ADD instructions.
func parseDockerCopyArgs(args string, flags map[string]string) (user, src, dest string, err error) {
	if chown, ok := flags["chown"]; ok {
		if u, _, found := strings.Cut(chown, ":"); found {
			user = u
		} else {
			user = chown
		}
	}
	args = dockerfile.StripHeredocMarkers(args)
	fields := strings.Fields(args)
	if len(fields) < 2 {
		return "", "", "", fmt.Errorf("COPY/ADD requires at least source and destination")
	}
	dest = fields[len(fields)-1]
	src = strings.Join(fields[:len(fields)-1], " ")
	return user, src, dest, nil
}

// parseDockerArgValues parses ARG name[=default_value] into ["name", "value"]
// and reports whether a default value was present.
func parseDockerArgValues(rest string) ([]string, bool) {
	key, value, hasDefault := strings.Cut(rest, "=")
	return []string{strings.TrimSpace(key), strings.TrimSpace(value)}, hasDefault
}

// makeDockerStep creates a simple TemplateStep with the provided args.
func makeDockerStep(typ string, args ...string) TemplateStep {
	a := make([]string, len(args))
	copy(a, args)
	return TemplateStep{Type: typ, Args: &a}
}
