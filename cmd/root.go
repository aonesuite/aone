package cmd

import (
	"os"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/internal/log"
)

// version is overwritten via -ldflags at release time. The fallback reads
// build info so `go install` users still get a meaningful version string.
var version = "dev"

// debugFlag mirrors AONE_DEBUG via a top-level --debug flag. Equivalent
// to -v: useful when users want the simplest possible knob.
var debugFlag bool

// verbosityFlag is incremented by -v / -vv. 1 → debug, 2+ → trace. Kept
// separate from --debug so both can be present without confusion.
var verbosityFlag int

// rootCmd is the top-level cobra command. Subcommands are added via init()
// in their respective files (auth.go, sandbox.go, …) to keep this file thin.
var rootCmd = &cobra.Command{
	Use:     "aone",
	Short:   "AoneSuite command line tools",
	Long:    "AoneSuite CLI — manage sandboxes, templates, and account credentials.",
	Version: resolveVersion(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Setting AONE_DEBUG here means anything that constructs an SDK
		// client during command execution sees the same value, regardless
		// of whether the user passed --debug or exported AONE_DEBUG.
		if debugFlag {
			_ = os.Setenv(config.EnvDebug, "1")
		}

		// Initialize the global structured logger. ResolveLevel handles
		// the precedence (AONE_LOG_LEVEL > -v/-vv > AONE_DEBUG > --debug
		// > default-silent), so we just hand it both inputs.
		log.Init(log.InitOptions{
			ResolveOptions: log.ResolveOptions{
				DebugFlag: debugFlag,
				Verbosity: verbosityFlag,
			},
		})

		// One-time startup banner at DEBUG so triage tickets carry the
		// environment context we'd otherwise have to ask the user for.
		log.Debug("aone cli startup",
			"version", resolveVersion(),
			"go", runtime.Version(),
			"os", runtime.GOOS,
			"arch", runtime.GOARCH,
			"command", cmd.CommandPath(),
		)
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
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug logging (equivalent to -v / AONE_DEBUG=1)")
	rootCmd.PersistentFlags().CountVarP(&verbosityFlag, "verbose", "v", "increase log verbosity (-v debug, -vv trace); writes to stderr")
}
