package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/aonesuite/aone/internal/config"
	internalsbx "github.com/aonesuite/aone/internal/sandbox"
)

// authCmd is the parent group for credential management commands. All
// subcommands operate on the user-level config file at
// ${AONE_CONFIG_HOME:-~/.config/aone}/config.json.
var authCmd = &cobra.Command{
	Use:     "auth",
	Short:   "Manage CLI authentication and credentials",
	GroupID: "core",
}

// authLoginAPIKey is bound to `--api-key`; if empty, login prompts interactively.
var authLoginAPIKey string

// authLoginEndpoint is bound to `--endpoint`; allows overriding the saved
// control-plane URL during login (e.g. for staging).
var authLoginEndpoint string

// authLoginNoVerify skips the post-login verification request. Useful in
// air-gapped environments where the user wants to persist a key without an
// outbound API call.
var authLoginNoVerify bool

// authLoginCmd writes API credentials to the user config file.
//
// Behavior matches the design from docs/cli-alignment-plan.md:
//   - --api-key wins; otherwise prompt (TTY → silent input, pipe → read line)
//   - --endpoint optional, defaults to whatever the resolver picks
//   - by default we hit the API once with the new key to catch typos early
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save an API key to the local credential store",
	Long: `Save an API key to the local credential store.

Provide the key via --api-key, or omit the flag to be prompted (input is
hidden when stdin is a terminal). Credentials are written to
${AONE_CONFIG_HOME:-~/.config/aone}/config.json with mode 0600.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		key := strings.TrimSpace(authLoginAPIKey)
		if key == "" {
			prompted, err := promptAPIKey()
			if err != nil {
				return err
			}
			key = prompted
		}
		if key == "" {
			return errors.New("API key is required")
		}

		// Persist first so a verification failure can still leave the user
		// with a recoverable state via `auth info` / `auth logout`.
		now := time.Now().UTC()
		if err := config.Update(func(f *config.File) error {
			f.APIKey = key
			if authLoginEndpoint != "" {
				f.Endpoint = strings.TrimRight(authLoginEndpoint, "/")
			}
			f.LastLoginAt = &now
			return nil
		}); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}

		path, _ := config.Path()
		internalsbx.PrintSuccess("Saved credentials to %s", path)

		if authLoginNoVerify {
			return nil
		}
		if err := verifyCredentials(); err != nil {
			internalsbx.PrintWarn("Credentials saved, but verification failed: %v", err)
			internalsbx.PrintWarn("Run `aone auth info` to inspect, or `aone auth logout` to clear.")
			// Non-fatal: the key is saved; user may be offline.
			return nil
		}
		internalsbx.PrintSuccess("Credentials verified.")
		return nil
	},
}

// authLogoutCmd clears the saved API key. Endpoint and other fields are
// preserved so the user doesn't have to re-set their staging URL after a
// rotation.
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the saved API key",
	RunE: func(cmd *cobra.Command, _ []string) error {
		f, err := config.Load()
		if err != nil {
			return err
		}
		if f.APIKey == "" {
			internalsbx.PrintWarn("No saved API key found.")
			return nil
		}
		f.APIKey = ""
		f.LastLoginAt = nil
		if err := config.Save(f); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		internalsbx.PrintSuccess("Logged out.")
		return nil
	},
}

// authInfoCmd reports the resolved credentials and where each value came
// from. It mirrors the `flag > env > config > default` precedence so users
// can debug "why is the wrong key being used?" at a glance.
var authInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show the active credentials and their sources",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// No flag overrides here: `info` reflects the ambient state.
		resolved, err := config.Resolver{}.Resolve()
		if err != nil {
			return err
		}
		path, _ := config.Path()
		f, _ := config.Load()

		fmt.Printf("Config file:   %s\n", path)
		if resolved.APIKey == "" {
			fmt.Printf("API key:       <not set>\n")
		} else {
			fmt.Printf("API key:       %s  (source: %s)\n", internalsbx.MaskAPIKey(resolved.APIKey), resolved.APIKeySource)
		}
		fmt.Printf("Endpoint:      %s  (source: %s)\n", resolved.Endpoint, resolved.EndpointSource)
		if f != nil && f.LastLoginAt != nil {
			fmt.Printf("Last login at: %s\n", f.LastLoginAt.Format(time.RFC3339))
		}
		return nil
	},
}

// authConfigureFlagAPIKey / authConfigureFlagEndpoint are bound to the
// non-interactive form of `auth configure`. When both are empty the command
// drops into an interactive prompt.
var (
	authConfigureFlagAPIKey   string
	authConfigureFlagEndpoint string
)

// authConfigureCmd lets the user adjust persisted credentials in place.
// Unlike `login` it does not require an API key (you can update only the
// endpoint, for instance).
var authConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Interactively edit saved credentials",
	RunE: func(cmd *cobra.Command, _ []string) error {
		f, err := config.Load()
		if err != nil {
			return err
		}

		// Non-interactive path: at least one flag was supplied.
		if authConfigureFlagAPIKey != "" || authConfigureFlagEndpoint != "" {
			if authConfigureFlagAPIKey != "" {
				f.APIKey = authConfigureFlagAPIKey
				now := time.Now().UTC()
				f.LastLoginAt = &now
			}
			if authConfigureFlagEndpoint != "" {
				f.Endpoint = strings.TrimRight(authConfigureFlagEndpoint, "/")
			}
			if err := config.Save(f); err != nil {
				return err
			}
			internalsbx.PrintSuccess("Configuration updated.")
			return nil
		}

		// Interactive path: prompt with current values as defaults.
		fmt.Printf("Endpoint [%s]: ", coalesce(f.Endpoint, config.DefaultEndpoint))
		newEndpoint, err := readLine()
		if err != nil {
			return err
		}
		if newEndpoint = strings.TrimSpace(newEndpoint); newEndpoint != "" {
			f.Endpoint = strings.TrimRight(newEndpoint, "/")
		}

		fmt.Printf("API key (leave empty to keep current): ")
		key, err := readSecretOrLine()
		if err != nil {
			return err
		}
		if key = strings.TrimSpace(key); key != "" {
			f.APIKey = key
			now := time.Now().UTC()
			f.LastLoginAt = &now
		}

		if err := config.Save(f); err != nil {
			return err
		}
		internalsbx.PrintSuccess("Configuration updated.")
		return nil
	},
}

// promptAPIKey reads an API key from stdin. When stdin is a TTY the input
// is hidden; when piped the line is read normally.
func promptAPIKey() (string, error) {
	fmt.Fprint(os.Stderr, "API key: ")
	v, err := readSecretOrLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v), nil
}

// readSecretOrLine returns hidden input for terminals, or a plain line read
// for piped/redirected stdin (tests, scripts).
func readSecretOrLine() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // term.ReadPassword swallows the newline
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return readLine()
}

// readLine reads a single line (without trailing newline) from stdin.
func readLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// coalesce returns the first non-empty argument; used as a tiny prompt helper.
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// verifyCredentials makes a cheap call against the control plane to confirm
// the saved key works. We use `GET /sandboxes?limit=1` because it exists on
// every deployment and returns quickly.
func verifyCredentials() error {
	resolved, err := config.Resolver{}.Resolve()
	if err != nil {
		return err
	}
	if resolved.APIKey == "" {
		return errors.New("no API key resolved")
	}
	url := strings.TrimRight(resolved.Endpoint, "/") + "/sandboxes?limit=1"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", resolved.APIKey)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication rejected (HTTP %d)", resp.StatusCode)
	}
	return fmt.Errorf("verification call returned HTTP %d", resp.StatusCode)
}

func init() {
	authLoginCmd.Flags().StringVar(&authLoginAPIKey, "api-key", "", "API key to save (omit to be prompted)")
	authLoginCmd.Flags().StringVar(&authLoginEndpoint, "endpoint", "", "control-plane endpoint to save (optional)")
	authLoginCmd.Flags().BoolVar(&authLoginNoVerify, "no-verify", false, "skip the post-save verification request")

	authConfigureCmd.Flags().StringVar(&authConfigureFlagAPIKey, "api-key", "", "set API key non-interactively")
	authConfigureCmd.Flags().StringVar(&authConfigureFlagEndpoint, "endpoint", "", "set endpoint non-interactively")

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authInfoCmd, authConfigureCmd)
	rootCmd.AddCommand(authCmd)
}
