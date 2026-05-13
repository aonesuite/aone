package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process/processconnect"
)

type gitProcessHandler struct {
	exitCode int32
	stdout   string
	stderr   string

	responses []gitProcessResponse
	startReq  *process.StartRequest
	startReqs []*process.StartRequest
}

type gitProcessResponse struct {
	exitCode int32
	stdout   string
	stderr   string
}

func (h *gitProcessHandler) List(context.Context, *connect.Request[process.ListRequest]) (*connect.Response[process.ListResponse], error) {
	return connect.NewResponse(&process.ListResponse{}), nil
}

func (h *gitProcessHandler) Connect(context.Context, *connect.Request[process.ConnectRequest], *connect.ServerStream[process.ConnectResponse]) error {
	return nil
}

func (h *gitProcessHandler) Start(_ context.Context, req *connect.Request[process.StartRequest], stream *connect.ServerStream[process.StartResponse]) error {
	h.startReq = req.Msg
	h.startReqs = append(h.startReqs, req.Msg)
	resp := gitProcessResponse{exitCode: h.exitCode, stdout: h.stdout, stderr: h.stderr}
	if len(h.responses) >= len(h.startReqs) {
		resp = h.responses[len(h.startReqs)-1]
	}
	if err := stream.Send(&process.StartResponse{Event: &process.ProcessEvent{
		Event: &process.ProcessEvent_Start{Start: &process.ProcessEvent_StartEvent{Pid: 123}},
	}}); err != nil {
		return err
	}
	if resp.stdout != "" {
		if err := stream.Send(&process.StartResponse{Event: &process.ProcessEvent{
			Event: &process.ProcessEvent_Data{Data: &process.ProcessEvent_DataEvent{
				Output: &process.ProcessEvent_DataEvent_Stdout{Stdout: []byte(resp.stdout)},
			}},
		}}); err != nil {
			return err
		}
	}
	if resp.stderr != "" {
		if err := stream.Send(&process.StartResponse{Event: &process.ProcessEvent{
			Event: &process.ProcessEvent_Data{Data: &process.ProcessEvent_DataEvent{
				Output: &process.ProcessEvent_DataEvent_Stderr{Stderr: []byte(resp.stderr)},
			}},
		}}); err != nil {
			return err
		}
	}
	return stream.Send(&process.StartResponse{Event: &process.ProcessEvent{
		Event: &process.ProcessEvent_End{End: &process.ProcessEvent_EndEvent{
			ExitCode: resp.exitCode,
			Exited:   true,
		}},
	}})
}

func (h *gitProcessHandler) Update(context.Context, *connect.Request[process.UpdateRequest]) (*connect.Response[process.UpdateResponse], error) {
	return connect.NewResponse(&process.UpdateResponse{}), nil
}

func (h *gitProcessHandler) StreamInput(context.Context, *connect.ClientStream[process.StreamInputRequest]) (*connect.Response[process.StreamInputResponse], error) {
	return connect.NewResponse(&process.StreamInputResponse{}), nil
}

func (h *gitProcessHandler) SendInput(context.Context, *connect.Request[process.SendInputRequest]) (*connect.Response[process.SendInputResponse], error) {
	return connect.NewResponse(&process.SendInputResponse{}), nil
}

func (h *gitProcessHandler) SendSignal(context.Context, *connect.Request[process.SendSignalRequest]) (*connect.Response[process.SendSignalResponse], error) {
	return connect.NewResponse(&process.SendSignalResponse{}), nil
}

func (h *gitProcessHandler) CloseStdin(context.Context, *connect.Request[process.CloseStdinRequest]) (*connect.Response[process.CloseStdinResponse], error) {
	return connect.NewResponse(&process.CloseStdinResponse{}), nil
}

func newGitForTest(t *testing.T, h *gitProcessHandler) *Git {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := processconnect.NewProcessHandler(h)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	commands := &Commands{
		sandbox: &Sandbox{},
		rpc:     processconnect.NewProcessClient(srv.Client(), srv.URL),
	}
	return &Git{commands: commands}
}

func TestGitReturnsErrorForNonZeroExit(t *testing.T) {
	git := newGitForTest(t, &gitProcessHandler{
		exitCode: 128,
		stderr:   "fatal: not a git repository",
	})

	_, err := git.Status(context.Background(), "/repo")
	if err == nil {
		t.Fatal("Status returned nil error for non-zero git exit")
	}
	if got, want := err.Error(), "git status failed (exit 128): fatal: not a git repository"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestGitMapsAuthAndUpstreamErrors(t *testing.T) {
	cases := []struct {
		name string
		err  string
		want error
	}{
		{
			name: "auth",
			err:  "fatal: Authentication failed for 'https://github.com/acme/repo.git'",
			want: ErrGitAuth,
		},
		{
			name: "upstream",
			err:  "fatal: The current branch main has no upstream branch.",
			want: ErrGitUpstream,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := newGitForTest(t, &gitProcessHandler{
				exitCode: 128,
				stderr:   tc.err,
			})

			_, err := git.Status(context.Background(), "/repo")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, tc.want)
			}
		})
	}
}

func TestGitForcesTerminalPromptOff(t *testing.T) {
	h := &gitProcessHandler{}
	git := newGitForTest(t, h)

	_, err := git.Status(context.Background(), "/repo",
		WithGitCommandOptions(WithEnvs(map[string]string{
			"GIT_AUTHOR_DATE":     "2026-05-13T00:00:00Z",
			"GIT_TERMINAL_PROMPT": "1",
		})))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if h.startReq == nil || h.startReq.Process == nil {
		t.Fatal("process start request was not captured")
	}
	if got := h.startReq.Process.Envs["GIT_TERMINAL_PROMPT"]; got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want %q; envs=%v", got, "0", h.startReq.Process.Envs)
	}
	if got := h.startReq.Process.Envs["GIT_AUTHOR_DATE"]; got != "2026-05-13T00:00:00Z" {
		t.Fatalf("GIT_AUTHOR_DATE = %q, want caller-provided value; envs=%v", got, h.startReq.Process.Envs)
	}
	if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' 'status' '--porcelain=1' '-b']"; got != want {
		t.Fatalf("args = %s, want %s", got, want)
	}
}

func TestGitStatusParsesPorcelainOutput(t *testing.T) {
	h := &gitProcessHandler{stdout: "## main...origin/main [ahead 2, behind 1]\n M modified.txt\nA  staged.txt\n?? \"with space.txt\"\nR  old.txt -> new.txt\n"}
	git := newGitForTest(t, h)

	status, err := git.Status(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.CurrentBranch != "main" || status.Upstream != "origin/main" || status.Ahead != 2 || status.Behind != 1 {
		t.Fatalf("unexpected branch status: %+v", status)
	}
	if status.IsClean() {
		t.Fatal("status should not be clean")
	}
	if got, want := status.TotalCount(), 4; got != want {
		t.Fatalf("TotalCount = %d, want %d", got, want)
	}
	if got, want := status.StagedCount(), 2; got != want {
		t.Fatalf("StagedCount = %d, want %d", got, want)
	}
	if got, want := status.UntrackedCount(), 1; got != want {
		t.Fatalf("UntrackedCount = %d, want %d", got, want)
	}
	if got := status.FileStatus[2].Name; got != "with space.txt" {
		t.Fatalf("quoted path = %q, want with space.txt", got)
	}
	rename := status.FileStatus[3]
	if rename.Name != "new.txt" || rename.RenamedFrom != "old.txt" || rename.Status != "renamed" {
		t.Fatalf("rename entry = %+v", rename)
	}
}

func TestGitStatusParsesDetachedAndUnbornBranches(t *testing.T) {
	cases := []struct {
		name     string
		stdout   string
		branch   string
		detached bool
	}{
		{name: "unborn", stdout: "## No commits yet on main\n", branch: "main"},
		{name: "detached", stdout: "## HEAD (detached at abc123)\n", detached: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := newGitForTest(t, &gitProcessHandler{stdout: tc.stdout})
			status, err := git.Status(context.Background(), "/repo")
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if status.CurrentBranch != tc.branch || status.Detached != tc.detached {
				t.Fatalf("status = %+v, want branch=%q detached=%v", status, tc.branch, tc.detached)
			}
		})
	}
}

func TestGitBranchesParsesCurrentBranch(t *testing.T) {
	git := newGitForTest(t, &gitProcessHandler{stdout: "main\t*\nfeature\t \n"})

	branches, err := git.Branches(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if got, want := fmt.Sprint(branches.Branches), "[main feature]"; got != want {
		t.Fatalf("Branches = %s, want %s", got, want)
	}
	if branches.CurrentBranch != "main" {
		t.Fatalf("CurrentBranch = %q, want main", branches.CurrentBranch)
	}
}

func TestGitBranchesFallsBackToCurrentBranchForUnbornRepository(t *testing.T) {
	h := &gitProcessHandler{responses: []gitProcessResponse{
		{},
		{stdout: "main\n"},
	}}
	git := newGitForTest(t, h)

	branches, err := git.Branches(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if got, want := fmt.Sprint(branches.Branches), "[]"; got != want {
		t.Fatalf("Branches = %s, want %s", got, want)
	}
	if branches.CurrentBranch != "main" {
		t.Fatalf("CurrentBranch = %q, want main", branches.CurrentBranch)
	}
	if len(h.startReqs) != 2 {
		t.Fatalf("start requests = %d, want 2", len(h.startReqs))
	}
	if got, want := fmt.Sprint(h.startReqs[1].Process.Args), "[-l -c 'git' '-C' '/repo' 'symbolic-ref' '--short' 'HEAD']"; got != want {
		t.Fatalf("fallback args = %s, want %s", got, want)
	}
}

func TestGitRemoteGetDistinguishesMissingRemoteFromRepositoryErrors(t *testing.T) {
	t.Run("missing remote returns empty", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{
			exitCode: 2,
			stderr:   "error: No such remote 'origin'",
		})

		got, err := git.RemoteGet(context.Background(), "/repo", "origin")
		if err != nil {
			t.Fatalf("RemoteGet missing remote: %v", err)
		}
		if got != "" {
			t.Fatalf("RemoteGet = %q, want empty", got)
		}
	})

	t.Run("repository error is returned", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{
			exitCode: 128,
			stderr:   "fatal: not a git repository",
		})

		_, err := git.RemoteGet(context.Background(), "/repo", "origin")
		if err == nil {
			t.Fatal("RemoteGet returned nil error for repository failure")
		}
		if got, want := err.Error(), "git remote failed (exit 128): fatal: not a git repository"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

func TestGitGetConfigDistinguishesMissingKeyFromRepositoryErrors(t *testing.T) {
	t.Run("missing key returns empty", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{exitCode: 1})

		got, err := git.GetConfig(context.Background(), "user.name", &ConfigOptions{Scope: GitConfigScopeLocal, RepoPath: "/repo"})
		if err != nil {
			t.Fatalf("GetConfig missing key: %v", err)
		}
		if got != "" {
			t.Fatalf("GetConfig = %q, want empty", got)
		}
	})

	t.Run("repository error is returned", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{
			exitCode: 128,
			stderr:   "fatal: not in a git directory",
		})

		_, err := git.GetConfig(context.Background(), "user.name", &ConfigOptions{Scope: GitConfigScopeLocal, RepoPath: "/repo"})
		if err == nil {
			t.Fatal("GetConfig returned nil error for repository failure")
		}
		if got, want := err.Error(), "git config failed (exit 128): fatal: not in a git directory"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

func TestGitAddUsesOptionsWithPointerDefault(t *testing.T) {
	cases := []struct {
		name string
		opts *AddOptions
		want string
	}{
		{name: "default all files", want: "[-l -c 'git' '-C' '/repo' 'add' '-A']"},
		{name: "explicit all", opts: &AddOptions{All: gitBoolPtr(true)}, want: "[-l -c 'git' '-C' '/repo' 'add' '-A']"},
		{name: "explicit false", opts: &AddOptions{All: gitBoolPtr(false)}, want: "[-l -c 'git' '-C' '/repo' 'add' '.']"},
		{name: "pathspecs", opts: &AddOptions{Files: []string{"README.md"}}, want: "[-l -c 'git' '-C' '/repo' 'add' '--' 'README.md']"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &gitProcessHandler{}
			git := newGitForTest(t, h)

			_, err := git.Add(context.Background(), "/repo", tc.opts)
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if got := fmt.Sprint(h.startReq.Process.Args); got != tc.want {
				t.Fatalf("args = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestGitRestoreUsesOptionsWithPointerFlags(t *testing.T) {
	h := &gitProcessHandler{}
	git := newGitForTest(t, h)

	_, err := git.Restore(context.Background(), "/repo", &RestoreOptions{
		Paths:    []string{"README.md"},
		Staged:   gitBoolPtr(false),
		Worktree: gitBoolPtr(true),
		Source:   "HEAD~1",
	})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' 'restore' '--worktree' '--source' 'HEAD~1' '--' 'README.md']"; got != want {
		t.Fatalf("args = %s, want %s", got, want)
	}
}

func TestGitRestoreRejectsExplicitFalseForBothTargets(t *testing.T) {
	git := newGitForTest(t, &gitProcessHandler{})

	_, err := git.Restore(context.Background(), "/repo", &RestoreOptions{
		Paths:    []string{"README.md"},
		Staged:   gitBoolPtr(false),
		Worktree: gitBoolPtr(false),
	})
	if err == nil {
		t.Fatal("Restore returned nil error for explicit false staged and worktree")
	}
	if got, want := err.Error(), "at least one of Staged or Worktree must be true when both are set"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestGitRemoteAddCommitResetAndSetConfigUseOptions(t *testing.T) {
	t.Run("init bare with initial branch", func(t *testing.T) {
		h := &gitProcessHandler{}
		git := newGitForTest(t, h)

		_, err := git.Init(context.Background(), "/repo.git", &InitOptions{
			Bare:          true,
			InitialBranch: "main",
		})
		if err != nil {
			t.Fatalf("Init: %v", err)
		}
		if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' 'init' '--bare' '--initial-branch' 'main' '/repo.git']"; got != want {
			t.Fatalf("args = %s, want %s", got, want)
		}
	})

	t.Run("remote add overwrite", func(t *testing.T) {
		h := &gitProcessHandler{}
		git := newGitForTest(t, h)

		_, err := git.RemoteAdd(context.Background(), "/repo", "origin", "https://example.com/repo.git", &RemoteAddOptions{Overwrite: true})
		if err != nil {
			t.Fatalf("RemoteAdd: %v", err)
		}
		if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' 'remote' 'add' 'origin' 'https://example.com/repo.git' || 'git' '-C' '/repo' 'remote' 'set-url' 'origin' 'https://example.com/repo.git']"; got != want {
			t.Fatalf("args = %s, want %s", got, want)
		}
	})

	t.Run("remote add fetch", func(t *testing.T) {
		h := &gitProcessHandler{responses: []gitProcessResponse{
			{},
			{},
		}}
		git := newGitForTest(t, h)

		_, err := git.RemoteAdd(context.Background(), "/repo", "origin", "https://example.com/repo.git", &RemoteAddOptions{Fetch: true})
		if err != nil {
			t.Fatalf("RemoteAdd fetch: %v", err)
		}
		if len(h.startReqs) != 2 {
			t.Fatalf("start requests = %d, want 2", len(h.startReqs))
		}
		if got, want := fmt.Sprint(h.startReqs[0].Process.Args), "[-l -c 'git' '-C' '/repo' 'remote' 'add' 'origin' 'https://example.com/repo.git']"; got != want {
			t.Fatalf("remote add args = %s, want %s", got, want)
		}
		if got, want := fmt.Sprint(h.startReqs[1].Process.Args), "[-l -c 'git' '-C' '/repo' 'fetch' 'origin']"; got != want {
			t.Fatalf("fetch args = %s, want %s", got, want)
		}
	})

	t.Run("commit author and allow empty", func(t *testing.T) {
		h := &gitProcessHandler{}
		git := newGitForTest(t, h)

		_, err := git.Commit(context.Background(), "/repo", "test: message", &CommitOptions{
			AuthorName:  "Alice",
			AuthorEmail: "alice@example.com",
			AllowEmpty:  true,
		})
		if err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' '-c' 'user.name=Alice' '-c' 'user.email=alice@example.com' 'commit' '-m' 'test: message' '--allow-empty']"; got != want {
			t.Fatalf("args = %s, want %s", got, want)
		}
	})

	t.Run("reset mode target and paths", func(t *testing.T) {
		h := &gitProcessHandler{}
		git := newGitForTest(t, h)

		_, err := git.Reset(context.Background(), "/repo", &ResetOptions{
			Mode:   GitResetModeHard,
			Target: "HEAD~1",
		})
		if err != nil {
			t.Fatalf("Reset: %v", err)
		}
		if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' 'reset' '--hard' 'HEAD~1']"; got != want {
			t.Fatalf("args = %s, want %s", got, want)
		}
	})

	t.Run("set local config", func(t *testing.T) {
		h := &gitProcessHandler{}
		git := newGitForTest(t, h)

		_, err := git.SetConfig(context.Background(), "user.name", "Alice", &ConfigOptions{Scope: GitConfigScopeLocal, RepoPath: "/repo"})
		if err != nil {
			t.Fatalf("SetConfig: %v", err)
		}
		if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' 'config' '--local' 'user.name' 'Alice']"; got != want {
			t.Fatalf("args = %s, want %s", got, want)
		}
	})
}

func TestGitDeleteBranchUsesOptionsAndRejectsEmptyBranch(t *testing.T) {
	t.Run("force delete", func(t *testing.T) {
		h := &gitProcessHandler{}
		git := newGitForTest(t, h)

		_, err := git.DeleteBranch(context.Background(), "/repo", "feature", &DeleteBranchOptions{Force: true})
		if err != nil {
			t.Fatalf("DeleteBranch: %v", err)
		}
		if got, want := fmt.Sprint(h.startReq.Process.Args), "[-l -c 'git' '-C' '/repo' 'branch' '-D' 'feature']"; got != want {
			t.Fatalf("args = %s, want %s", got, want)
		}
	})

	t.Run("empty branch", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{})

		_, err := git.DeleteBranch(context.Background(), "/repo", "", nil)
		if err == nil {
			t.Fatal("DeleteBranch returned nil error for empty branch")
		}
		if got, want := err.Error(), "branch is required"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

func TestGitResetRejectsInvalidModeAndModeWithPaths(t *testing.T) {
	t.Run("invalid mode", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{})

		_, err := git.Reset(context.Background(), "/repo", &ResetOptions{Mode: GitResetMode("invalid")})
		if err == nil {
			t.Fatal("Reset returned nil error for invalid mode")
		}
		if got, want := err.Error(), `unsupported reset mode "invalid"`; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})

	t.Run("mode with paths", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{})

		_, err := git.Reset(context.Background(), "/repo", &ResetOptions{
			Mode:  GitResetModeHard,
			Paths: []string{"README.md"},
		})
		if err == nil {
			t.Fatal("Reset returned nil error for mode with paths")
		}
		if got, want := err.Error(), "reset mode cannot be used with pathspecs"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

func TestGitConfigLocalScopeRequiresRepoPath(t *testing.T) {
	git := newGitForTest(t, &gitProcessHandler{})

	_, err := git.SetConfig(context.Background(), "user.name", "Alice", &ConfigOptions{Scope: GitConfigScopeLocal})
	if err == nil {
		t.Fatal("SetConfig returned nil error for local scope without repo path")
	}
	if got, want := err.Error(), "RepoPath is required when Scope is local"; got != want {
		t.Fatalf("SetConfig error = %q, want %q", got, want)
	}

	_, err = git.GetConfig(context.Background(), "user.name", &ConfigOptions{Scope: GitConfigScopeLocal})
	if err == nil {
		t.Fatal("GetConfig returned nil error for local scope without repo path")
	}
	if got, want := err.Error(), "RepoPath is required when Scope is local"; got != want {
		t.Fatalf("GetConfig error = %q, want %q", got, want)
	}
}

func TestGitConfigRejectsUnsupportedScope(t *testing.T) {
	git := newGitForTest(t, &gitProcessHandler{})

	_, err := git.SetConfig(context.Background(), "user.name", "Alice", &ConfigOptions{Scope: GitConfigScope("workspace")})
	if err == nil {
		t.Fatal("SetConfig returned nil error for unsupported scope")
	}
	if got, want := err.Error(), `unsupported git config scope "workspace"`; got != want {
		t.Fatalf("SetConfig error = %q, want %q", got, want)
	}

	_, err = git.GetConfig(context.Background(), "user.name", &ConfigOptions{Scope: GitConfigScope("workspace")})
	if err == nil {
		t.Fatal("GetConfig returned nil error for unsupported scope")
	}
	if got, want := err.Error(), `unsupported git config scope "workspace"`; got != want {
		t.Fatalf("GetConfig error = %q, want %q", got, want)
	}
}

func TestGitConfigureUserUsesConfigOptionsAndValidatesInput(t *testing.T) {
	t.Run("local config", func(t *testing.T) {
		h := &gitProcessHandler{responses: []gitProcessResponse{
			{},
			{},
		}}
		git := newGitForTest(t, h)

		_, err := git.ConfigureUser(context.Background(), "Alice", "alice@example.com", &ConfigOptions{
			Scope:    GitConfigScopeLocal,
			RepoPath: "/repo",
		})
		if err != nil {
			t.Fatalf("ConfigureUser: %v", err)
		}
		if len(h.startReqs) != 2 {
			t.Fatalf("start requests = %d, want 2", len(h.startReqs))
		}
		if got, want := fmt.Sprint(h.startReqs[0].Process.Args), "[-l -c 'git' '-C' '/repo' 'config' '--local' 'user.name' 'Alice']"; got != want {
			t.Fatalf("user.name args = %s, want %s", got, want)
		}
		if got, want := fmt.Sprint(h.startReqs[1].Process.Args), "[-l -c 'git' '-C' '/repo' 'config' '--local' 'user.email' 'alice@example.com']"; got != want {
			t.Fatalf("user.email args = %s, want %s", got, want)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{})

		_, err := git.ConfigureUser(context.Background(), "", "alice@example.com", nil)
		if err == nil {
			t.Fatal("ConfigureUser returned nil error for empty name")
		}
		if got, want := err.Error(), "name is required"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		git := newGitForTest(t, &gitProcessHandler{})

		_, err := git.ConfigureUser(context.Background(), "Alice", "", nil)
		if err == nil {
			t.Fatal("ConfigureUser returned nil error for empty email")
		}
		if got, want := err.Error(), "email is required"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

func TestGitPushAutoSelectsSingleRemoteAndDefaultsSetUpstream(t *testing.T) {
	h := &gitProcessHandler{responses: []gitProcessResponse{
		{stdout: "origin\n"},
		{stdout: "main\n"},
		{},
	}}
	git := newGitForTest(t, h)

	_, err := git.Push(context.Background(), "/repo", nil)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(h.startReqs) != 3 {
		t.Fatalf("start requests = %d, want 3", len(h.startReqs))
	}
	if got, want := fmt.Sprint(h.startReqs[2].Process.Args), "[-l -c 'git' '-C' '/repo' 'push' '--set-upstream' 'origin' 'main']"; got != want {
		t.Fatalf("push args = %s, want %s", got, want)
	}
}

func TestGitPushTemporarilyInjectsAndRestoresCredentials(t *testing.T) {
	h := &gitProcessHandler{responses: []gitProcessResponse{
		{stdout: "https://github.com/acme/repo.git\n"},
		{},
		{},
		{},
	}}
	git := newGitForTest(t, h)

	setUpstream := false
	_, err := git.Push(context.Background(), "/repo", &PushOptions{
		Remote:      "origin",
		Branch:      "main",
		SetUpstream: &setUpstream,
		Username:    "alice",
		Password:    "secret",
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(h.startReqs) != 4 {
		t.Fatalf("start requests = %d, want 4", len(h.startReqs))
	}
	if got, want := fmt.Sprint(h.startReqs[1].Process.Args), "[-l -c 'git' '-C' '/repo' 'remote' 'set-url' 'origin' 'https://alice:secret@github.com/acme/repo.git']"; got != want {
		t.Fatalf("credential injection args = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(h.startReqs[2].Process.Args), "[-l -c 'git' '-C' '/repo' 'push' 'origin' 'main']"; got != want {
		t.Fatalf("push args = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(h.startReqs[3].Process.Args), "[-l -c 'git' '-C' '/repo' 'remote' 'set-url' 'origin' 'https://github.com/acme/repo.git']"; got != want {
		t.Fatalf("credential restore args = %s, want %s", got, want)
	}
}

func TestURLWithCredentialsRequiresHTTPS(t *testing.T) {
	if _, err := urlWithCredentials("http://github.com/acme/repo.git", "alice", "secret"); err == nil {
		t.Fatal("expected http credential URL to be rejected")
	}
	if _, err := urlWithCredentials("git@github.com:acme/repo.git", "alice", "secret"); err == nil {
		t.Fatal("expected scp-style credential URL to be rejected")
	}
}

func TestGitDangerouslyAuthenticateRequiresHTTPS(t *testing.T) {
	git := newGitForTest(t, &gitProcessHandler{})

	if _, err := git.DangerouslyAuthenticate(context.Background(), "alice", "secret", "github.com", "http"); err == nil {
		t.Fatal("expected non-https protocol to be rejected")
	}
}

func gitBoolPtr(v bool) *bool { return &v }
