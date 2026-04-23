package cmd

import (
	"github.com/spf13/cobra"

	"github.com/aonesuite/aone/internal/sandbox/volume"
)

func newVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "volume",
		Aliases: []string{"vol"},
		Short:   "Manage persistent volumes (alias: vol)",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
	cmd.AddCommand(
		newVolumeListCmd(),
		newVolumeCreateCmd(),
		newVolumeInfoCmd(),
		newVolumeDeleteCmd(),
		newVolumeLsCmd(),
		newVolumeCatCmd(),
		newVolumeCpCmd(),
		newVolumeRmCmd(),
		newVolumeMkdirCmd(),
	)
	return cmd
}

func newVolumeListCmd() *cobra.Command {
	info := volume.ListInfo{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List volumes (alias: ls)",
		Run: func(cmd *cobra.Command, args []string) {
			volume.List(info)
		},
	}
	cmd.Flags().StringVarP(&info.Format, "format", "f", "pretty", "output format: pretty or json")
	return cmd
}

func newVolumeCreateCmd() *cobra.Command {
	info := volume.CreateInfo{}
	cmd := &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"cr"},
		Short:   "Create a new persistent volume (alias: cr)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.Name = args[0]
			volume.Create(info)
		},
	}
	cmd.Flags().StringVarP(&info.Format, "format", "f", "pretty", "output format: pretty or json")
	return cmd
}

func newVolumeInfoCmd() *cobra.Command {
	info := volume.InfoInfo{}
	cmd := &cobra.Command{
		Use:     "info <volumeID>",
		Aliases: []string{"in"},
		Short:   "Show volume metadata (alias: in)",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeID = args[0]
			volume.Info(info)
		},
	}
	cmd.Flags().StringVarP(&info.Format, "format", "f", "pretty", "output format: pretty or json")
	return cmd
}

func newVolumeDeleteCmd() *cobra.Command {
	info := volume.DeleteInfo{}
	cmd := &cobra.Command{
		Use:     "delete [volumeIDs...]",
		Aliases: []string{"dl"},
		Short:   "Delete one or more volumes (alias: dl)",
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeIDs = args
			volume.Delete(info)
		},
	}
	cmd.Flags().BoolVarP(&info.Yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVar(&info.Select, "select", false, "interactively select volumes to delete")
	return cmd
}

func newVolumeLsCmd() *cobra.Command {
	info := volume.LsInfo{}
	cmd := &cobra.Command{
		Use:   "ls <volumeID> [path]",
		Short: "List entries inside a volume",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeID = args[0]
			if len(args) > 1 {
				info.Path = args[1]
			}
			volume.Ls(info)
		},
	}
	cmd.Flags().Int32VarP(&info.Depth, "depth", "d", 0, "recursion depth (0 = server default)")
	cmd.Flags().StringVarP(&info.Format, "format", "f", "pretty", "output format: pretty or json")
	return cmd
}

func newVolumeCatCmd() *cobra.Command {
	info := volume.CatInfo{}
	return &cobra.Command{
		Use:   "cat <volumeID> <path>",
		Short: "Print a volume file's contents to stdout",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeID = args[0]
			info.Path = args[1]
			volume.Cat(info)
		},
	}
}

func newVolumeCpCmd() *cobra.Command {
	info := volume.CpInfo{}
	return &cobra.Command{
		Use:   "cp <volumeID> <source> <destination>",
		Short: "Copy files between local disk and a volume (prefix remote with 'volume:')",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeID = args[0]
			info.Source = args[1]
			info.Destination = args[2]
			volume.Cp(info)
		},
	}
}

func newVolumeRmCmd() *cobra.Command {
	info := volume.RmInfo{}
	return &cobra.Command{
		Use:   "rm <volumeID> <path>",
		Short: "Remove a file or directory inside a volume",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeID = args[0]
			info.Path = args[1]
			volume.Rm(info)
		},
	}
}

func newVolumeMkdirCmd() *cobra.Command {
	info := volume.MkdirInfo{}
	cmd := &cobra.Command{
		Use:   "mkdir <volumeID> <path>",
		Short: "Create a directory inside a volume",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			info.VolumeID = args[0]
			info.Path = args[1]
			volume.Mkdir(info)
		},
	}
	cmd.Flags().BoolVarP(&info.Force, "force", "F", false, "create parent directories as needed")
	return cmd
}
