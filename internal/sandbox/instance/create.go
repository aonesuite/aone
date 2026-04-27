package instance

import (
	"context"
	"fmt"
	"strings"

	"github.com/aonesuite/aone/packages/go/sandbox"

	"github.com/aonesuite/aone/internal/config"
	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// CreateInfo holds parameters for creating a sandbox.
type CreateInfo struct {
	TemplateID string
	Timeout    int32
	Metadata   string
	Detach     bool
	EnvVars    []string // KEY=VALUE pairs
	AutoPause  bool

	// ConfigPath optionally points at an explicit aone.sandbox.toml. When
	// empty, the file is looked up under Path (or CWD).
	ConfigPath string

	// Path is the project root used to locate aone.sandbox.toml when
	// TemplateID is not provided on the command line.
	Path string
}

// Create creates a new sandbox and connects to its terminal.
// When the terminal session ends, the sandbox is killed.
// The sandbox stays alive through keep-alive pings in the terminal session.
func Create(info CreateInfo) {
	// Fall back to the project config so `aone sandbox create` works inside
	// an initialized project without re-typing the template id.
	if info.TemplateID == "" {
		if p, _, err := config.LoadProject(info.ConfigPath, info.Path); err == nil && p != nil && p.TemplateID != "" {
			info.TemplateID = p.TemplateID
		}
	}
	if info.TemplateID == "" {
		info.TemplateID = "base"
	}
	if info.TemplateID == "" {
		sbClient.PrintError("template ID is required (pass as argument or set template_id in aone.sandbox.toml)")
		return
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	ctx := context.Background()
	params := sandbox.CreateParams{
		TemplateID: info.TemplateID,
	}
	if info.Timeout > 0 {
		params.Timeout = &info.Timeout
	}
	if info.Metadata != "" {
		meta := sandbox.Metadata(sbClient.ParseMetadataMap(info.Metadata))
		params.Metadata = &meta
	}
	if len(info.EnvVars) > 0 {
		envMap := parseEnvPairs(info.EnvVars)
		if len(envMap) > 0 {
			params.EnvVars = &envMap
		}
	}
	if info.AutoPause {
		params.AutoPause = &info.AutoPause
	}

	fmt.Printf("Creating sandbox from template %s...\n", info.TemplateID)
	sb, _, err := client.CreateAndWait(ctx, params)
	if err != nil {
		sbClient.PrintError("create sandbox failed: %v", err)
		return
	}
	if info.Detach {
		sbClient.PrintSuccess("Sandbox %s created", sb.ID())
		fmt.Printf("Sandbox ID:   %s\n", sb.ID())
		fmt.Printf("Template ID:  %s\n", sb.TemplateID())
		fmt.Println()
		fmt.Printf("Connect:  aone sandbox connect %s\n", sb.ID())
		fmt.Printf("Exec:     aone sandbox exec %s -- <command>\n", sb.ID())
		fmt.Printf("Kill:     aone sandbox kill %s\n", sb.ID())
		return
	}

	sbClient.PrintSuccess("Sandbox %s created, connecting...", sb.ID())

	// When create session ends, kill the sandbox
	defer func() {
		fmt.Printf("\nKilling sandbox %s...\n", sb.ID())
		if kErr := sb.Kill(context.Background()); kErr != nil {
			// Ignore 404 errors: sandbox may have already been terminated by timeout
			if !strings.Contains(kErr.Error(), "404") {
				sbClient.PrintWarn("kill sandbox failed: %v", kErr)
			}
		}
	}()

	runTerminalSession(ctx, sb)
}
