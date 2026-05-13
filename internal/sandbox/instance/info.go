package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aonesuite/aone/packages/go/sandbox"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// InfoInfo holds parameters for getting sandbox details.
type InfoInfo struct {
	// SandboxID identifies the sandbox to inspect.
	SandboxID string

	// Format selects the output format. Supported values: "pretty" (default) and "json".
	Format string
}

// Info prints detailed information for a single sandbox.
// It resolves the sandbox by ID, then renders a labelled summary in pretty
// mode (the default) or the raw info JSON when --format json is requested.
func Info(info InfoInfo) {
	if info.SandboxID == "" {
		sbClient.PrintError("sandbox ID is required")
		return
	}

	format := info.Format
	if format == "" {
		format = sbClient.FormatPretty
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	ctx := context.Background()
	sb, err := client.Connect(ctx, info.SandboxID, sandbox.ConnectParams{Timeout: sbClient.ConnectTimeoutCommand})
	if err != nil {
		sbClient.PrintError("connect to sandbox %s failed: %v", info.SandboxID, err)
		return
	}

	detail, err := sb.GetInfo(ctx)
	if err != nil {
		sbClient.PrintError("get sandbox info failed: %v", err)
		return
	}

	switch strings.ToLower(format) {
	case sbClient.FormatJSON:
		sbClient.PrintJSON(detail)
	case sbClient.FormatPretty:
		renderPrettyInfo(detail)
	default:
		sbClient.PrintError("unsupported output format: %s", format)
	}
}

// renderPrettyInfo prints the sandbox info with human-readable labels.
func renderPrettyInfo(d *sandbox.SandboxInfo) {
	fmt.Printf("\nSandbox info for %s:\n", d.SandboxID)

	printField("Sandbox ID", d.SandboxID)
	printField("Template ID", d.TemplateID)
	if d.Alias != nil {
		printField("Alias", *d.Alias)
	} else if d.Name != nil {
		printField("Alias", *d.Name)
	}
	printField("State", string(d.State))
	printField("Started at", sbClient.FormatTimestamp(d.StartedAt))
	printField("End at", sbClient.FormatTimestamp(d.EndAt))
	printField("vCPUs", fmt.Sprintf("%d", d.CPUCount))
	printField("RAM MiB", fmt.Sprintf("%d", d.MemoryMB))
	if d.DiskSizeMB > 0 {
		printField("Disk MiB", fmt.Sprintf("%d", d.DiskSizeMB))
	}
	printField("Envd version", d.EnvdVersion)
	if d.AllowInternetAccess != nil {
		printField("Internet access", fmt.Sprintf("%v", *d.AllowInternetAccess))
	}
	if d.Network != nil {
		if b, err := json.MarshalIndent(d.Network, "  ", "  "); err == nil {
			fmt.Printf("Network:\n  %s\n", string(b))
		}
	}
	if d.Domain != nil {
		printField("Sandbox domain", *d.Domain)
	}
	if d.Metadata != nil && len(*d.Metadata) > 0 {
		printField("Metadata", sbClient.FormatMetadata(map[string]string(*d.Metadata)))
	}
	fmt.Println()
}

// printField writes a single "label: value" line, skipping empty values.
func printField(label, value string) {
	if value == "" {
		return
	}
	fmt.Printf("%s: %s\n", label, value)
}
