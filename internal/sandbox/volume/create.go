package volume

import (
	"context"
	"fmt"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// CreateInfo holds parameters for creating a volume.
type CreateInfo struct {
	// Name is the human-readable volume name.
	Name string

	// Format selects the output format for the created volume's metadata.
	Format string
}

// Create creates a new persistent volume.
func Create(info CreateInfo) {
	if info.Name == "" {
		sbClient.PrintError("volume name is required")
		return
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	v, err := client.CreateVolume(context.Background(), info.Name)
	if err != nil {
		sbClient.PrintError("create volume failed: %v", err)
		return
	}

	if info.Format == sbClient.FormatJSON {
		sbClient.PrintJSON(map[string]string{"volumeID": v.VolumeID, "name": v.Name})
		return
	}

	sbClient.PrintSuccess("Volume %s created", v.VolumeID)
	fmt.Printf("Volume ID: %s\n", v.VolumeID)
	fmt.Printf("Name:      %s\n", v.Name)
}
