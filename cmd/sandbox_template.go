package cmd

import (
	"github.com/aonesuite/aone/internal/sandbox/template"
	"github.com/spf13/cobra"
)

func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "template",
		Aliases: []string{"tpl"},
		Short:   "Manage sandbox templates (alias: tpl)",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	// `build` is kept as an internal low-level command (hidden) so the
	// primary surface stays focused: `create` drives Dockerfile builds and
	// `migrate` converts Dockerfiles to SDK template code.
	buildCmd := newTemplateBuildCmd()
	buildCmd.Hidden = true

	cmd.AddCommand(
		newTemplateListCmd(),
		newTemplateGetCmd(),
		newTemplateCreateCmd(),
		newTemplateDeleteCmd(),
		buildCmd,
		newTemplateBuildsCmd(),
		newTemplateLogsCmd(),
		newTemplatePublishCmd(true),
		newTemplatePublishCmd(false),
		newTemplateInitCmd(),
		newTemplateMigrateCmd(),
	)
	return cmd
}

func newTemplateListCmd() *cobra.Command {
	info := template.ListInfo{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sandbox templates (alias: ls)",
		Run: func(cmd *cobra.Command, args []string) {
			template.List(info)
		},
	}
	cmd.Flags().StringVar(&info.Format, "format", "pretty", "output format: pretty or json")
	cmd.Flags().StringVar(&info.Name, "name", "", "filter by template name")
	cmd.Flags().StringVar(&info.BuildStatus, "build-status", "", "filter by latest build status")
	cmd.Flags().StringVar(&info.Public, "public", "", "filter by visibility (true or false)")
	cmd.Flags().StringVar(&info.Cursor, "cursor", "", "pagination cursor returned by the API")
	cmd.Flags().Int32Var(&info.Limit, "limit", 0, "maximum number of templates to return")
	return cmd
}

func newTemplateGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <templateID>",
		Aliases: []string{"gt"},
		Short:   "Get template details (alias: gt)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			template.Get(template.GetInfo{TemplateID: args[0]})
		},
	}
}

func newTemplateDeleteCmd() *cobra.Command {
	info := template.DeleteInfo{}
	cmd := &cobra.Command{
		Use:     "delete [templateIDs...]",
		Aliases: []string{"dl"},
		Short:   "Delete one or more templates (alias: dl)",
		Run: func(cmd *cobra.Command, args []string) {
			info.TemplateIDs = args
			template.Delete(info)
		},
	}
	cmd.Flags().BoolVarP(&info.Yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVarP(&info.Select, "select", "s", false, "interactively select templates")
	cmd.Flags().StringVar(&info.ConfigPath, "config", "", "path to aone.sandbox.toml (overrides --path lookup)")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "project root used to locate aone.sandbox.toml")
	return cmd
}

func newTemplateBuildsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "builds <templateID> <buildID>",
		Aliases: []string{"bds"},
		Short:   "View template build status (alias: bds)",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			template.Builds(template.BuildsInfo{TemplateID: args[0], BuildID: args[1]})
		},
	}
}

func newTemplateLogsCmd() *cobra.Command {
	info := template.LogsInfo{}
	cmd := &cobra.Command{
		Use:     "logs <templateID> <buildID>",
		Aliases: []string{"blg"},
		Short:   "View template build logs (alias: blg)",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			info.TemplateID = args[0]
			info.BuildID = args[1]
			template.Logs(info)
		},
	}
	cmd.Flags().Int64Var(&info.Cursor, "cursor", 0, "build log cursor")
	cmd.Flags().Int32Var(&info.Limit, "limit", 0, "maximum number of log entries")
	cmd.Flags().StringVar(&info.Direction, "direction", "", "pagination direction (forward or backward)")
	cmd.Flags().StringVar(&info.Level, "level", "", "filter by log level")
	cmd.Flags().StringVar(&info.Source, "source", "", "filter by log source")
	cmd.Flags().StringVar(&info.Format, "format", "pretty", "output format: pretty or json")
	return cmd
}

func newTemplateBuildCmd() *cobra.Command {
	info := template.BuildInfo{SaveConfig: true}
	cmd := &cobra.Command{
		Use:     "build",
		Aliases: []string{"bd"},
		Short:   "Build a template (alias: bd)",
		Run: func(cmd *cobra.Command, args []string) {
			template.Build(info)
		},
	}
	cmd.Flags().StringVar(&info.Name, "name", "", "template name (for creating a new template)")
	cmd.Flags().StringVar(&info.FromImage, "from-image", "", "base Docker image")
	cmd.Flags().StringVar(&info.FromTemplate, "from-template", "", "base template")
	cmd.Flags().StringVar(&info.StartCmd, "start-cmd", "", "command to run after build")
	cmd.Flags().StringVar(&info.ReadyCmd, "ready-cmd", "", "readiness check command")
	cmd.Flags().Int32Var(&info.CPUCount, "cpu", 0, "sandbox CPU count")
	cmd.Flags().Int32Var(&info.MemoryMB, "memory", 0, "sandbox memory size in MiB")
	cmd.Flags().Int32Var(&info.DiskSizeMB, "disk-size-mb", 0, "sandbox disk size in MiB")
	cmd.Flags().StringVar(&info.Public, "public", "", "set template visibility (true or false)")
	cmd.Flags().BoolVar(&info.Wait, "wait", false, "wait for build to complete")
	cmd.Flags().StringVar(&info.Dockerfile, "dockerfile", "", "path to Dockerfile")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "project root (defaults to current directory)")
	cmd.Flags().StringVar(&info.ConfigPath, "config", "", "path to aone.sandbox.toml (overrides --path lookup)")
	return cmd
}

func newTemplatePublishCmd(public bool) *cobra.Command {
	info := template.PublishInfo{Public: public}
	use, alias, short := "publish [templateIDs...]", "pb", "Publish templates"
	if !public {
		use, alias, short = "unpublish [templateIDs...]", "upb", "Unpublish templates"
	}
	cmd := &cobra.Command{
		Use:     use,
		Aliases: []string{alias},
		Short:   short,
		Run: func(cmd *cobra.Command, args []string) {
			info.TemplateIDs = args
			template.Publish(info)
		},
	}
	cmd.Flags().BoolVarP(&info.Yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVarP(&info.Select, "select", "s", false, "interactively select templates")
	cmd.Flags().StringVar(&info.ConfigPath, "config", "", "path to aone.sandbox.toml (overrides --path lookup)")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "project root used to locate aone.sandbox.toml")
	return cmd
}

func newTemplateInitCmd() *cobra.Command {
	info := template.InitInfo{}
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"it"},
		Short:   "Initialize a new template project (alias: it)",
		Run: func(cmd *cobra.Command, args []string) {
			template.Init(info)
		},
	}
	cmd.Flags().StringVarP(&info.Name, "name", "n", "", "template project name")
	cmd.Flags().StringVarP(&info.Language, "language", "l", "", "programming language (go, typescript, python)")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "output directory")
	return cmd
}

// newTemplateCreateCmd builds a Dockerfile as a sandbox template directly.
// The template name is the only required positional argument; Dockerfile and
// resource options are provided via flags. After a successful build the
// resolved template_id is written back to aone.sandbox.toml under --path
// (or the directory containing the Dockerfile).
func newTemplateCreateCmd() *cobra.Command {
	info := template.BuildInfo{SaveConfig: true}
	cmd := &cobra.Command{
		Use:     "create <template-name>",
		Aliases: []string{"ct"},
		Short:   "Build a Dockerfile as a sandbox template (alias: ct)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.Name = args[0]
			template.Create(info)
		},
	}
	cmd.Flags().StringVarP(&info.Dockerfile, "dockerfile", "d", "", "path to Dockerfile (defaults to aone.Dockerfile or Dockerfile)")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "build context directory (defaults to Dockerfile directory)")
	cmd.Flags().StringVar(&info.ConfigPath, "config", "", "path to aone.sandbox.toml (overrides --path lookup)")
	cmd.Flags().StringVarP(&info.StartCmd, "cmd", "c", "", "command executed when the sandbox starts")
	cmd.Flags().StringVar(&info.ReadyCmd, "ready-cmd", "", "readiness check command")
	cmd.Flags().Int32Var(&info.CPUCount, "cpu-count", 0, "sandbox CPU count")
	cmd.Flags().Int32Var(&info.MemoryMB, "memory-mb", 0, "sandbox memory in MiB")
	cmd.Flags().Int32Var(&info.DiskSizeMB, "disk-size-mb", 0, "sandbox disk size in MiB")
	cmd.Flags().StringVar(&info.Public, "public", "", "set template visibility (true or false)")
	return cmd
}

// newTemplateMigrateCmd converts a Dockerfile into SDK-native template code
// for Go / TypeScript / Python.
func newTemplateMigrateCmd() *cobra.Command {
	info := template.MigrateInfo{}
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate a Dockerfile to SDK-native template code",
		Run: func(cmd *cobra.Command, args []string) {
			template.Migrate(info)
		},
	}
	cmd.Flags().StringVarP(&info.Dockerfile, "dockerfile", "d", "", "path to Dockerfile (defaults to aone.Dockerfile or Dockerfile)")
	cmd.Flags().StringVarP(&info.Path, "path", "p", "", "project root (defaults to current directory)")
	cmd.Flags().StringVar(&info.ConfigPath, "config", "", "path to aone.sandbox.toml (overrides --path lookup)")
	cmd.Flags().StringVarP(&info.Language, "language", "l", "", "target language: go, typescript, python")
	cmd.Flags().StringVar(&info.Name, "name", "", "template name used in generated code (defaults to directory name)")
	return cmd
}
