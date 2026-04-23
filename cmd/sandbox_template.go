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
	cmd.AddCommand(
		newTemplateListCmd(),
		newTemplateGetCmd(),
		newTemplateDeleteCmd(),
		newTemplateBuildCmd(),
		newTemplateBuildsCmd(),
		newTemplatePublishCmd(true),
		newTemplatePublishCmd(false),
		newTemplateInitCmd(),
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

func newTemplateBuildCmd() *cobra.Command {
	info := template.BuildInfo{}
	cmd := &cobra.Command{
		Use:     "build",
		Aliases: []string{"bd"},
		Short:   "Build a template (alias: bd)",
		Run: func(cmd *cobra.Command, args []string) {
			template.Build(info)
		},
	}
	cmd.Flags().StringVar(&info.Name, "name", "", "template name (for creating a new template)")
	cmd.Flags().StringVar(&info.TemplateID, "template-id", "", "existing template ID (for rebuilding)")
	cmd.Flags().StringVar(&info.FromImage, "from-image", "", "base Docker image")
	cmd.Flags().StringVar(&info.FromTemplate, "from-template", "", "base template")
	cmd.Flags().StringVar(&info.StartCmd, "start-cmd", "", "command to run after build")
	cmd.Flags().StringVar(&info.ReadyCmd, "ready-cmd", "", "readiness check command")
	cmd.Flags().Int32Var(&info.CPUCount, "cpu", 0, "sandbox CPU count")
	cmd.Flags().Int32Var(&info.MemoryMB, "memory", 0, "sandbox memory size in MiB")
	cmd.Flags().BoolVar(&info.Wait, "wait", false, "wait for build to complete")
	cmd.Flags().BoolVar(&info.NoCache, "no-cache", false, "force full rebuild ignoring cache")
	cmd.Flags().StringVar(&info.Dockerfile, "dockerfile", "", "path to Dockerfile")
	cmd.Flags().StringVar(&info.Path, "path", "", "build context directory")
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
	cmd.Flags().StringVar(&info.Name, "name", "", "template project name")
	cmd.Flags().StringVar(&info.Language, "language", "", "programming language (go, typescript, python)")
	cmd.Flags().StringVar(&info.Path, "path", "", "output directory")
	return cmd
}
