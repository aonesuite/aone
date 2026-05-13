package template

import (
	"context"
	"fmt"
	"time"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// GetInfo holds parameters for getting template details.
type GetInfo struct {
	TemplateID string
}

// Get retrieves and displays template details.
func Get(info GetInfo) {
	if info.TemplateID == "" {
		sbClient.PrintError("template ID is required")
		return
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	tmpl, err := client.GetTemplate(context.Background(), info.TemplateID)
	if err != nil {
		sbClient.PrintError("get template failed: %v", err)
		return
	}

	fmt.Printf("Template ID:    %s\n", tmpl.TemplateID)
	fmt.Printf("Aliases:        %v\n", tmpl.Aliases)
	fmt.Printf("Names:          %v\n", tmpl.Names)
	fmt.Printf("Public:         %v\n", tmpl.Public)
	fmt.Printf("Source:         %s\n", tmpl.Source)
	fmt.Printf("Editable:       %v\n", tmpl.Editable)
	fmt.Printf("Deletable:      %v\n", tmpl.Deletable)
	fmt.Printf("Build ID:       %s\n", tmpl.BuildID)
	fmt.Printf("Build Status:   %s\n", tmpl.BuildStatus)
	fmt.Printf("vCPUs:          %d\n", tmpl.CPUCount)
	fmt.Printf("RAM MiB:        %d\n", tmpl.MemoryMB)
	fmt.Printf("DISK MiB:       %d\n", tmpl.DiskSizeMB)
	fmt.Printf("Envd Version:   %s\n", tmpl.EnvdVersion)
	fmt.Printf("Created At:     %s\n", tmpl.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated At:     %s\n", tmpl.UpdatedAt.Format(time.RFC3339))
}
