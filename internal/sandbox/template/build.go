package template

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox"

	sbClient "github.com/aonesuite/aone/internal/sandbox"
	"github.com/aonesuite/aone/internal/sandbox/template/dockerfile"
)

// BuildInfo holds parameters for building templates.
type BuildInfo struct {
	// Name is the template name used when creating and building a template.
	Name string

	// TemplateID is an existing template ID used for rebuilds.
	TemplateID string

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

	// Wait indicates whether to wait for build completion.
	Wait bool

	// NoCache forces a full build and ignores cache.
	NoCache bool

	// Dockerfile is the Dockerfile path and enables v2 Dockerfile builds.
	Dockerfile string

	// Path is the build context directory and defaults to the Dockerfile directory.
	Path string
}

// Build creates or rebuilds a template.
// When TemplateID is provided, it starts a new build for an existing template.
// Otherwise, it creates a new template using Name and starts its first build.
func Build(info BuildInfo) {
	client, err := sbClient.NewSandboxClient()
	if err != nil {
		sbClient.PrintError("%v", err)
		return
	}

	ctx := context.Background()
	templateID := info.TemplateID
	buildID := ""

	if templateID == "" {
		// Create a new template.
		if info.Name == "" {
			sbClient.PrintError("template name (--name) or template ID (--template-id) is required")
			return
		}

		createParams := sandbox.CreateTemplateParams{
			Name: &info.Name,
		}
		if info.CPUCount > 0 {
			createParams.CPUCount = &info.CPUCount
		}
		if info.MemoryMB > 0 {
			createParams.MemoryMB = &info.MemoryMB
		}

		fmt.Printf("Creating template %s...\n", info.Name)
		resp, cErr := client.CreateTemplate(ctx, createParams)
		if cErr != nil {
			sbClient.PrintError("create template failed: %v", cErr)
			return
		}
		templateID = resp.TemplateID
		buildID = resp.BuildID
		sbClient.PrintSuccess("Template %s created (build ID: %s)", templateID, buildID)
	} else {
		// Fetch the existing template to find the latest build ID.
		tmpl, gErr := client.GetTemplate(ctx, templateID, nil)
		if gErr != nil {
			sbClient.PrintError("get template failed: %v", gErr)
			return
		}
		if len(tmpl.Builds) > 0 {
			// Use the last build; the API returns builds in ascending time order.
			buildID = tmpl.Builds[len(tmpl.Builds)-1].BuildID
		} else {
			sbClient.PrintError("no builds found for template, cannot rebuild")
			return
		}
	}

	if info.Dockerfile != "" {
		if err := buildFromDockerfile(ctx, client, templateID, buildID, info); err != nil {
			sbClient.PrintError("%v", err)
			return
		}
	} else {
		// Validate the build source.
		if info.FromImage == "" && info.FromTemplate == "" {
			sbClient.PrintError("--from-image, --from-template, or --dockerfile is required")
			return
		}

		buildParams := sandbox.StartTemplateBuildParams{}
		if info.FromImage != "" {
			buildParams.FromImage = &info.FromImage
		}
		if info.FromTemplate != "" {
			buildParams.FromTemplate = &info.FromTemplate
		}
		if info.StartCmd != "" {
			buildParams.StartCmd = &info.StartCmd
		}
		if info.ReadyCmd != "" {
			buildParams.ReadyCmd = &info.ReadyCmd
		}
		if info.NoCache {
			force := true
			buildParams.Force = &force
		}

		fmt.Printf("Starting build for template %s (build ID: %s)...\n", templateID, buildID)
		if err := client.StartTemplateBuild(ctx, templateID, buildID, buildParams); err != nil {
			sbClient.PrintError("start build failed: %v", err)
			return
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
		buildInfo, bErr := client.GetTemplateBuildStatus(ctx, templateID, buildID, nil)
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

// buildFromDockerfile handles the v2 Dockerfile build flow:
// parse Dockerfile, upload COPY files, then start the build with steps.
func buildFromDockerfile(ctx context.Context, client *sandbox.Client, templateID, buildID string, info BuildInfo) error {
	// Read the Dockerfile.
	content, err := os.ReadFile(info.Dockerfile)
	if err != nil {
		return fmt.Errorf("read Dockerfile: %w", err)
	}

	// Determine the build context directory.
	contextPath := info.Path
	if contextPath == "" {
		contextPath = filepath.Dir(info.Dockerfile)
	}
	contextPath, err = filepath.Abs(contextPath)
	if err != nil {
		return fmt.Errorf("resolve context path: %w", err)
	}

	// Parse the Dockerfile.
	result, err := dockerfile.Convert(string(content))
	if err != nil {
		return fmt.Errorf("parse Dockerfile: %w", err)
	}
	fmt.Printf("Parsed Dockerfile: base image=%s, %d steps\n", result.BaseImage, len(result.Steps))

	// Read .dockerignore.
	ignorePatterns := dockerfile.ReadDockerignore(contextPath)

	// Process COPY steps: compute file hashes and upload files.
	for i := range result.Steps {
		step := &result.Steps[i]
		if step.Type != "COPY" || step.Args == nil || len(*step.Args) < 2 {
			continue
		}
		args := *step.Args
		src, dest := args[0], args[1]

		// Compute the file hash.
		hash, err := dockerfile.ComputeFilesHash(src, dest, contextPath, ignorePatterns)
		if err != nil {
			return fmt.Errorf("compute file hash for COPY %s %s: %w", src, dest, err)
		}
		step.FilesHash = &hash

		// Check whether files need to be uploaded.
		fileInfo, err := client.GetTemplateFiles(ctx, templateID, hash)
		if err != nil {
			return fmt.Errorf("get template files for hash %s: %w", hash, err)
		}

		if !fileInfo.Present && fileInfo.URL != nil {
			fmt.Printf("Uploading files for COPY %s %s...\n", src, dest)
			if err := dockerfile.CollectAndUpload(ctx, *fileInfo.URL, src, contextPath, ignorePatterns); err != nil {
				return fmt.Errorf("upload files for COPY %s %s: %w", src, dest, err)
			}
		} else if fileInfo.Present {
			fmt.Printf("Files for COPY %s %s already uploaded (cached)\n", src, dest)
		}
	}

	// Build parameters.
	buildParams := sandbox.StartTemplateBuildParams{
		FromImage: &result.BaseImage,
		Steps:     &result.Steps,
	}

	// Apply startup/readiness commands from Dockerfile or CLI overrides.
	startCmd := result.StartCmd
	if info.StartCmd != "" {
		startCmd = info.StartCmd
	}
	if startCmd != "" {
		buildParams.StartCmd = &startCmd
	}

	readyCmd := result.ReadyCmd
	if info.ReadyCmd != "" {
		readyCmd = info.ReadyCmd
	}
	if readyCmd != "" {
		buildParams.ReadyCmd = &readyCmd
	}

	if info.NoCache {
		force := true
		buildParams.Force = &force
	}

	fmt.Printf("Starting build for template %s (build ID: %s)...\n", templateID, buildID)
	if err := client.StartTemplateBuild(ctx, templateID, buildID, buildParams); err != nil {
		return fmt.Errorf("start build: %w", err)
	}

	return nil
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
