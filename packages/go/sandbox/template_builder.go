package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// ReadyCmd is a shell command used to decide when a template is ready.
type ReadyCmd struct {
	cmd string
}

func (r ReadyCmd) String() string { return r.cmd }

// WaitForPort returns a readiness check that waits for a TCP port.
func WaitForPort(port int) ReadyCmd {
	return ReadyCmd{cmd: fmt.Sprintf("python3 - <<'PY'\nimport socket, time\nfor _ in range(300):\n    s=socket.socket(); s.settimeout(1)\n    try:\n        s.connect(('127.0.0.1', %d)); s.close(); raise SystemExit(0)\n    except Exception:\n        time.sleep(1)\nraise SystemExit(1)\nPY", port)}
}

// WaitForURL returns a readiness check that waits for an HTTP status code.
func WaitForURL(rawURL string, statusCode int) ReadyCmd {
	if statusCode == 0 {
		statusCode = 200
	}
	return ReadyCmd{cmd: fmt.Sprintf("python3 - <<'PY'\nimport urllib.request, time\nfor _ in range(300):\n    try:\n        r=urllib.request.urlopen(%q, timeout=2)\n        if r.status == %d:\n            raise SystemExit(0)\n    except Exception:\n        pass\n    time.sleep(1)\nraise SystemExit(1)\nPY", rawURL, statusCode)}
}

// WaitForProcess returns a readiness check that waits for a process name.
func WaitForProcess(processName string) ReadyCmd {
	return ReadyCmd{cmd: "pgrep -f " + shellQuote(processName)}
}

// WaitForFile returns a readiness check that waits for a file path.
func WaitForFile(filename string) ReadyCmd {
	return ReadyCmd{cmd: "test -e " + shellQuote(filename)}
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

func (t *TemplateBuilder) SkipCache() *TemplateBuilder {
	t.force = true
	return t
}

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
