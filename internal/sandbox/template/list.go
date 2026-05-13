package template

import (
	"context"
	"fmt"
	"os"

	"github.com/aonesuite/aone/packages/go/sandbox"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// ListInfo holds parameters for listing templates.
type ListInfo struct {
	Name        string
	BuildStatus string
	Public      string
	Cursor      string
	Limit       int32
	Format      string // pretty or json
}

// List lists all templates.
func List(info ListInfo) {
	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	params := &sandbox.ListTemplatesParams{}
	if info.Name != "" {
		params.Name = &info.Name
	}
	if info.BuildStatus != "" {
		params.BuildStatus = &info.BuildStatus
	}
	if info.Public != "" {
		params.Public = &info.Public
	}
	if info.Cursor != "" {
		params.Cursor = &info.Cursor
	}
	if info.Limit > 0 {
		params.Limit = &info.Limit
	}

	templates, err := client.ListTemplates(context.Background(), params)
	if err != nil {
		sbClient.PrintError("list templates failed: %v", err)
		return
	}

	if info.Format == sbClient.FormatJSON {
		sbClient.PrintJSON(templates)
		return
	}

	if len(templates) == 0 {
		fmt.Println("No templates found")
		return
	}

	tw := sbClient.NewTable(os.Stdout)
	fmt.Fprintf(tw, "TEMPLATE ID\tALIASES\tSTATUS\tPUBLIC\tvCPUs\tRAM MiB\tDISK MiB\tENVD VERSION\tCREATED AT\tUPDATED AT\n")
	for _, t := range templates {
		aliases := "-"
		if len(t.Aliases) > 0 {
			aliases = t.Aliases[0]
		}
		public := "no"
		if t.Public {
			public = "yes"
		}
		envdVersion := t.EnvdVersion
		if envdVersion == "" {
			envdVersion = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
			t.TemplateID,
			aliases,
			t.BuildStatus,
			public,
			t.CPUCount,
			t.MemoryMB,
			t.DiskSizeMB,
			envdVersion,
			sbClient.FormatTimestamp(t.CreatedAt),
			sbClient.FormatTimestamp(t.UpdatedAt),
		)
	}
	tw.Flush()
}
