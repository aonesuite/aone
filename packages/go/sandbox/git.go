package sandbox

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Git provides common git operations inside a sandbox.
type Git struct {
	commands *Commands
}

// Git returns the git helper bound to this sandbox.
func (s *Sandbox) Git() *Git {
	return &Git{commands: s.Commands()}
}

// GitOption customizes git command execution.
type GitOption func(*gitOpts)

type gitOpts struct {
	commandOpts []CommandOption
}

func WithGitCommandOptions(opts ...CommandOption) GitOption {
	return func(o *gitOpts) { o.commandOpts = append(o.commandOpts, opts...) }
}

func applyGitOpts(opts []GitOption) *gitOpts {
	o := &gitOpts{}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

func (g *Git) run(ctx context.Context, repoPath string, args []string, opts ...GitOption) (*CommandResult, error) {
	o := applyGitOpts(opts)
	cmd := buildGitCommand(args, repoPath)
	return g.commands.Run(ctx, cmd, o.commandOpts...)
}

func (g *Git) runShell(ctx context.Context, cmd string, opts ...GitOption) (*CommandResult, error) {
	o := applyGitOpts(opts)
	return g.commands.Run(ctx, cmd, o.commandOpts...)
}

// Clone clones a repository into the sandbox.
func (g *Git) Clone(ctx context.Context, repoURL string, opts *GitCloneOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &GitCloneOptions{}
	}
	cloneURL := repoURL
	if opts.Username != "" || opts.Password != "" {
		if opts.Username == "" || opts.Password == "" {
			return nil, fmt.Errorf("username and password must be provided together")
		}
		withCreds, err := urlWithCredentials(repoURL, opts.Username, opts.Password)
		if err != nil {
			return nil, err
		}
		cloneURL = withCreds
	}
	args := []string{"clone", cloneURL}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch, "--single-branch")
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprint(opts.Depth))
	}
	if opts.Path != "" {
		args = append(args, opts.Path)
	}
	res, err := g.run(ctx, "", args, opts.Options...)
	if err != nil {
		return nil, err
	}
	if (opts.Username != "" || opts.Password != "") && !opts.DangerouslyStoreCredentials {
		repoPath := opts.Path
		if repoPath == "" {
			repoPath = deriveRepoDir(repoURL)
		}
		if repoPath != "" {
			_, _ = g.run(ctx, repoPath, []string{"remote", "set-url", "origin", stripURLCredentials(cloneURL)}, opts.Options...)
		}
	}
	return res, nil
}

type GitCloneOptions struct {
	Path                        string
	Branch                      string
	Depth                       int
	Username                    string
	Password                    string
	DangerouslyStoreCredentials bool
	Options                     []GitOption
}

func (g *Git) Init(ctx context.Context, repoPath string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, "", []string{"init", repoPath}, opts...)
}

func (g *Git) RemoteAdd(ctx context.Context, repoPath, name, remoteURL string, overwrite bool, opts ...GitOption) (*CommandResult, error) {
	if overwrite {
		cmd := buildGitCommand([]string{"remote", "add", name, remoteURL}, repoPath) + " || " + buildGitCommand([]string{"remote", "set-url", name, remoteURL}, repoPath)
		return g.runShell(ctx, cmd, opts...)
	}
	return g.run(ctx, repoPath, []string{"remote", "add", name, remoteURL}, opts...)
}

func (g *Git) RemoteGet(ctx context.Context, repoPath, name string, opts ...GitOption) (string, error) {
	res, err := g.runShell(ctx, buildGitCommand([]string{"remote", "get-url", name}, repoPath)+" || true", opts...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (g *Git) Status(ctx context.Context, repoPath string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, repoPath, []string{"status", "--porcelain=1", "-b"}, opts...)
}

func (g *Git) Branches(ctx context.Context, repoPath string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, repoPath, []string{"branch", "--format=%(refname:short)\t%(HEAD)"}, opts...)
}

func (g *Git) CreateBranch(ctx context.Context, repoPath, branch string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, repoPath, []string{"checkout", "-b", branch}, opts...)
}

func (g *Git) CheckoutBranch(ctx context.Context, repoPath, branch string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, repoPath, []string{"checkout", branch}, opts...)
}

func (g *Git) DeleteBranch(ctx context.Context, repoPath, branch string, force bool, opts ...GitOption) (*CommandResult, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.run(ctx, repoPath, []string{"branch", flag, branch}, opts...)
}

func (g *Git) Add(ctx context.Context, repoPath string, files []string, all bool, opts ...GitOption) (*CommandResult, error) {
	args := []string{"add"}
	if len(files) == 0 {
		if all {
			args = append(args, "-A")
		} else {
			args = append(args, ".")
		}
	} else {
		args = append(args, "--")
		args = append(args, files...)
	}
	return g.run(ctx, repoPath, args, opts...)
}

func (g *Git) Commit(ctx context.Context, repoPath, message, authorName, authorEmail string, allowEmpty bool, opts ...GitOption) (*CommandResult, error) {
	args := []string{}
	if authorName != "" {
		args = append(args, "-c", "user.name="+authorName)
	}
	if authorEmail != "" {
		args = append(args, "-c", "user.email="+authorEmail)
	}
	args = append(args, "commit", "-m", message)
	if allowEmpty {
		args = append(args, "--allow-empty")
	}
	return g.run(ctx, repoPath, args, opts...)
}

func (g *Git) Reset(ctx context.Context, repoPath, mode, target string, paths []string, opts ...GitOption) (*CommandResult, error) {
	args := []string{"reset"}
	if mode != "" {
		args = append(args, "--"+mode)
	}
	if target != "" {
		args = append(args, target)
	}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return g.run(ctx, repoPath, args, opts...)
}

func (g *Git) Restore(ctx context.Context, repoPath string, paths []string, staged, worktree bool, source string, opts ...GitOption) (*CommandResult, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}
	args := []string{"restore"}
	if worktree {
		args = append(args, "--worktree")
	}
	if staged {
		args = append(args, "--staged")
	}
	if source != "" {
		args = append(args, "--source", source)
	}
	args = append(args, "--")
	args = append(args, paths...)
	return g.run(ctx, repoPath, args, opts...)
}

func (g *Git) Push(ctx context.Context, repoPath, remote, branch string, setUpstream bool, opts ...GitOption) (*CommandResult, error) {
	args := []string{"push"}
	if setUpstream {
		args = append(args, "--set-upstream")
	}
	if remote != "" {
		args = append(args, remote)
	}
	if branch != "" {
		args = append(args, branch)
	}
	return g.run(ctx, repoPath, args, opts...)
}

func (g *Git) Pull(ctx context.Context, repoPath, remote, branch string, opts ...GitOption) (*CommandResult, error) {
	args := []string{"pull"}
	if remote != "" {
		args = append(args, remote)
	}
	if branch != "" {
		args = append(args, branch)
	}
	return g.run(ctx, repoPath, args, opts...)
}

func (g *Git) SetConfig(ctx context.Context, key, value, scope, repoPath string, opts ...GitOption) (*CommandResult, error) {
	if scope == "" {
		scope = "global"
	}
	return g.run(ctx, repoPath, []string{"config", "--" + scope, key, value}, opts...)
}

func (g *Git) GetConfig(ctx context.Context, key, scope, repoPath string, opts ...GitOption) (string, error) {
	if scope == "" {
		scope = "global"
	}
	res, err := g.runShell(ctx, buildGitCommand([]string{"config", "--" + scope, "--get", key}, repoPath)+" || true", opts...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (g *Git) ConfigureUser(ctx context.Context, name, email string, opts ...GitOption) (*CommandResult, error) {
	return g.runShell(ctx, buildGitCommand([]string{"config", "--global", "user.name", name}, "")+" && "+buildGitCommand([]string{"config", "--global", "user.email", email}, ""), opts...)
}

// DangerouslyAuthenticate stores HTTPS git credentials in the sandbox global
// credential helper. Prefer short-lived tokens when using this helper.
func (g *Git) DangerouslyAuthenticate(ctx context.Context, username, password, host, protocol string, opts ...GitOption) (*CommandResult, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}
	if host == "" {
		host = "github.com"
	}
	if protocol == "" {
		protocol = "https"
	}
	input := strings.Join([]string{
		"protocol=" + protocol,
		"host=" + host,
		"username=" + username,
		"password=" + password,
		"",
		"",
	}, "\n")
	cmd := buildGitCommand([]string{"config", "--global", "credential.helper", "store"}, "") +
		" && printf %s " + shellQuote(input) + " | " + buildGitCommand([]string{"credential", "approve"}, "")
	return g.runShell(ctx, cmd, opts...)
}

func buildGitCommand(args []string, repoPath string) string {
	parts := []string{"git"}
	if repoPath != "" {
		parts = append(parts, "-C", repoPath)
	}
	parts = append(parts, args...)
	for i, p := range parts {
		parts[i] = shellQuote(p)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func urlWithCredentials(raw, username, password string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(username, password)
	return u.String(), nil
}

func stripURLCredentials(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	return u.String()
}

func deriveRepoDir(raw string) string {
	u, err := url.Parse(raw)
	var base string
	if err == nil && u.Path != "" {
		base = path.Base(u.Path)
	} else {
		base = path.Base(raw)
	}
	return strings.TrimSuffix(base, ".git")
}
