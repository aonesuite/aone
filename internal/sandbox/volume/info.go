package volume

import (
	"context"
	"fmt"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// InfoInfo holds parameters for getting volume details.
type InfoInfo struct {
	// VolumeID identifies the volume to inspect.
	VolumeID string

	// Format selects the output format.
	Format string
}

// Info prints metadata for a single volume.
func Info(info InfoInfo) {
	if info.VolumeID == "" {
		sbClient.PrintError("volume ID is required")
		return
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	v, err := client.GetVolumeInfo(context.Background(), info.VolumeID)
	if err != nil {
		sbClient.PrintError("get volume info failed: %v", err)
		return
	}

	if info.Format == sbClient.FormatJSON {
		sbClient.PrintJSON(map[string]string{"volumeID": v.VolumeID, "name": v.Name})
		return
	}

	fmt.Printf("Volume ID: %s\n", v.VolumeID)
	fmt.Printf("Name:      %s\n", v.Name)
}
