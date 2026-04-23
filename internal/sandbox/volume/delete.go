package volume

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// DeleteInfo holds parameters for deleting volumes.
type DeleteInfo struct {
	// VolumeIDs is the list of volume IDs to delete.
	VolumeIDs []string

	// Yes skips the confirmation prompt.
	Yes bool

	// Select shows an interactive multi-select for choosing volumes.
	Select bool
}

// Delete destroys one or more persistent volumes.
func Delete(info DeleteInfo) {
	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	ctx := context.Background()
	volumeIDs := info.VolumeIDs

	if info.Select {
		volumes, lErr := client.ListVolumes(ctx)
		if lErr != nil {
			sbClient.PrintError("list volumes failed: %v", lErr)
			return
		}
		if len(volumes) == 0 {
			fmt.Println("No volumes found")
			return
		}

		options := make([]huh.Option[string], 0, len(volumes))
		for _, v := range volumes {
			options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", v.VolumeID, v.Name), v.VolumeID))
		}

		var selected []string
		form := huh.NewForm(huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select volumes to delete").
				Options(options...).
				Value(&selected),
		))
		if fErr := form.Run(); fErr != nil {
			sbClient.PrintError("selection cancelled: %v", fErr)
			return
		}
		if len(selected) == 0 {
			fmt.Println("No volumes selected")
			return
		}
		volumeIDs = selected
	}

	if len(volumeIDs) == 0 {
		sbClient.PrintError("at least one volume ID is required (or use --select)")
		return
	}

	if !info.Yes {
		fmt.Printf("Are you sure you want to delete %d volume(s)? [y/N] ", len(volumeIDs))
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Aborted")
			return
		}
	}

	for _, id := range volumeIDs {
		ok, dErr := client.DestroyVolume(ctx, id)
		switch {
		case dErr != nil:
			sbClient.PrintError("delete volume %s failed: %v", id, dErr)
		case !ok:
			sbClient.PrintWarn("volume %s not found", id)
		default:
			sbClient.PrintSuccess("Volume %s deleted", id)
		}
	}
}
