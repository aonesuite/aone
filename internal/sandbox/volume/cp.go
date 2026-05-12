package volume

import (
	"context"
	"os"
	"strings"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// CpInfo holds parameters for copying files between local disk and a volume.
// Directions follow scp semantics:
//   - upload:   local path -> volume:remote
//   - download: volume:remote -> local path
//
// A source or destination with the "volume:" prefix (or matching the info's
// VolumeID) designates the remote side; exactly one side must be remote.
type CpInfo struct {
	// VolumeID identifies the volume (used when Source/Destination omit prefixes).
	VolumeID string

	// Source is the copy source.
	Source string

	// Destination is the copy destination.
	Destination string
}

// Cp uploads a local file to a volume or downloads a volume file locally.
func Cp(info CpInfo) {
	if info.Source == "" || info.Destination == "" {
		sbClient.PrintError("source and destination are required")
		return
	}

	srcRemote, srcPath := parseRemoteRef(info.Source)
	dstRemote, dstPath := parseRemoteRef(info.Destination)

	if srcRemote == dstRemote {
		sbClient.PrintError("exactly one of source/destination must reference the volume (prefix with 'volume:')")
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

	if srcRemote {
		data, rErr := vol.ReadFile(ctx, srcPath)
		if rErr != nil {
			sbClient.PrintError("read volume file failed: %v", rErr)
			return
		}
		if wErr := os.WriteFile(dstPath, data, 0o644); wErr != nil {
			sbClient.PrintError("write local file failed: %v", wErr)
			return
		}
		sbClient.PrintSuccess("Downloaded %s to %s", srcPath, dstPath)
		return
	}

	data, rErr := os.ReadFile(srcPath)
	if rErr != nil {
		sbClient.PrintError("read local file failed: %v", rErr)
		return
	}
	if _, wErr := vol.WriteFile(ctx, dstPath, data, nil); wErr != nil {
		sbClient.PrintError("write volume file failed: %v", wErr)
		return
	}
	sbClient.PrintSuccess("Uploaded %s to %s", srcPath, dstPath)
}

// parseRemoteRef strips the "volume:" prefix from a copy endpoint and reports
// whether the endpoint targets the volume side.
func parseRemoteRef(ref string) (bool, string) {
	if rest, ok := strings.CutPrefix(ref, "volume:"); ok {
		return true, rest
	}
	return false, ref
}
