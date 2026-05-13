package template

import (
	"context"
	"fmt"

	"github.com/aonesuite/aone/packages/go/sandbox"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// LogsInfo holds parameters for viewing template build logs.
type LogsInfo struct {
	TemplateID string
	BuildID    string
	Cursor     int64
	Limit      int32
	Direction  string
	Level      string
	Source     string
	Format     string
}

// Logs retrieves and displays structured template build logs.
func Logs(info LogsInfo) {
	if info.TemplateID == "" {
		sbClient.PrintError("template ID is required")
		return
	}
	if info.BuildID == "" {
		sbClient.PrintError("build ID is required")
		return
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	params := &sandbox.GetBuildLogsParams{}
	if info.Cursor > 0 {
		params.Cursor = &info.Cursor
	}
	if info.Limit > 0 {
		params.Limit = &info.Limit
	}
	if info.Direction != "" {
		direction := sandbox.LogsDirection(info.Direction)
		params.Direction = &direction
	}
	if info.Level != "" {
		level := sandbox.LogLevel(info.Level)
		params.Level = &level
	}
	if info.Source != "" {
		source := sandbox.LogsSource(info.Source)
		params.Source = &source
	}

	logs, err := client.GetTemplateBuildLogs(context.Background(), info.TemplateID, info.BuildID, params)
	if err != nil {
		sbClient.PrintError("get build logs failed: %v", err)
		return
	}

	if info.Format == sbClient.FormatJSON {
		sbClient.PrintJSON(logs)
		return
	}
	if logs == nil || len(logs.Logs) == 0 {
		fmt.Println("No build logs found")
		return
	}
	for _, entry := range logs.Logs {
		fmt.Printf("[%s] %s %s\n",
			sbClient.FormatTimestamp(entry.Timestamp),
			sbClient.LogLevelBadge(string(entry.Level)),
			entry.Message,
		)
	}
}
