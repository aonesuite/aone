package volume

import (
	"context"
	"fmt"
	"os"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// CatInfo holds parameters for reading a volume file.
type CatInfo struct {
	// VolumeID identifies the volume.
	VolumeID string

	// Path is the file to read.
	Path string
}

// Cat prints the contents of a file inside a volume to stdout.
func Cat(info CatInfo) {
	if info.VolumeID == "" {
		sbClient.PrintError("volume ID is required")
		return
	}
	if info.Path == "" {
		sbClient.PrintError("file path is required")
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

	data, err := vol.ReadFile(ctx, info.Path)
	if err != nil {
		sbClient.PrintError("read file failed: %v", err)
		return
	}

	if _, err := os.Stdout.Write(data); err != nil {
		sbClient.PrintError("write stdout failed: %v", err)
		return
	}
	// Ensure trailing newline when the file doesn't end with one, so subsequent
	// shell prompts aren't glued onto the last line.
	if len(data) == 0 || data[len(data)-1] != '\n' {
		fmt.Println()
	}
}
