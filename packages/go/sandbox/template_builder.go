package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aonesuite/aone/packages/go/sandbox/dockerfile"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
)

// ReadyCmd is a shell command used to decide when a template is ready.
type ReadyCmd struct {
	cmd string
}

func (r ReadyCmd) String() string { return r.cmd }

// WaitForPort returns a readiness check that waits for a TCP port.
// Uses ss(8) to detect a listening socket, matching E2B's waitForPort so
// templates produced by one SDK work the same on the other.
func WaitForPort(port int) ReadyCmd {
	return ReadyCmd{cmd: fmt.Sprintf("ss -tuln | grep :%d", port)}
}

// WaitForURL returns a readiness check that waits for an HTTP status code.
// Uses curl + grep so the check returns success as soon as the status
// matches, matching E2B's waitForURL.
func WaitForURL(rawURL string, statusCode int) ReadyCmd {
	if statusCode == 0 {
		statusCode = 200
	}
	cmd := fmt.Sprintf(`curl -s -o /dev/null -w "%%{http_code}" %s | grep -q "%d"`, rawURL, statusCode)
	return ReadyCmd{cmd: cmd}
}

// WaitForProcess returns a readiness check that waits for a process name.
// Uses pgrep (matching on process name rather than full command line) to
// match E2B's waitForProcess.
func WaitForProcess(processName string) ReadyCmd {
	return ReadyCmd{cmd: "pgrep " + shellQuote(processName) + " > /dev/null"}
}

// WaitForFile returns a readiness check that waits for a file path.
func WaitForFile(filename string) ReadyCmd {
	return ReadyCmd{cmd: "[ -f " + shellQuote(filename) + " ]"}
}

// WaitForTimeout returns a readiness check that waits for timeoutSeconds.
func WaitForTimeout(timeoutSeconds int) ReadyCmd {
	return ReadyCmd{cmd: fmt.Sprintf("sleep %d", timeoutSeconds)}
}

// TemplateBuilder provides a fluent template build DSL.
type TemplateBuilder struct {
	fromImage         *string
	fromImageRegistry *FromImageRegistry
	fromTemplate      *string
	startCmd          *string
	readyCmd          *string
	steps             []TemplateStep
	force             bool
	contextPath       string
	ignorePatterns    []string
}

// NewTemplate creates a new fluent template builder.
func NewTemplate() *TemplateBuilder {
	base := "base"
	return &TemplateBuilder{fromImage: &base}
}

func (t *TemplateBuilder) FromDebianImage(variant string) *TemplateBuilder {
	if variant == "" {
		variant = "stable"
	}
	return t.FromImage("debian:" + variant)
}

func (t *TemplateBuilder) FromUbuntuImage(variant string) *TemplateBuilder {
	if variant == "" {
		variant = "latest"
	}
	return t.FromImage("ubuntu:" + variant)
}

func (t *TemplateBuilder) FromPythonImage(version string) *TemplateBuilder {
	if version == "" {
		version = "3"
	}
	return t.FromImage("python:" + version)
}

func (t *TemplateBuilder) FromNodeImage(variant string) *TemplateBuilder {
	if variant == "" {
		variant = "lts"
	}
	return t.FromImage("node:" + variant)
}

func (t *TemplateBuilder) FromBunImage(variant string) *TemplateBuilder {
	if variant == "" {
		variant = "latest"
	}
	return t.FromImage("oven/bun:" + variant)
}

func (t *TemplateBuilder) FromBaseImage() *TemplateBuilder {
	return t.FromImage("base")
}

func (t *TemplateBuilder) FromImage(image string) *TemplateBuilder {
	t.fromImage = &image
	t.fromTemplate = nil
	t.fromImageRegistry = nil
	return t
}

func (t *TemplateBuilder) FromTemplate(template string) *TemplateBuilder {
	t.fromTemplate = &template
	t.fromImage = nil
	t.fromImageRegistry = nil
	return t
}

// FromDockerfile parses a Dockerfile and preloads the builder with the base
// image, converted steps, and (when present) the CMD/ENTRYPOINT start command.
// Remaining customization (ReadyCmd, SetEnvs, extra steps) chains on the
// returned builder. Mirrors E2B Template.fromDockerfile.
func (t *TemplateBuilder) FromDockerfile(content string) (*TemplateBuilder, error) {
	result, err := ConvertDockerfile(content)
	if err != nil {
		return nil, err
	}
	t.FromImage(result.BaseImage)
	applyConvertedSteps(t, result.Steps)
	if result.StartCmd != "" {
		t.SetStartCmd(result.StartCmd, WaitForTimeout(20))
	}
	return t, nil
}

// FromRegistry starts the build from a private container registry using
// username/password authentication. Equivalent to E2B's fromRegistry.
func (t *TemplateBuilder) FromRegistry(image, username, password string) *TemplateBuilder {
	reg := apis.GeneralRegistry{Username: username, Password: password, Type: "registry"}
	payload, _ := json.Marshal(reg)
	raw := FromImageRegistry(payload)
	t.fromImage = &image
	t.fromImageRegistry = &raw
	t.fromTemplate = nil
	return t
}

// FromAWSRegistry starts the build from an AWS ECR image using the supplied
// credentials. Equivalent to E2B's fromAWSRegistry.
func (t *TemplateBuilder) FromAWSRegistry(image, accessKeyID, secretAccessKey, region string) *TemplateBuilder {
	reg := apis.AWSRegistry{
		AwsAccessKeyID:     accessKeyID,
		AwsSecretAccessKey: secretAccessKey,
		AwsRegion:          region,
		Type:               "aws",
	}
	payload, _ := json.Marshal(reg)
	raw := FromImageRegistry(payload)
	t.fromImage = &image
	t.fromImageRegistry = &raw
	t.fromTemplate = nil
	return t
}

// FromGCPRegistry starts the build from a Google Cloud Artifact / Container
// Registry image using a service-account JSON. Equivalent to E2B's
// fromGCPRegistry.
func (t *TemplateBuilder) FromGCPRegistry(image, serviceAccountJSON string) *TemplateBuilder {
	reg := apis.GCPRegistry{ServiceAccountJSON: serviceAccountJSON, Type: "gcp"}
	payload, _ := json.Marshal(reg)
	raw := FromImageRegistry(payload)
	t.fromImage = &image
	t.fromImageRegistry = &raw
	t.fromTemplate = nil
	return t
}

func (t *TemplateBuilder) AddStep(stepType string, args ...string) *TemplateBuilder {
	force := t.force
	t.steps = append(t.steps, TemplateStep{Type: stepType, Args: &args, Force: &force})
	return t
}

func (t *TemplateBuilder) Copy(src, dest string) *TemplateBuilder {
	return t.AddStep("COPY", src, dest)
}

func (t *TemplateBuilder) CopyItems(items []CopyItem) *TemplateBuilder {
	for _, item := range items {
		t.Copy(item.Src, item.Dest)
	}
	return t
}

type CopyItem struct {
	Src  string
	Dest string
}

func (t *TemplateBuilder) Remove(path string, recursive, force bool, user string) *TemplateBuilder {
	args := []string{"rm"}
	if recursive {
		args = append(args, "-r")
	}
	if force {
		args = append(args, "-f")
	}
	args = append(args, path)
	return t.runCmdAs(strings.Join(args, " "), user)
}

func (t *TemplateBuilder) Rename(src, dest, user string) *TemplateBuilder {
	return t.runCmdAs("mv "+shellQuote(src)+" "+shellQuote(dest), user)
}

func (t *TemplateBuilder) MakeDir(path, user string) *TemplateBuilder {
	return t.runCmdAs("mkdir -p "+shellQuote(path), user)
}

func (t *TemplateBuilder) MakeSymlink(src, dest, user string, force bool) *TemplateBuilder {
	flag := ""
	if force {
		flag = "-f "
	}
	return t.runCmdAs("ln -s "+flag+shellQuote(src)+" "+shellQuote(dest), user)
}

func (t *TemplateBuilder) RunCmd(cmd string) *TemplateBuilder {
	return t.runCmdAs(cmd, "")
}

func (t *TemplateBuilder) runCmdAs(cmd, user string) *TemplateBuilder {
	if user != "" {
		cmd = "sudo -u " + shellQuote(user) + " bash -lc " + shellQuote(cmd)
	}
	return t.AddStep("RUN", cmd)
}

func (t *TemplateBuilder) SetWorkdir(workdir string) *TemplateBuilder {
	return t.AddStep("WORKDIR", workdir)
}

func (t *TemplateBuilder) SetUser(user string) *TemplateBuilder {
	return t.AddStep("USER", user)
}

func (t *TemplateBuilder) PipInstall(packages ...string) *TemplateBuilder {
	if len(packages) == 0 {
		return t.RunCmd("pip install .")
	}
	return t.RunCmd("pip install " + strings.Join(quoteAll(packages), " "))
}

func (t *TemplateBuilder) NpmInstall(packages ...string) *TemplateBuilder {
	if len(packages) == 0 {
		return t.RunCmd("npm install")
	}
	return t.RunCmd("npm install " + strings.Join(quoteAll(packages), " "))
}

func (t *TemplateBuilder) BunInstall(packages ...string) *TemplateBuilder {
	if len(packages) == 0 {
		return t.RunCmd("bun install")
	}
	return t.RunCmd("bun add " + strings.Join(quoteAll(packages), " "))
}

func (t *TemplateBuilder) AptInstall(packages ...string) *TemplateBuilder {
	return t.RunCmd("apt-get update && apt-get install -y " + strings.Join(quoteAll(packages), " "))
}

func (t *TemplateBuilder) GitClone(repoURL, dest string) *TemplateBuilder {
	cmd := "git clone " + shellQuote(repoURL)
	if dest != "" {
		cmd += " " + shellQuote(dest)
	}
	return t.RunCmd(cmd)
}

func (t *TemplateBuilder) SetEnvs(envs map[string]string) *TemplateBuilder {
	for k, v := range envs {
		t.AddStep("ENV", k+"="+v)
	}
	return t
}

// AddMcpServer appends an MCP_SERVER step for each server name. Mirrors E2B
// addMcpServer: the aone build system interprets these as MCP registrations
// so sandboxes spawned from the template auto-start the requested servers.
func (t *TemplateBuilder) AddMcpServer(servers ...string) *TemplateBuilder {
	for _, name := range servers {
		if name == "" {
			continue
		}
		t.AddStep("MCP_SERVER", name)
	}
	return t
}

func (t *TemplateBuilder) SkipCache() *TemplateBuilder {
	t.force = true
	return t
}

// ForceBuild is an alias for SkipCache that matches E2B's naming. All
// subsequent AddStep calls (and the final build) will bypass the layer
// cache.
func (t *TemplateBuilder) ForceBuild() *TemplateBuilder {
	return t.SkipCache()
}

// ForceNextLayer flips the cache-bypass flag for just the next step added,
// then restores the previous value. Useful when only one layer needs a
// forced rebuild (for example a RUN that pulls fresh upstream content).
func (t *TemplateBuilder) ForceNextLayer(fn func(*TemplateBuilder)) *TemplateBuilder {
	prev := t.force
	t.force = true
	fn(t)
	t.force = prev
	return t
}

// SetContextPath records the local build-context directory used by COPY
// steps. When set, the SDK/CLI upload path can hash and upload files
// relative to this directory and automatically respects the .dockerignore
// file located there. Mirrors how `docker build <context>` behaves.
//
// Calling SetContextPath also reloads .dockerignore from contextPath, so
// late edits to that file before Build start will be honored as long as
// SetContextPath is called after the edit.
func (t *TemplateBuilder) SetContextPath(contextPath string) *TemplateBuilder {
	t.contextPath = contextPath
	if contextPath != "" {
		t.ignorePatterns = readDockerignorePatterns(contextPath)
	} else {
		t.ignorePatterns = nil
	}
	return t
}

// ContextPath returns the build-context directory previously set with
// SetContextPath. Empty string means no context has been configured.
func (t *TemplateBuilder) ContextPath() string { return t.contextPath }

// IgnorePatterns returns the .dockerignore patterns loaded by SetContextPath.
// The slice is owned by the builder; callers must not modify it.
func (t *TemplateBuilder) IgnorePatterns() []string { return t.ignorePatterns }

func (t *TemplateBuilder) SetStartCmd(start string, ready ReadyCmd) *TemplateBuilder {
	t.startCmd = &start
	cmd := ready.String()
	t.readyCmd = &cmd
	return t
}

func (t *TemplateBuilder) SetReadyCmd(ready ReadyCmd) *TemplateBuilder {
	cmd := ready.String()
	t.readyCmd = &cmd
	return t
}

// ToDockerfile renders a best-effort Dockerfile for image-based templates.
func (t *TemplateBuilder) ToDockerfile() (string, error) {
	if t.fromTemplate != nil {
		return "", fmt.Errorf("templates based on another template cannot be converted to Dockerfile")
	}
	image := "base"
	if t.fromImage != nil {
		image = *t.fromImage
	}
	var b strings.Builder
	b.WriteString("FROM " + image + "\n")
	for _, step := range t.steps {
		args := []string{}
		if step.Args != nil {
			args = *step.Args
		}
		b.WriteString(step.Type + " " + strings.Join(args, " ") + "\n")
	}
	if t.startCmd != nil {
		b.WriteString("CMD " + *t.startCmd + "\n")
	}
	return b.String(), nil
}

// Build creates and builds a template, then waits for completion.
func (t *TemplateBuilder) Build(ctx context.Context, c *Client, name string, opts BuildTemplateOptions, pollOpts ...PollOption) (*BuildInfo, error) {
	info, err := t.BuildInBackground(ctx, c, name, opts)
	if err != nil {
		return nil, err
	}
	if _, err := c.WaitForBuild(ctx, info.TemplateID, info.BuildID, pollOpts...); err != nil {
		return nil, err
	}
	return info, nil
}

// BuildInBackground creates and starts a template build without waiting.
func (t *TemplateBuilder) BuildInBackground(ctx context.Context, c *Client, name string, opts BuildTemplateOptions) (*BuildInfo, error) {
	create := CreateTemplateParams{
		Alias:    &name,
		Name:     &name,
		Tags:     &opts.Tags,
		CPUCount: opts.CPUCount,
		MemoryMB: opts.MemoryMB,
	}
	created, err := c.CreateTemplate(ctx, create)
	if err != nil {
		return nil, err
	}
	force := opts.SkipCache || t.force
	start := StartTemplateBuildParams{
		Force:             &force,
		FromImage:         t.fromImage,
		FromImageRegistry: t.fromImageRegistry,
		FromTemplate:      t.fromTemplate,
		StartCmd:          t.startCmd,
		ReadyCmd:          t.readyCmd,
		Steps:             &t.steps,
	}
	if err := c.StartTemplateBuild(ctx, created.TemplateID, created.BuildID, start); err != nil {
		return nil, err
	}
	return &BuildInfo{
		Name:       name,
		TemplateID: created.TemplateID,
		BuildID:    created.BuildID,
		Tags:       created.Tags,
	}, nil
}

type BuildTemplateOptions struct {
	Tags      []string
	CPUCount  *int32
	MemoryMB  *int32
	SkipCache bool
}

type BuildInfo struct {
	Name       string
	TemplateID string
	BuildID    string
	Tags       []string
}

func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = shellQuote(v)
	}
	return out
}

// readDockerignorePatterns wraps dockerfile.ReadDockerignore so the builder
// does not need to import the subpackage from every call site.
func readDockerignorePatterns(contextPath string) []string {
	return dockerfile.ReadDockerignore(contextPath)
}
