// Package config manages persistent CLI configuration stored in the user's
// home directory.
//
// Two layers of configuration are recognized:
//
//   - User-level credentials (this file): API key + endpoint, persisted in
//     ${AONE_CONFIG_HOME:-~/.config/aone}/config.json. Written by `aone auth`
//     subcommands and consumed by every command that talks to the control
//     plane.
//   - Project-level template metadata: handled by the project subpackage.
//
// File format is intentionally minimal JSON for easy manual editing.
// Permissions are forced to 0600 because the API key is sensitive.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Environment variable names used for configuration overrides.
const (
	// EnvAPIKey is the environment variable holding the API key. It takes
	// priority over any value persisted in the user config file.
	EnvAPIKey = "AONE_API_KEY"
	// EnvEndpoint overrides the control-plane endpoint URL.
	EnvEndpoint = "AONE_SANDBOX_API_URL"
	// EnvDebug enables verbose SDK debug logging when truthy ("1"/"true").
	EnvDebug = "AONE_DEBUG"
	// EnvConfigHome overrides the user-level config directory. Useful in
	// tests so they don't pollute the real ~/.config/aone.
	EnvConfigHome = "AONE_CONFIG_HOME"
)

// configFileName is the file name (within the config home directory) that
// stores user-level credentials.
const configFileName = "config.json"

// File is the on-disk JSON representation of the user-level config file.
//
// All fields are optional; absence means "fall back to env or default".
// The file is owned by the CLI; users may edit it manually but unrecognized
// fields are preserved across writes via Extra.
type File struct {
	// Endpoint is the control-plane base URL (e.g. https://sandbox.aonesuite.com).
	Endpoint string `json:"endpoint,omitempty"`
	// APIKey is the long-lived API key used to authenticate every request.
	APIKey string `json:"apiKey,omitempty"`
	// LastLoginAt records the most recent successful `auth login`. Purely
	// informational, surfaced by `auth info`.
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

// Source identifies where a resolved configuration value originated. Used by
// `aone auth info` to show the user which layer is currently winning.
type Source string

const (
	// SourceFlag means the value came from a CLI flag.
	SourceFlag Source = "flag"
	// SourceEnv means the value came from an environment variable.
	SourceEnv Source = "env"
	// SourceFile means the value came from the user-level config file.
	SourceFile Source = "config"
	// SourceDefault means the value fell back to a built-in default.
	SourceDefault Source = "default"
	// SourceNone means no value was found at all.
	SourceNone Source = "none"
)

// Home returns the directory where the config file lives. Honors
// AONE_CONFIG_HOME for testing; otherwise defaults to ~/.config/aone.
func Home() (string, error) {
	if v := os.Getenv(EnvConfigHome); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	return filepath.Join(home, ".config", "aone"), nil
}

// Path returns the absolute path of the user-level config file.
func Path() (string, error) {
	dir, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads the config file from disk. A missing file is not an error;
// it returns an empty File so callers can treat "no config" and "empty
// config" identically.
func Load() (*File, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f File
	if len(data) == 0 {
		return &f, nil
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &f, nil
}

// Save writes f to the config file with 0600 permissions, creating parent
// directories as needed. The write is atomic: it goes through a temp file
// and rename so a crash mid-write can never leave a half-formed config.
func Save(f *File) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Best-effort cleanup of the temp file if rename fails.
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp config: %w", err)
	}
	return nil
}

// Update loads, mutates, then saves the config file in one shot. The mutator
// receives a pointer so it can edit fields in place; if it returns an error,
// the file is left unchanged.
func Update(mutate func(*File) error) error {
	f, err := Load()
	if err != nil {
		return err
	}
	if err := mutate(f); err != nil {
		return err
	}
	return Save(f)
}
