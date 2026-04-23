// Package volume implements the `aone sandbox volume` CLI subcommands. It
// wraps the persistent volume management APIs from packages/go/sandbox so the
// CLI can list, create, inspect, and modify volumes plus their contents.
package volume

import (
	"context"
	"fmt"
	"os"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// ListInfo holds parameters for listing volumes.
type ListInfo struct {
	// Format selects the output format. See sbClient.FormatPretty / FormatJSON.
	Format string
}

// List prints all persistent volumes visible to the authenticated caller.
func List(info ListInfo) {
	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	volumes, err := client.ListVolumes(context.Background())
	if err != nil {
		sbClient.PrintError("list volumes failed: %v", err)
		return
	}

	if info.Format == sbClient.FormatJSON {
		sbClient.PrintJSON(volumes)
		return
	}

	if len(volumes) == 0 {
		fmt.Println("No volumes found")
		return
	}

	tw := sbClient.NewTable(os.Stdout)
	fmt.Fprintf(tw, "VOLUME ID\tNAME\n")
	for _, v := range volumes {
		fmt.Fprintf(tw, "%s\t%s\n", v.VolumeID, v.Name)
	}
	tw.Flush()
}
