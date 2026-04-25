package cmd

import (
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/aonesuite/aone/internal/config"
)

// version is overwritten via -ldflags at release time. The fallback reads
// build info so `go install` users still get a meaningful version string.
var version = "dev"

// debugFlag mirrors AONE_DEBUG via a top-level --debug flag. The flag is
// applied early in PersistentPreRun so subcommands building SDK clients pick
// it up through the standard env-driven path.
var debugFlag bool

// rootCmd is the top-level cobra command. Subcommands are added via init()
// in their respective files (auth.go, sandbox.go, …) to keep this file thin.
var rootCmd = &cobra.Command{
	Use:     "aone",
	Short:   "AoneSuite command line tools",
	Long:    "AoneSuite CLI — manage sandboxes, templates, volumes, and account credentials.",
	Version: resolveVersion(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Setting AONE_DEBUG here means anything that constructs an SDK
		// client during command execution sees the same value, regardless
		// of whether the user passed --debug or exported AONE_DEBUG.
		if debugFlag {
			_ = os.Setenv(config.EnvDebug, "1")
		}
	},
}

// resolveVersion returns the linker-injected version when present, otherwise
// falls back to module build info (so `go install` reports something useful).
func resolveVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// Execute runs the root Cobra command and lets Cobra handle argument parsing,
// command dispatch, and error presentation for the CLI process.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Group declarations let `aone --help` show subcommands in logical
	// sections (core / sandbox) instead of one flat alphabetical list.
	rootCmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Account & configuration:"},
		&cobra.Group{ID: "sandbox", Title: "Sandbox management:"},
	)
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable SDK debug logging (equivalent to AONE_DEBUG=1)")
}
