package volume

import (
	"context"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// RmInfo holds parameters for deleting a path inside a volume.
type RmInfo struct {
	// VolumeID identifies the volume.
	VolumeID string

	// Path is the file or directory to remove.
	Path string
}

// Rm removes a file or directory inside a volume.
func Rm(info RmInfo) {
	if info.VolumeID == "" {
		sbClient.PrintError("volume ID is required")
		return
	}
	if info.Path == "" {
		sbClient.PrintError("path is required")
		return
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

	if err := vol.Remove(ctx, info.Path); err != nil {
		sbClient.PrintError("remove failed: %v", err)
		return
	}
	sbClient.PrintSuccess("Removed %s", info.Path)
}
