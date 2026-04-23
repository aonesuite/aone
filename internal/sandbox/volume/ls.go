package volume

import (
	"context"
	"fmt"
	"os"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// LsInfo holds parameters for listing files inside a volume.
type LsInfo struct {
	// VolumeID identifies the volume.
	VolumeID string

	// Path is the directory to list. Defaults to "/".
	Path string

	// Depth limits recursion depth. Zero means server default.
	Depth int32

	// Format selects the output format.
	Format string
}

// Ls prints the entries under a volume path.
func Ls(info LsInfo) {
	if info.VolumeID == "" {
		sbClient.PrintError("volume ID is required")
		return
	}
	if info.Path == "" {
		info.Path = "/"
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	ctx := context.Background()
	vol, err := client.ConnectVolume(ctx, info.VolumeID)
	if err != nil {
		sbClient.PrintError("connect volume failed: %v", err)
		return
	}

	var depth *int32
	if info.Depth > 0 {
		depth = &info.Depth
	}
	entries, err := vol.List(ctx, info.Path, depth)
	if err != nil {
		sbClient.PrintError("list failed: %v", err)
		return
	}

	if info.Format == sbClient.FormatJSON {
		sbClient.PrintJSON(entries)
		return
	}

	if len(entries) == 0 {
		fmt.Println("No entries found")
		return
	}

	tw := sbClient.NewTable(os.Stdout)
	fmt.Fprintf(tw, "TYPE\tSIZE\tMODE\tNAME\n")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%d\t%o\t%s\n", e.Type, e.Size, e.Mode, e.Path)
	}
	tw.Flush()
}
