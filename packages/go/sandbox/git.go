package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

var defaultGitEnv = map[string]string{
	"GIT_TERMINAL_PROMPT": "0",
}

const credentialCleanupTimeout = 10 * time.Second

// Git provides common git operations inside a sandbox.
type Git struct {
	commands *Commands
}

type gitCommandError struct {
	Cmd    string
	Result *CommandResult
}

func (e *gitCommandError) Error() string {
	if e.Result == nil {
		return fmt.Sprintf("git %s failed", e.Cmd)
	}
	if stderr := strings.TrimSpace(e.Result.Stderr); stderr != "" {
		return fmt.Sprintf("git %s failed (exit %d): %s", e.Cmd, e.Result.ExitCode, stderr)
	}
	return fmt.Sprintf("git %s failed (exit %d)", e.Cmd, e.Result.ExitCode)
}

func (e *gitCommandError) Unwrap() error {
	if e.Result == nil {
		return nil
	}
	message := strings.ToLower(e.Result.Stdout + "\n" + e.Result.Stderr)
	switch {
	case containsAny(message, []string{
		"authentication failed",
		"terminal prompts disabled",
		"could not read username",
		"could not read password",
		"invalid username or password",
		"bad credentials",
		"requested url returned error: 401",
		"requested url returned error: 403",
		"http basic: access denied",
	}):
		return ErrGitAuth
	case containsAny(message, []string{
		"has no upstream branch",
		"no upstream branch",
		"no upstream configured",
		"no tracking information for the current branch",
		"no tracking information",
		"set the remote as upstream",
		"set the upstream branch",
		"please specify which branch you want to merge with",
	}):
		return ErrGitUpstream
	default:
		return nil
	}
}

func containsAny(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// GitFileStatus describes one changed file from git status.
type GitFileStatus struct {
	Name              string
	Status            string
	IndexStatus       string
	WorkingTreeStatus string
	Staged            bool
	RenamedFrom       string
}

// GitStatus describes repository status.
type GitStatus struct {
	CurrentBranch string
	Upstream      string
	Ahead         int
	Behind        int
	Detached      bool
	FileStatus    []GitFileStatus
}

func (s *GitStatus) IsClean() bool { return len(s.FileStatus) == 0 }

func (s *GitStatus) HasChanges() bool { return len(s.FileStatus) > 0 }

func (s *GitStatus) HasStaged() bool {
	for i := range s.FileStatus {
		if s.FileStatus[i].Staged {
			return true
		}
	}
	return false
}

func (s *GitStatus) TotalCount() int { return len(s.FileStatus) }

func (s *GitStatus) StagedCount() int {
	n := 0
	for i := range s.FileStatus {
		if s.FileStatus[i].Staged {
			n++
		}
	}
	return n
}

func (s *GitStatus) UntrackedCount() int {
	n := 0
	for i := range s.FileStatus {
		if s.FileStatus[i].Status == "untracked" {
			n++
		}
	}
	return n
}

// GitBranches describes local branches.
type GitBranches struct {
	Branches      []string
	CurrentBranch string
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
	cmd := buildGitCommand(args, repoPath)
	return g.runGitShell(ctx, args[0], cmd, opts...)
}

func (g *Git) runShell(ctx context.Context, cmd string, opts ...GitOption) (*CommandResult, error) {
	return g.runGitShell(ctx, "command", cmd, opts...)
}

func (g *Git) runGitShell(ctx context.Context, sub, cmd string, opts ...GitOption) (*CommandResult, error) {
	o := applyGitOpts(opts)
	result, err := g.commands.Run(ctx, cmd, gitCommandOptions(o.commandOpts)...)
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", sub, err)
	}
	if result.ExitCode != 0 {
		return result, &gitCommandError{Cmd: sub, Result: result}
	}
	return result, nil
}

func gitCommandOptions(opts []CommandOption) []CommandOption {
	co := applyCommandOpts(opts)
	envs := make(map[string]string, len(co.envs)+len(defaultGitEnv))
	for k, v := range co.envs {
		envs[k] = v
	}
	for k, v := range defaultGitEnv {
		envs[k] = v
	}
	out := []CommandOption{WithEnvs(envs)}
	if co.cwd != "" {
		out = append(out, WithCwd(co.cwd))
	}
	if co.user != "" {
		out = append(out, WithCommandUser(co.user))
	}
	if co.tag != "" {
		out = append(out, WithTag(co.tag))
	}
	if co.onStdout != nil {
		out = append(out, WithOnStdout(co.onStdout))
	}
	if co.onStderr != nil {
		out = append(out, WithOnStderr(co.onStderr))
	}
	if co.onPtyData != nil {
		out = append(out, WithOnPtyData(co.onPtyData))
	}
	if co.timeout > 0 {
		out = append(out, WithTimeout(co.timeout))
	}
	if co.stdin {
		out = append(out, WithStdin())
	}
	return out
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
			cleanupCtx, cancel := context.WithTimeout(context.Background(), credentialCleanupTimeout)
			defer cancel()
			if _, serr := g.run(cleanupCtx, repoPath, []string{"remote", "set-url", "origin", stripURLCredentials(cloneURL)}, opts.Options...); serr != nil {
				return res, fmt.Errorf("clone succeeded but failed to strip credentials: %w", serr)
			}
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

// PushOptions customizes Push.
type PushOptions struct {
	Remote      string
	Branch      string
	SetUpstream *bool
	Username    string
	Password    string
	Options     []GitOption
}

// PullOptions customizes Pull.
type PullOptions struct {
	Remote   string
	Branch   string
	Username string
	Password string
	Options  []GitOption
}

// InitOptions customizes Init.
type InitOptions struct {
	InitialBranch string
	Bare          bool
	Options       []GitOption
}

// RemoteAddOptions customizes RemoteAdd.
type RemoteAddOptions struct {
	Overwrite bool
	Fetch     bool
	Options   []GitOption
}

// AddOptions customizes Add.
type AddOptions struct {
	Files   []string
	All     *bool
	Options []GitOption
}

// CommitOptions customizes Commit.
type CommitOptions struct {
	AuthorName  string
	AuthorEmail string
	AllowEmpty  bool
	Options     []GitOption
}

// GitResetMode is a supported git reset mode.
type GitResetMode string

const (
	// GitResetModeSoft keeps index and working tree changes.
	GitResetModeSoft GitResetMode = "soft"
	// GitResetModeMixed resets the index and keeps working tree changes.
	GitResetModeMixed GitResetMode = "mixed"
	// GitResetModeHard resets both index and working tree changes.
	GitResetModeHard GitResetMode = "hard"
	// GitResetModeMerge resets while preserving unmerged entries when possible.
	GitResetModeMerge GitResetMode = "merge"
	// GitResetModeKeep resets while keeping local working tree changes.
	GitResetModeKeep GitResetMode = "keep"
)

// ResetOptions customizes Reset.
type ResetOptions struct {
	Mode    GitResetMode
	Target  string
	Paths   []string
	Options []GitOption
}

// RestoreOptions customizes Restore.
type RestoreOptions struct {
	Paths    []string
	Staged   *bool
	Worktree *bool
	Source   string
	Options  []GitOption
}

// GitConfigScope is a supported git config scope.
type GitConfigScope string

const (
	// GitConfigScopeLocal applies config to the repository at ConfigOptions.RepoPath.
	GitConfigScopeLocal GitConfigScope = "local"
	// GitConfigScopeGlobal applies config to the sandbox user's global git config.
	GitConfigScopeGlobal GitConfigScope = "global"
	// GitConfigScopeSystem applies config to the system git config.
	GitConfigScopeSystem GitConfigScope = "system"
	// GitConfigScopeWorktree applies config to the current git worktree.
	GitConfigScopeWorktree GitConfigScope = "worktree"
)

// ConfigOptions customizes SetConfig and GetConfig.
type ConfigOptions struct {
	Scope    GitConfigScope
	RepoPath string
	Options  []GitOption
}

// DeleteBranchOptions customizes DeleteBranch.
type DeleteBranchOptions struct {
	Force   bool
	Options []GitOption
}

func (g *Git) Init(ctx context.Context, repoPath string, opts *InitOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &InitOptions{}
	}
	args := []string{"init"}
	if opts.Bare {
		args = append(args, "--bare")
	}
	if opts.InitialBranch != "" {
		args = append(args, "--initial-branch", opts.InitialBranch)
	}
	args = append(args, repoPath)
	return g.run(ctx, "", args, opts.Options...)
}

func (g *Git) RemoteAdd(ctx context.Context, repoPath, name, remoteURL string, opts *RemoteAddOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &RemoteAddOptions{}
	}
	runAdd := func() (*CommandResult, error) {
		if opts.Overwrite {
			cmd := buildGitCommand([]string{"remote", "add", name, remoteURL}, repoPath) + " || " + buildGitCommand([]string{"remote", "set-url", name, remoteURL}, repoPath)
			return g.runShell(ctx, cmd, opts.Options...)
		}
		return g.run(ctx, repoPath, []string{"remote", "add", name, remoteURL}, opts.Options...)
	}
	result, err := runAdd()
	if err != nil {
		return result, err
	}
	if opts.Fetch {
		return g.run(ctx, repoPath, []string{"fetch", name}, opts.Options...)
	}
	return result, nil
}

func (g *Git) RemoteGet(ctx context.Context, repoPath, name string, opts ...GitOption) (string, error) {
	res, err := g.run(ctx, repoPath, []string{"remote", "get-url", name}, opts...)
	if err != nil {
		var gitErr *gitCommandError
		if errors.As(err, &gitErr) && gitErr.Result != nil && gitErr.Result.ExitCode == 2 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (g *Git) Status(ctx context.Context, repoPath string, opts ...GitOption) (*GitStatus, error) {
	res, err := g.run(ctx, repoPath, []string{"status", "--porcelain=1", "-b"}, opts...)
	if err != nil {
		return nil, err
	}
	return parseGitStatus(res.Stdout), nil
}

func (g *Git) Branches(ctx context.Context, repoPath string, opts ...GitOption) (*GitBranches, error) {
	res, err := g.run(ctx, repoPath, []string{"branch", "--format=%(refname:short)\t%(HEAD)"}, opts...)
	if err != nil {
		return nil, err
	}
	branches := parseGitBranches(res.Stdout)
	if len(branches.Branches) > 0 {
		return branches, nil
	}
	current, err := g.run(ctx, repoPath, []string{"symbolic-ref", "--short", "HEAD"}, opts...)
	if err != nil {
		return branches, nil
	}
	name := strings.TrimSpace(current.Stdout)
	if name != "" {
		branches.CurrentBranch = name
	}
	return branches, nil
}

func (g *Git) CreateBranch(ctx context.Context, repoPath, branch string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, repoPath, []string{"checkout", "-b", branch}, opts...)
}

func (g *Git) CheckoutBranch(ctx context.Context, repoPath, branch string, opts ...GitOption) (*CommandResult, error) {
	return g.run(ctx, repoPath, []string{"checkout", branch}, opts...)
}

func (g *Git) DeleteBranch(ctx context.Context, repoPath, branch string, opts *DeleteBranchOptions) (*CommandResult, error) {
	if branch == "" {
		return nil, fmt.Errorf("branch is required")
	}
	if opts == nil {
		opts = &DeleteBranchOptions{}
	}
	flag := "-d"
	if opts.Force {
		flag = "-D"
	}
	return g.run(ctx, repoPath, []string{"branch", flag, branch}, opts.Options...)
}

func (g *Git) Add(ctx context.Context, repoPath string, opts *AddOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &AddOptions{}
	}
	args := []string{"add"}
	if len(opts.Files) == 0 {
		all := true
		if opts.All != nil {
			all = *opts.All
		}
		if all {
			args = append(args, "-A")
		} else {
			args = append(args, ".")
		}
	} else {
		args = append(args, "--")
		args = append(args, opts.Files...)
	}
	return g.run(ctx, repoPath, args, opts.Options...)
}

func (g *Git) Commit(ctx context.Context, repoPath, message string, opts *CommitOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &CommitOptions{}
	}
	args := []string{}
	if opts.AuthorName != "" {
		args = append(args, "-c", "user.name="+opts.AuthorName)
	}
	if opts.AuthorEmail != "" {
		args = append(args, "-c", "user.email="+opts.AuthorEmail)
	}
	args = append(args, "commit", "-m", message)
	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}
	return g.run(ctx, repoPath, args, opts.Options...)
}

func (g *Git) Reset(ctx context.Context, repoPath string, opts *ResetOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &ResetOptions{}
	}
	if opts.Mode != "" {
		if !isValidGitResetMode(opts.Mode) {
			return nil, fmt.Errorf("unsupported reset mode %q", opts.Mode)
		}
		if len(opts.Paths) > 0 {
			return nil, fmt.Errorf("reset mode cannot be used with pathspecs")
		}
	}
	args := []string{"reset"}
	if opts.Mode != "" {
		args = append(args, "--"+string(opts.Mode))
	}
	if opts.Target != "" {
		args = append(args, opts.Target)
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}
	return g.run(ctx, repoPath, args, opts.Options...)
}

func (g *Git) Restore(ctx context.Context, repoPath string, opts *RestoreOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &RestoreOptions{}
	}
	if len(opts.Paths) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}
	if opts.Staged != nil && opts.Worktree != nil && !*opts.Staged && !*opts.Worktree {
		return nil, fmt.Errorf("at least one of Staged or Worktree must be true when both are set")
	}
	args := []string{"restore"}
	if opts.Worktree != nil && *opts.Worktree {
		args = append(args, "--worktree")
	}
	if opts.Staged != nil && *opts.Staged {
		args = append(args, "--staged")
	}
	if opts.Source != "" {
		args = append(args, "--source", opts.Source)
	}
	args = append(args, "--")
	args = append(args, opts.Paths...)
	return g.run(ctx, repoPath, args, opts.Options...)
}

// Push pushes commits to a remote repository.
//
// When Remote is empty and the repository has exactly one remote, that remote
// is selected automatically. SetUpstream defaults to true; set it to false
// explicitly to omit --set-upstream. Username and Password, when provided, are
// written to the remote URL only for the duration of the push and then removed.
func (g *Git) Push(ctx context.Context, repoPath string, opts *PushOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &PushOptions{}
	}
	if err := validateCredentialPair("push", opts.Username, opts.Password); err != nil {
		return nil, err
	}
	setUpstream := true
	if opts.SetUpstream != nil {
		setUpstream = *opts.SetUpstream
	}
	buildArgs := func(remote string) ([]string, error) {
		branch := opts.Branch
		if remote == "" {
			if branch != "" {
				return nil, fmt.Errorf("remote is required when branch is specified and the repository does not have a single remote")
			}
			return []string{"push"}, nil
		}
		if setUpstream && branch == "" {
			current, err := g.currentBranch(ctx, repoPath, opts.Options...)
			if err != nil {
				return nil, err
			}
			branch = current
		}
		args := []string{"push"}
		if setUpstream {
			args = append(args, "--set-upstream")
		}
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
		return args, nil
	}
	return g.runWithOptionalCredentials(ctx, "push", repoPath, opts.Remote, opts.Username, opts.Password, opts.Options, buildArgs)
}

// Pull pulls changes from a remote repository.
//
// When Remote is empty and the repository has exactly one remote, that remote
// is selected automatically. Username and Password, when provided, are written
// to the remote URL only for the duration of the pull and then removed.
func (g *Git) Pull(ctx context.Context, repoPath string, opts *PullOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &PullOptions{}
	}
	if err := validateCredentialPair("pull", opts.Username, opts.Password); err != nil {
		return nil, err
	}
	buildArgs := func(remote string) ([]string, error) {
		if remote == "" && opts.Branch != "" {
			return nil, fmt.Errorf("remote is required when branch is specified and the repository does not have a single remote")
		}
		args := []string{"pull"}
		if remote != "" {
			args = append(args, remote)
		}
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
		return args, nil
	}
	return g.runWithOptionalCredentials(ctx, "pull", repoPath, opts.Remote, opts.Username, opts.Password, opts.Options, buildArgs)
}

func (g *Git) SetConfig(ctx context.Context, key, value string, opts *ConfigOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &ConfigOptions{}
	}
	scope := opts.Scope
	if scope == "" {
		scope = GitConfigScopeGlobal
	}
	if err := validateGitConfigOptions(scope, opts.RepoPath); err != nil {
		return nil, err
	}
	return g.run(ctx, opts.RepoPath, []string{"config", "--" + string(scope), key, value}, opts.Options...)
}

func (g *Git) GetConfig(ctx context.Context, key string, opts *ConfigOptions) (string, error) {
	if opts == nil {
		opts = &ConfigOptions{}
	}
	scope := opts.Scope
	if scope == "" {
		scope = GitConfigScopeGlobal
	}
	if err := validateGitConfigOptions(scope, opts.RepoPath); err != nil {
		return "", err
	}
	res, err := g.run(ctx, opts.RepoPath, []string{"config", "--" + string(scope), "--get", key}, opts.Options...)
	if err != nil {
		var gitErr *gitCommandError
		if errors.As(err, &gitErr) && gitErr.Result != nil &&
			gitErr.Result.ExitCode == 1 &&
			strings.TrimSpace(gitErr.Result.Stdout) == "" &&
			strings.TrimSpace(gitErr.Result.Stderr) == "" {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (g *Git) ConfigureUser(ctx context.Context, name, email string, opts *ConfigOptions) (*CommandResult, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if _, err := g.SetConfig(ctx, "user.name", name, opts); err != nil {
		return nil, err
	}
	return g.SetConfig(ctx, "user.email", email, opts)
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
	if protocol != "https" {
		return nil, fmt.Errorf("only https protocol is supported for git authentication")
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
	if u.Scheme != "https" {
		return "", fmt.Errorf("only https git URLs support username/password credentials")
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

func parseGitStatus(out string) *GitStatus {
	status := &GitStatus{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			parseGitStatusBranch(line[3:], status)
			continue
		}
		if file, ok := parseGitFileStatus(line); ok {
			status.FileStatus = append(status.FileStatus, file)
		}
	}
	return status
}

func parseGitStatusBranch(line string, status *GitStatus) {
	branchPart := line
	trackingPart := ""
	if i := strings.Index(line, " ["); i >= 0 && strings.HasSuffix(line, "]") {
		branchPart = line[:i]
		trackingPart = line[i+2 : len(line)-1]
	}

	switch {
	case strings.HasPrefix(branchPart, "HEAD (no branch)"),
		strings.HasPrefix(branchPart, "HEAD (detached"),
		strings.HasPrefix(branchPart, "(no branch)"):
		status.Detached = true
	case strings.HasPrefix(branchPart, "No commits yet on "):
		status.CurrentBranch = strings.TrimPrefix(branchPart, "No commits yet on ")
	case strings.HasPrefix(branchPart, "Initial commit on "):
		status.CurrentBranch = strings.TrimPrefix(branchPart, "Initial commit on ")
	default:
		if branch, upstream, ok := strings.Cut(branchPart, "..."); ok {
			status.CurrentBranch = branch
			status.Upstream = upstream
		} else {
			status.CurrentBranch = branchPart
		}
	}

	for _, part := range strings.Split(trackingPart, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "ahead "):
			_, _ = fmt.Sscanf(part, "ahead %d", &status.Ahead)
		case strings.HasPrefix(part, "behind "):
			_, _ = fmt.Sscanf(part, "behind %d", &status.Behind)
		}
	}
}

func parseGitFileStatus(line string) (GitFileStatus, bool) {
	if len(line) < 3 {
		return GitFileStatus{}, false
	}
	x := string(line[0])
	y := string(line[1])
	name := line[3:]

	file := GitFileStatus{
		IndexStatus:       x,
		WorkingTreeStatus: y,
		Staged:            x != " " && x != "?",
	}

	if x == "R" || x == "C" || y == "R" || y == "C" {
		before, after, ok := strings.Cut(name, " -> ")
		if ok {
			file.RenamedFrom = unquoteGitPath(before)
			file.Name = unquoteGitPath(after)
		} else {
			file.Name = unquoteGitPath(name)
		}
	} else {
		file.Name = unquoteGitPath(name)
	}
	file.Status = normalizeGitFileStatus(x, y)
	return file, true
}

func normalizeGitFileStatus(x, y string) string {
	switch {
	case x == "?" && y == "?":
		return "untracked"
	case x == "!" && y == "!":
		return "ignored"
	case x == "R" || y == "R":
		return "renamed"
	case x == "C" || y == "C":
		return "copied"
	case x == "U" || y == "U" || (x == "A" && y == "A") || (x == "D" && y == "D"):
		return "conflict"
	case x == "A" || y == "A":
		return "added"
	case x == "D" || y == "D":
		return "deleted"
	case x == "M" || y == "M":
		return "modified"
	default:
		return "unknown"
	}
}

func unquoteGitPath(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if unquoted, err := strconv.Unquote(s); err == nil {
			return unquoted
		}
	}
	return s
}

func parseGitBranches(out string) *GitBranches {
	branches := &GitBranches{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, mark, ok := strings.Cut(line, "\t")
		if !ok {
			name = line
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		branches.Branches = append(branches.Branches, name)
		if strings.TrimSpace(mark) == "*" {
			branches.CurrentBranch = name
		}
	}
	return branches
}

func isValidGitResetMode(mode GitResetMode) bool {
	switch mode {
	case GitResetModeSoft, GitResetModeMixed, GitResetModeHard, GitResetModeMerge, GitResetModeKeep:
		return true
	default:
		return false
	}
}

func validateGitConfigOptions(scope GitConfigScope, repoPath string) error {
	switch scope {
	case GitConfigScopeLocal, GitConfigScopeGlobal, GitConfigScopeSystem, GitConfigScopeWorktree:
	default:
		return fmt.Errorf("unsupported git config scope %q", scope)
	}
	if scope == GitConfigScopeLocal && repoPath == "" {
		return fmt.Errorf("RepoPath is required when Scope is local")
	}
	return nil
}

func validateCredentialPair(verb, username, password string) error {
	if (username == "") == (password == "") {
		return nil
	}
	return fmt.Errorf("username and password must be provided together for git %s", verb)
}

func (g *Git) currentBranch(ctx context.Context, repoPath string, opts ...GitOption) (string, error) {
	result, err := g.run(ctx, repoPath, []string{"rev-parse", "--abbrev-ref", "HEAD"}, opts...)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(result.Stdout)
	if name == "" || name == "HEAD" {
		return "", fmt.Errorf("cannot push with setUpstream on a detached HEAD; specify Branch explicitly or set SetUpstream=false")
	}
	return name, nil
}

func (g *Git) runWithOptionalCredentials(
	ctx context.Context,
	sub, repoPath, remote, username, password string,
	opts []GitOption,
	buildArgs func(remote string) ([]string, error),
) (*CommandResult, error) {
	if username == "" && password == "" {
		target := remote
		if target == "" {
			selected, err := g.autoSelectRemote(ctx, repoPath, opts...)
			if err != nil {
				return nil, err
			}
			target = selected
		}
		args, err := buildArgs(target)
		if err != nil {
			return nil, err
		}
		return g.run(ctx, repoPath, args, opts...)
	}

	remoteName, originalURL, err := g.resolveRemote(ctx, repoPath, remote, opts...)
	if err != nil {
		return nil, err
	}
	args, err := buildArgs(remoteName)
	if err != nil {
		return nil, err
	}
	var result *CommandResult
	err = g.withRemoteCredentials(ctx, repoPath, remoteName, originalURL, username, password, opts, func() error {
		r, runErr := g.run(ctx, repoPath, args, opts...)
		result = r
		return runErr
	})
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", sub, err)
	}
	return result, nil
}

func (g *Git) autoSelectRemote(ctx context.Context, repoPath string, opts ...GitOption) (string, error) {
	remotes, err := g.listRemotes(ctx, repoPath, opts...)
	if err != nil {
		return "", err
	}
	if len(remotes) == 1 {
		return remotes[0], nil
	}
	return "", nil
}

func (g *Git) listRemotes(ctx context.Context, repoPath string, opts ...GitOption) ([]string, error) {
	result, err := g.run(ctx, repoPath, []string{"remote"}, opts...)
	if err != nil {
		return nil, err
	}
	var remotes []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		if remote := strings.TrimSpace(line); remote != "" {
			remotes = append(remotes, remote)
		}
	}
	return remotes, nil
}

func (g *Git) resolveRemote(ctx context.Context, repoPath, remote string, opts ...GitOption) (string, string, error) {
	name := remote
	if name == "" {
		remotes, err := g.listRemotes(ctx, repoPath, opts...)
		if err != nil {
			return "", "", err
		}
		switch len(remotes) {
		case 0:
			return "", "", fmt.Errorf("repository has no remote configured")
		case 1:
			name = remotes[0]
		default:
			return "", "", fmt.Errorf("remote is required when using username/password and the repository has multiple remotes")
		}
	}
	remoteURL, err := g.RemoteGet(ctx, repoPath, name, opts...)
	if err != nil {
		return "", "", err
	}
	if remoteURL == "" {
		return "", "", fmt.Errorf("remote %q is not configured", name)
	}
	return name, remoteURL, nil
}

func (g *Git) withRemoteCredentials(ctx context.Context, repoPath, remote, originalURL, username, password string, opts []GitOption, fn func() error) (err error) {
	authedURL, err := urlWithCredentials(originalURL, username, password)
	if err != nil {
		return err
	}
	if authedURL == originalURL {
		return fn()
	}
	if _, err = g.run(ctx, repoPath, []string{"remote", "set-url", remote, authedURL}, opts...); err != nil {
		return err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), credentialCleanupTimeout)
		defer cancel()
		_, restoreErr := g.run(cleanupCtx, repoPath, []string{"remote", "set-url", remote, originalURL}, opts...)
		err = errors.Join(err, restoreErr)
	}()
	return fn()
}
