package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ProjectFileName is the canonical name of the per-project template config
// file. It lives in the project root next to the Dockerfile.
const ProjectFileName = "aone.sandbox.toml"

// LegacyProjectFileNames is consulted when ProjectFileName is missing, to
// help users migrating from upstream sandbox tooling. We never write to
// these names; we only read them.
var LegacyProjectFileNames = []string{"e2b.toml"}

// Project holds the on-disk fields persisted to aone.sandbox.toml.
//
// Field tags are intentionally snake_case so the TOML stays idiomatic.
// All fields except TemplateID are optional; the file is meant to
// accumulate state during the build → publish lifecycle.
type Project struct {
	// TemplateID is the server-assigned identifier returned by `template
	// build/create`. Required for subsequent build/publish/delete commands
	// to know which template to act on.
	TemplateID string `toml:"template_id,omitempty"`

	// TemplateName is the human-readable alias for the template.
	TemplateName string `toml:"template_name,omitempty"`

	// Dockerfile is the relative path (from the project root) to the
	// Dockerfile that defines the template image.
	Dockerfile string `toml:"dockerfile,omitempty"`

	// StartCmd is the command executed when a sandbox is created from
	// this template. Optional; if absent the image's CMD applies.
	StartCmd string `toml:"start_cmd,omitempty"`

	// ReadyCmd is the readiness probe used by the build pipeline to
	// detect when the template is fully booted.
	ReadyCmd string `toml:"ready_cmd,omitempty"`

	// CPUCount is the requested vCPU count for sandboxes created from
	// this template. 0 means "use server default".
	CPUCount int `toml:"cpu_count,omitempty"`

	// MemoryMB is the requested memory in MiB.
	MemoryMB int `toml:"memory_mb,omitempty"`
}

// ProjectLocation describes where a Project was loaded from. Useful for
// error messages and for writing the same file back atomically.
type ProjectLocation struct {
	Path   string // Absolute path to the TOML file
	Legacy bool   // True if loaded from a legacy file name
}

// LoadProject finds and parses the project config file using this lookup order:
//
//  1. configPath (if non-empty) — exact file path, must exist.
//  2. <pathDir>/aone.sandbox.toml
//  3. <pathDir>/<legacy names…> (e.g. e2b.toml, read-only)
//
// pathDir defaults to the current working directory when empty.
//
// Returns (nil, nil, nil) when no config file is found and configPath is
// empty — callers decide whether that's an error in their context.
func LoadProject(configPath, pathDir string) (*Project, *ProjectLocation, error) {
	if pathDir == "" {
		pathDir = "."
	}

	candidates := []string{}
	if configPath != "" {
		candidates = append(candidates, configPath)
	} else {
		candidates = append(candidates, filepath.Join(pathDir, ProjectFileName))
		for _, legacy := range LegacyProjectFileNames {
			candidates = append(candidates, filepath.Join(pathDir, legacy))
		}
	}

	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve %s: %w", c, err)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if configPath != "" {
					// Explicit path must exist.
					return nil, nil, fmt.Errorf("config file not found: %s", abs)
				}
				continue
			}
			return nil, nil, fmt.Errorf("read %s: %w", abs, err)
		}
		var p Project
		if _, err := toml.Decode(string(data), &p); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", abs, err)
		}
		loc := &ProjectLocation{
			Path:   abs,
			Legacy: isLegacyName(filepath.Base(abs)),
		}
		return &p, loc, nil
	}

	if configPath != "" {
		return nil, nil, fmt.Errorf("config file not found: %s", configPath)
	}
	return nil, nil, nil
}

// SaveProject atomically writes p to dest. If dest points at a legacy file
// name we still write to it (don't move the user's file behind their back),
// but callers can detect via ProjectLocation.Legacy and warn.
func SaveProject(p *Project, dest string) error {
	abs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", dest, err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", abs, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(abs), ".project-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(p); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode toml: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, abs); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// DefaultProjectPath returns the canonical path under pathDir where a new
// project file should be created. pathDir defaults to "." when empty.
func DefaultProjectPath(pathDir string) string {
	if pathDir == "" {
		pathDir = "."
	}
	return filepath.Join(pathDir, ProjectFileName)
}

// isLegacyName reports whether base matches one of the legacy file names.
// Comparison is case-insensitive to avoid platform surprises.
func isLegacyName(base string) bool {
	lower := strings.ToLower(base)
	for _, name := range LegacyProjectFileNames {
		if lower == strings.ToLower(name) {
			return true
		}
	}
	return false
}
