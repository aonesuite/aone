package volume

import (
	"context"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
	"github.com/aonesuite/aone/packages/go/sandbox"
)

// MkdirInfo holds parameters for creating a directory inside a volume.
type MkdirInfo struct {
	// VolumeID identifies the volume.
	VolumeID string

	// Path is the directory to create.
	Path string

	// Force creates parent directories as needed.
	Force bool
}

// Mkdir creates a directory inside a volume.
func Mkdir(info MkdirInfo) {
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

	var opts *sandbox.VolumeWriteOptions
	if info.Force {
		force := true
		opts = &sandbox.VolumeWriteOptions{Force: &force}
	}
	if _, err := vol.MakeDir(ctx, info.Path, opts); err != nil {
		sbClient.PrintError("mkdir failed: %v", err)
		return
	}
	sbClient.PrintSuccess("Created %s", info.Path)
}
