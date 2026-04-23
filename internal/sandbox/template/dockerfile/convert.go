// Package dockerfile adapts generic Dockerfile parse results to sandbox.TemplateStep values.
package dockerfile

import (
	"fmt"
	"strings"

	"github.com/aonesuite/aone/packages/go/sandbox"
)

// ConvertResult holds Dockerfile conversion output in sandbox format.
type ConvertResult struct {
	// BaseImage is the image name from the FROM instruction.
	BaseImage string

	// Steps is the list of build steps converted from Dockerfile instructions.
	Steps []sandbox.TemplateStep

	// StartCmd is the startup command extracted from CMD or ENTRYPOINT.
	StartCmd string

	// ReadyCmd defaults to "sleep 20" when CMD or ENTRYPOINT exists, matching the template build behavior.
	ReadyCmd string

	// Warnings contains non-fatal parse issues.
	Warnings []string
}

// Convert parses Dockerfile content and converts instructions to sandbox TemplateStep values.
// It matches the template build-system behavior:
//   - prepend USER root and WORKDIR / steps
//   - append USER user when no USER instruction is present
//   - append WORKDIR /home/user when no WORKDIR instruction is present
func Convert(content string) (*ConvertResult, error) {
	parsed, err := Parse(content)
	if err != nil {
		return nil, err
	}

	result := &ConvertResult{
		Warnings: parsed.Warnings,
	}
	var steps []sandbox.TemplateStep
	hasUser := false
	hasWorkdir := false
	fromCount := 0

	// Insert default steps to match the template build behavior.
	steps = append(steps, makeStep("USER", "root"))
	steps = append(steps, makeStep("WORKDIR", "/"))

	for _, inst := range parsed.Instructions {
		switch inst.Name {
		case "FROM":
			fromCount++
			if fromCount > 1 {
				result.Warnings = append(result.Warnings,
					"multi-stage build detected; using the last FROM stage as the runtime base image")
			}
			// Extract the image name; multi-stage builds use the last FROM stage.
			result.BaseImage = extractImage(inst.Args)

		case "RUN":
			if inst.Args == "" {
				return nil, fmt.Errorf("line %d: empty RUN instruction", inst.Line)
			}
			args := []string{inst.Args}
			steps = append(steps, sandbox.TemplateStep{
				Type: "RUN",
				Args: &args,
			})

		case "COPY", "ADD":
			user, src, dest, err := parseCopyArgs(inst.Args, inst.Flags)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid %s instruction: %w", inst.Line, inst.Name, err)
			}
			args := []string{src, dest, user, ""}
			steps = append(steps, sandbox.TemplateStep{
				Type: "COPY",
				Args: &args,
			})

		case "WORKDIR":
			if inst.Args == "" {
				return nil, fmt.Errorf("line %d: empty WORKDIR instruction", inst.Line)
			}
			hasWorkdir = true
			args := []string{inst.Args}
			steps = append(steps, sandbox.TemplateStep{
				Type: "WORKDIR",
				Args: &args,
			})

		case "USER":
			if inst.Args == "" {
				return nil, fmt.Errorf("line %d: empty USER instruction", inst.Line)
			}
			hasUser = true
			args := []string{inst.Args}
			steps = append(steps, sandbox.TemplateStep{
				Type: "USER",
				Args: &args,
			})

		case "ENV":
			envArgs, err := ParseEnvValues(inst.Args, parsed.EscapeToken)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid ENV instruction: %w", inst.Line, err)
			}
			steps = append(steps, sandbox.TemplateStep{
				Type: "ENV",
				Args: &envArgs,
			})

		case "ARG":
			argArgs, hasDefault := parseArgValues(inst.Args)
			if hasDefault {
				steps = append(steps, sandbox.TemplateStep{
					Type: "ENV",
					Args: &argArgs,
				})
			}
			// ARG without a default is build-time only and does not create an ENV step.

		case "CMD":
			result.StartCmd = ParseCommand(inst.Args)
			result.ReadyCmd = "sleep 20"

		case "ENTRYPOINT":
			result.StartCmd = ParseCommand(inst.Args)
			result.ReadyCmd = "sleep 20"
		}
	}

	// Append defaults when they were not explicitly set, matching the template build behavior.
	if !hasUser {
		steps = append(steps, makeStep("USER", "user"))
	}
	if !hasWorkdir {
		steps = append(steps, makeStep("WORKDIR", "/home/user"))
	}

	if result.BaseImage == "" {
		return nil, fmt.Errorf("no FROM instruction found in Dockerfile")
	}

	result.Steps = steps
	return result, nil
}

// extractImage extracts the image name from FROM arguments and ignores AS aliases.
func extractImage(args string) string {
	for f := range strings.FieldsSeq(args) {
		if strings.ToUpper(f) == "AS" {
			break
		}
		return f
	}
	return args
}

// parseCopyArgs extracts user, source, and destination from COPY/ADD instructions.
func parseCopyArgs(args string, flags map[string]string) (user, src, dest string, err error) {
	// Extract the user from the --chown flag.
	if chown, ok := flags["chown"]; ok {
		if u, _, found := strings.Cut(chown, ":"); found {
			user = u
		} else {
			user = chown
		}
	}

	// Remove heredoc markers.
	args = StripHeredocMarkers(args)

	fields := strings.Fields(args)
	if len(fields) < 2 {
		return "", "", "", fmt.Errorf("COPY/ADD requires at least source and destination")
	}

	dest = fields[len(fields)-1]
	src = strings.Join(fields[:len(fields)-1], " ")
	return user, src, dest, nil
}

// parseArgValues parses ARG name[=default_value] into ["name", "value"].
// It returns the parsed values and whether a default value was present.
func parseArgValues(rest string) ([]string, bool) {
	key, value, hasDefault := strings.Cut(rest, "=")
	return []string{strings.TrimSpace(key), strings.TrimSpace(value)}, hasDefault
}

// makeStep creates a simple TemplateStep.
func makeStep(typ string, args ...string) sandbox.TemplateStep {
	a := make([]string, len(args))
	copy(a, args)
	return sandbox.TemplateStep{
		Type: typ,
		Args: &a,
	}
}
