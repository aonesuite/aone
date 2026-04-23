package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aone",
	Short: "AoneSuite command line tools",
}

// Execute runs the root Cobra command and lets Cobra handle argument parsing,
// command dispatch, and error presentation for the CLI process.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
