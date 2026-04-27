package cmd

import (
	"github.com/aonesuite/aone/internal/sandbox/instance"
	"github.com/spf13/cobra"
)

var sandboxCmd = &cobra.Command{
	Use:     "sandbox",
	Aliases: []string{"sbx"},
	Short:   "Manage sandboxes (alias: sbx)",
	GroupID: "sandbox",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func newSandboxListCmd() *cobra.Command {
	info := instance.ListInfo{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sandboxes (alias: ls)",
		Run: func(cmd *cobra.Command, args []string) {
			instance.List(info)
		},
	}
	cmd.Flags().StringVarP(&info.State, "state", "s", "", "filter by state (comma-separated: running,paused). Defaults to running")
	cmd.Flags().StringVarP(&info.Metadata, "metadata", "m", "", "filter by metadata (key1=value1,key2=value2)")
	cmd.Flags().Int32VarP(&info.Limit, "limit", "l", 0, "maximum number of sandboxes to return; 0 uses the server default")
	cmd.Flags().StringVarP(&info.Format, "format", "f", "pretty", "output format: pretty or json")
	return cmd
}

func newSandboxCreateCmd() *cobra.Command {
	info := instance.CreateInfo{}
	cmd := &cobra.Command{
		Use:     "create [template]",
		Aliases: []string{"cr"},
		Short:   "Create a sandbox and connect to its terminal (alias: cr)",
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				info.TemplateID = args[0]
			}
			instance.Create(info)
		},
	}
	cmd.Flags().Int32VarP(&info.Timeout, "timeout", "t", 0, "sandbox timeout in seconds")
	cmd.Flags().BoolVar(&info.Detach, "detach", false, "create sandbox without connecting terminal")
	cmd.Flags().StringVarP(&info.Metadata, "metadata", "m", "", "metadata key=value pairs (comma-separated)")
	cmd.Flags().StringArrayVarP(&info.EnvVars, "env-var", "e", nil, "environment variables (KEY=VALUE, can be specified multiple times)")
	cmd.Flags().BoolVar(&info.AutoPause, "auto-pause", false, "automatically pause sandbox when timeout expires")
	cmd.Flags().StringVar(&info.ConfigPath, "config", "", "path to aone.sandbox.toml (overrides --path lookup)")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "project root used to locate aone.sandbox.toml")
	return cmd
}

func newSandboxConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "connect <sandboxID>",
		Aliases: []string{"cn"},
		Short:   "Connect to an existing sandbox terminal (alias: cn)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			instance.Connect(instance.ConnectInfo{SandboxID: args[0]})
		},
	}
}

func newSandboxKillCmd() *cobra.Command {
	info := instance.KillInfo{}
	cmd := &cobra.Command{
		Use:     "kill [sandboxIDs...]",
		Aliases: []string{"kl"},
		Short:   "Kill one or more sandboxes (alias: kl)",
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxIDs = args
			instance.Kill(info)
		},
	}
	cmd.Flags().BoolVarP(&info.All, "all", "a", false, "kill all sandboxes")
	cmd.Flags().StringVarP(&info.State, "state", "s", "", "filter by state when using --all")
	cmd.Flags().StringVarP(&info.Metadata, "metadata", "m", "", "filter by metadata when using --all")
	return cmd
}

func newSandboxPauseCmd() *cobra.Command {
	info := instance.PauseInfo{}
	cmd := &cobra.Command{
		Use:     "pause [sandboxIDs...]",
		Aliases: []string{"ps"},
		Short:   "Pause one or more sandboxes (alias: ps)",
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxIDs = args
			instance.Pause(info)
		},
	}
	cmd.Flags().BoolVarP(&info.All, "all", "a", false, "pause all sandboxes")
	cmd.Flags().StringVarP(&info.State, "state", "s", "", "filter by state when using --all")
	cmd.Flags().StringVarP(&info.Metadata, "metadata", "m", "", "filter by metadata when using --all")
	return cmd
}

func newSandboxResumeCmd() *cobra.Command {
	info := instance.ResumeInfo{}
	cmd := &cobra.Command{
		Use:     "resume [sandboxIDs...]",
		Aliases: []string{"rs"},
		Short:   "Resume one or more paused sandboxes (alias: rs)",
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxIDs = args
			instance.Resume(info)
		},
	}
	cmd.Flags().BoolVarP(&info.All, "all", "a", false, "resume all paused sandboxes")
	cmd.Flags().StringVarP(&info.Metadata, "metadata", "m", "", "filter by metadata when using --all")
	return cmd
}

func newSandboxExecCmd() *cobra.Command {
	info := instance.ExecInfo{}
	cmd := &cobra.Command{
		Use:     "exec <sandboxID> -- <command...>",
		Aliases: []string{"ex"},
		Short:   "Execute a command in a sandbox (alias: ex)",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxID = args[0]
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				info.Command = args[dash:]
			} else if len(args) > 1 {
				info.Command = args[1:]
			}
			instance.Exec(info)
		},
	}
	cmd.Flags().BoolVarP(&info.Background, "background", "b", false, "run command in background")
	cmd.Flags().StringVarP(&info.Cwd, "cwd", "c", "", "working directory for the command")
	cmd.Flags().StringVarP(&info.User, "user", "u", "", "user to run the command as")
	cmd.Flags().StringArrayVarP(&info.Envs, "env", "e", nil, "environment variables (KEY=VALUE)")
	return cmd
}

func newSandboxLogsCmd() *cobra.Command {
	info := instance.LogsInfo{}
	cmd := &cobra.Command{
		Use:     "logs <sandboxID>",
		Aliases: []string{"lg"},
		Short:   "View sandbox logs (alias: lg)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxID = args[0]
			instance.Logs(info)
		},
	}
	cmd.Flags().StringVar(&info.Level, "level", "INFO", "filter by log level (DEBUG, INFO, WARN, ERROR)")
	cmd.Flags().Int32Var(&info.Limit, "limit", 0, "maximum number of log entries")
	cmd.Flags().StringVar(&info.Format, "format", "pretty", "output format: pretty or json")
	cmd.Flags().BoolVarP(&info.Follow, "follow", "f", false, "keep streaming logs")
	cmd.Flags().StringVar(&info.Loggers, "loggers", "", "filter logs by logger prefixes")
	return cmd
}

func newSandboxMetricsCmd() *cobra.Command {
	info := instance.MetricsInfo{}
	cmd := &cobra.Command{
		Use:     "metrics <sandboxID>",
		Aliases: []string{"mt"},
		Short:   "View sandbox resource metrics (alias: mt)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxID = args[0]
			instance.Metrics(info)
		},
	}
	cmd.Flags().StringVar(&info.Format, "format", "pretty", "output format: pretty or json")
	cmd.Flags().BoolVarP(&info.Follow, "follow", "f", false, "keep streaming metrics")
	return cmd
}

func newSandboxInfoCmd() *cobra.Command {
	info := instance.InfoInfo{}
	cmd := &cobra.Command{
		Use:     "info <sandboxID>",
		Aliases: []string{"in"},
		Short:   "Show information for a sandbox (alias: in)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.SandboxID = args[0]
			instance.Info(info)
		},
	}
	cmd.Flags().StringVarP(&info.Format, "format", "f", "pretty", "output format: pretty or json")
	return cmd
}

func init() {
	sandboxCmd.AddCommand(
		newSandboxListCmd(),
		newSandboxCreateCmd(),
		newSandboxConnectCmd(),
		newSandboxInfoCmd(),
		newSandboxKillCmd(),
		newSandboxPauseCmd(),
		newSandboxResumeCmd(),
		newSandboxExecCmd(),
		newSandboxLogsCmd(),
		newSandboxMetricsCmd(),
		newTemplateCmd(),
		newVolumeCmd(),
	)
	rootCmd.AddCommand(sandboxCmd)
}
