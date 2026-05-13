package template

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/packages/go/sandbox"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// BuildInfo holds parameters for building templates.
type BuildInfo struct {
	// Name is the template name used when creating and building a template.
	Name string

	// FromImage is the base Docker image.
	FromImage string

	// FromTemplate is the base template.
	FromTemplate string

	// StartCmd is the command to run after the build completes.
	StartCmd string

	// ReadyCmd is the readiness check command.
	ReadyCmd string

	// CPUCount is the sandbox CPU core count.
	CPUCount int32

	// MemoryMB is the sandbox memory size in MiB.
	MemoryMB int32

	// DiskSizeMB is the sandbox disk size in MiB.
	DiskSizeMB int32

	// Public controls template visibility. Empty means use the server default.
	Public string

	// Wait indicates whether to wait for build completion.
	Wait bool

	// Dockerfile is the Dockerfile path and enables v2 Dockerfile builds.
	Dockerfile string

	// Path is the build context directory and defaults to the Dockerfile directory.
	Path string

	// ConfigPath, when non-empty, points at an explicit aone.sandbox.toml.
	// Otherwise the file is looked up under Path (or CWD).
	ConfigPath string

	// SaveConfig, when true, writes the resolved template_id, name, and
	// resource fields back to aone.sandbox.toml after a successful build
	// or template creation. Defaults to true at the CLI layer.
	SaveConfig bool
}

// Build creates a template using the OpenAPI-backed create-template endpoint.
func Build(info BuildInfo) {
	// Pull defaults from aone.sandbox.toml when fields are missing. Flag
	// values always win; the file fills in the blanks so users can re-run
	// `aone sandbox template build` without re-typing every option.
	projectCfg, projectLoc, pErr := config.LoadProject(info.ConfigPath, info.Path)
	if pErr != nil {
		sbClient.PrintError("%v", pErr)
		return
	}
	if projectCfg != nil {
		applyProjectDefaults(&info, projectCfg)
	}
	if info.Dockerfile == "" && info.FromImage == "" && info.FromTemplate == "" {
		if resolved, err := resolveDockerfilePath(info.Path, ""); err == nil {
			info.Dockerfile = resolved
		}
	}
	if info.Dockerfile != "" {
		resolved, err := resolveDockerfilePath(info.Path, info.Dockerfile)
		if err != nil {
			sbClient.PrintError("%v", err)
			return
		}
		info.Dockerfile = resolved
	}

	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	ctx := context.Background()
	if info.Name == "" {
		sbClient.PrintError("template name (--name) is required")
		return
	}
	if info.Dockerfile == "" && info.FromImage == "" && info.FromTemplate == "" {
		sbClient.PrintError("--from-image, --from-template, or --dockerfile is required")
		return
	}

	createParams := sandbox.CreateTemplateParams{
		Name:     &info.Name,
		StartCmd: stringPtrFromNonEmpty(info.StartCmd),
		ReadyCmd: stringPtrFromNonEmpty(info.ReadyCmd),
	}
	if info.CPUCount > 0 {
		createParams.CPUCount = &info.CPUCount
	}
	if info.MemoryMB > 0 {
		createParams.MemoryMB = &info.MemoryMB
	}
	if info.DiskSizeMB > 0 {
		createParams.DiskSizeMB = &info.DiskSizeMB
	}
	if info.Public != "" {
		public, pErr := strconv.ParseBool(info.Public)
		if pErr != nil {
			sbClient.PrintError("--public must be true or false")
			return
		}
		createParams.Public = &public
	}
	if info.Dockerfile != "" {
		content, rErr := os.ReadFile(info.Dockerfile)
		if rErr != nil {
			sbClient.PrintError("read Dockerfile failed: %v", rErr)
			return
		}
		dockerfile := string(content)
		createParams.Dockerfile = &dockerfile
	} else if info.FromImage != "" {
		dockerfile := "FROM " + info.FromImage + "\n"
		createParams.Dockerfile = &dockerfile
	} else if info.FromTemplate != "" {
		dockerfile := "FROM " + info.FromTemplate + "\n"
		createParams.Dockerfile = &dockerfile
	}

	fmt.Printf("Creating template %s...\n", info.Name)
	resp, cErr := client.CreateTemplate(ctx, createParams)
	if cErr != nil {
		sbClient.PrintError("create template failed: %v", cErr)
		return
	}
	templateID := resp.TemplateID
	buildID := resp.BuildID
	sbClient.PrintSuccess("Template %s created (build ID: %s)", templateID, buildID)

	// Persist newly assigned identifiers so subsequent commands can
	// pick them up without explicit flags. Best-effort: log on failure
	// but don't fail the build.
	if info.SaveConfig {
		if pErr := saveProjectFromBuild(info, projectLoc, templateID); pErr != nil {
			sbClient.PrintWarn("could not save %s: %v", config.ProjectFileName, pErr)
		}
	}

	if !info.Wait {
		fmt.Printf("Build started. Use 'aone sandbox template builds %s %s' to check status.\n", templateID, buildID)
		return
	}

	// Stream build logs and support Ctrl+C cancellation.
	fmt.Println("Waiting for build to complete...")

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	var cursor *int64
	for {
		logs, blErr := client.GetTemplateBuildLogs(ctx, templateID, buildID, &sandbox.GetBuildLogsParams{
			Cursor: cursor,
		})
		if blErr == nil && logs != nil {
			for _, entry := range logs.Logs {
				fmt.Printf("[%s] %s %s\n",
					sbClient.FormatTimestamp(entry.Timestamp),
					sbClient.LogLevelBadge(string(entry.Level)),
					entry.Message,
				)
				ts := entry.Timestamp.UnixMilli() + 1
				cursor = &ts
			}
		}

		// Check build status.
		buildInfo, bErr := client.GetTemplateBuildStatus(ctx, templateID, buildID)
		if bErr != nil {
			sbClient.PrintError("get build status failed: %v", bErr)
			return
		}

		if buildInfo.Status == "ready" || buildInfo.Status == "error" {
			if buildInfo.Status == "error" {
				sbClient.PrintError("build failed")
			} else {
				sbClient.PrintSuccess("Build completed!")
			}
			fmt.Printf("Template ID:  %s\n", buildInfo.TemplateID)
			fmt.Printf("Build ID:     %s\n", buildInfo.BuildID)
			fmt.Printf("Status:       %s\n", buildInfo.Status)

			if buildInfo.Status == "ready" {
				printSDKExamples(buildInfo.TemplateID)
			}
			return
		}

		select {
		case <-ctx.Done():
			sbClient.PrintError("build watch cancelled")
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func stringPtrFromNonEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// applyProjectDefaults fills in BuildInfo fields that are still zero from
// the loaded project config. Flag/CLI-supplied values are never overridden.
func applyProjectDefaults(info *BuildInfo, p *config.Project) {
	if info.Name == "" {
		info.Name = p.TemplateName
	}
	if info.Dockerfile == "" {
		info.Dockerfile = p.Dockerfile
	}
	if info.StartCmd == "" {
		info.StartCmd = p.StartCmd
	}
	if info.ReadyCmd == "" {
		info.ReadyCmd = p.ReadyCmd
	}
	if info.CPUCount == 0 && p.CPUCount > 0 {
		info.CPUCount = int32(p.CPUCount)
	}
	if info.MemoryMB == 0 && p.MemoryMB > 0 {
		info.MemoryMB = int32(p.MemoryMB)
	}
	if info.DiskSizeMB == 0 && p.DiskSizeMB > 0 {
		info.DiskSizeMB = int32(p.DiskSizeMB)
	}
	if info.Public == "" && p.Public != nil {
		info.Public = strconv.FormatBool(*p.Public)
	}
}

// saveProjectFromBuild writes the resolved template id (and other fields)
// back to the project config. When loc is nil we create a fresh file at
// info.Path/aone.sandbox.toml; otherwise we update in place to avoid
// surprising users with a relocated config.
func saveProjectFromBuild(info BuildInfo, loc *config.ProjectLocation, templateID string) error {
	dest := ""
	switch {
	case info.ConfigPath != "":
		dest = info.ConfigPath
	case loc != nil:
		dest = loc.Path
	default:
		dest = config.DefaultProjectPath(info.Path)
	}

	// Re-read to preserve any fields we don't manage here (forward-compat).
	existing, _, _ := config.LoadProject(dest, "")
	if existing == nil {
		existing = &config.Project{}
	}
	existing.TemplateID = templateID
	if info.Name != "" {
		existing.TemplateName = info.Name
	}
	if info.Dockerfile != "" {
		existing.Dockerfile = info.Dockerfile
	}
	if info.StartCmd != "" {
		existing.StartCmd = info.StartCmd
	}
	if info.ReadyCmd != "" {
		existing.ReadyCmd = info.ReadyCmd
	}
	if info.CPUCount > 0 {
		existing.CPUCount = int(info.CPUCount)
	}
	if info.MemoryMB > 0 {
		existing.MemoryMB = int(info.MemoryMB)
	}
	if info.DiskSizeMB > 0 {
		existing.DiskSizeMB = int(info.DiskSizeMB)
	}
	if info.Public != "" {
		public, err := strconv.ParseBool(info.Public)
		if err == nil {
			existing.Public = &public
		}
	}
	return config.SaveProject(existing, dest)
}

// printSDKExamples prints SDK usage examples for the given template ID.
func printSDKExamples(templateID string) {
	fmt.Println()
	sbClient.PrintSuccessBox("Template is ready! Use it with the SDK:")

	fmt.Printf("\n%s\n", sbClient.ColorInfo.Sprint("Go:"))
	fmt.Println(sbClient.FormatCodeBlock(fmt.Sprintf(`sb, _ := client.CreateAndWait(ctx, sandbox.CreateParams{
    TemplateID: "%s",
})`, templateID), "go"))

	fmt.Printf("\n%s\n", sbClient.ColorInfo.Sprint("Python:"))
	fmt.Println(sbClient.FormatCodeBlock(fmt.Sprintf(`sandbox = client.sandboxes.create("%s")`, templateID), "python"))

	fmt.Printf("\n%s\n", sbClient.ColorInfo.Sprint("TypeScript:"))
	fmt.Println(sbClient.FormatCodeBlock(fmt.Sprintf(`const sandbox = await client.sandboxes.create("%s")`, templateID), "typescript"))
	fmt.Println()
}
