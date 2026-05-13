//go:build integration

package sandbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aonesuite/aone/packages/go/internal/sdkconfig"
)

func TestIntegrationGitLocalRemoteAndCredentialCleanup(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv(sdkconfig.EnvAPIKey))
	if apiKey == "" {
		t.Skipf("%s is required for integration tests", sdkconfig.EnvAPIKey)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := NewClient(&Config{APIKey: apiKey, Endpoint: os.Getenv(sdkconfig.EnvEndpoint)})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	timeout := int32(300)
	sb, _, err := client.CreateAndWait(ctx, CreateParams{Timeout: &timeout}, WithPollInterval(2*time.Second))
	if err != nil {
		t.Fatalf("CreateAndWait: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = sb.Kill(cleanupCtx)
	})

	git := sb.Git()
	const repo = "/tmp/aone-git-work"
	const bare = "/tmp/aone-git-remote.git"
	const clone = "/tmp/aone-git-clone"

	if _, err := sb.Commands().Run(ctx, "rm -rf "+repo+" "+bare+" "+clone); err != nil {
		t.Fatalf("cleanup paths: %v", err)
	}
	if _, err := git.Init(ctx, repo, &InitOptions{InitialBranch: "main"}); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	unbornBranches, err := git.Branches(ctx, repo)
	if err != nil {
		t.Fatalf("branches in unborn repo: %v", err)
	}
	if unbornBranches.CurrentBranch == "" {
		t.Fatalf("unborn branches = %+v, want current branch fallback and no branch list", unbornBranches)
	}
	if len(unbornBranches.Branches) != 0 {
		t.Fatalf("unborn branch list = %v, want empty list", unbornBranches.Branches)
	}
	if _, err := git.Init(ctx, bare, &InitOptions{Bare: true, InitialBranch: "main"}); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}
	if _, err := git.ConfigureUser(ctx, "Aone Integration", "integration@example.com", &ConfigOptions{
		Scope:    GitConfigScopeLocal,
		RepoPath: repo,
	}); err != nil {
		t.Fatalf("configure user: %v", err)
	}
	if _, err := sb.Commands().Run(ctx, "printf initial > "+repo+"/README.md"); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	if _, err := git.Add(ctx, repo, nil); err != nil {
		t.Fatalf("add default: %v", err)
	}
	if _, err := git.Reset(ctx, repo, &ResetOptions{Paths: []string{"README.md"}}); err != nil {
		t.Fatalf("reset path after default add: %v", err)
	}
	if _, err := git.Add(ctx, repo, &AddOptions{All: boolPtr(false)}); err != nil {
		t.Fatalf("add explicit false: %v", err)
	}
	if _, err := git.Reset(ctx, repo, &ResetOptions{Paths: []string{"README.md"}}); err != nil {
		t.Fatalf("reset path after explicit false add: %v", err)
	}
	if _, err := git.Add(ctx, repo, &AddOptions{All: boolPtr(true)}); err != nil {
		t.Fatalf("add initial file: %v", err)
	}
	if _, err := git.Commit(ctx, repo, "test: initial commit", nil); err != nil {
		t.Fatalf("commit initial file: %v", err)
	}
	if _, err := git.RemoteAdd(ctx, repo, "origin", bare, nil); err != nil {
		t.Fatalf("remote add: %v", err)
	}
	if _, err := git.Push(ctx, repo, nil); err != nil {
		t.Fatalf("push with auto-selected remote: %v", err)
	}

	status, err := git.Status(ctx, repo)
	if err != nil {
		t.Fatalf("status after push: %v", err)
	}
	if status.Upstream != "origin/master" && status.Upstream != "origin/main" {
		t.Fatalf("upstream = %q, want origin/master or origin/main; status=%+v", status.Upstream, status)
	}

	branches, err := git.Branches(ctx, repo)
	if err != nil {
		t.Fatalf("branches: %v", err)
	}
	if branches.CurrentBranch == "" || len(branches.Branches) == 0 {
		t.Fatalf("unexpected branches: %+v", branches)
	}

	if _, err := git.Clone(ctx, bare, &GitCloneOptions{Path: clone}); err != nil {
		t.Fatalf("clone local bare repo: %v", err)
	}
	if _, err := sb.Commands().Run(ctx, "printf change >> "+repo+"/README.md"); err != nil {
		t.Fatalf("write change: %v", err)
	}
	if _, err := git.Add(ctx, repo, &AddOptions{All: boolPtr(true)}); err != nil {
		t.Fatalf("add change: %v", err)
	}
	if _, err := git.Commit(ctx, repo, "test: update readme", nil); err != nil {
		t.Fatalf("commit change: %v", err)
	}
	if _, err := git.Push(ctx, repo, &PushOptions{Remote: "origin", Branch: branches.CurrentBranch}); err != nil {
		t.Fatalf("push explicit remote: %v", err)
	}
	if _, err := git.Pull(ctx, clone, &PullOptions{Remote: "origin", Branch: branches.CurrentBranch}); err != nil {
		t.Fatalf("pull explicit remote: %v", err)
	}

	if _, err := sb.Commands().Run(ctx, "printf scratch >> "+repo+"/README.md"); err != nil {
		t.Fatalf("write scratch change: %v", err)
	}
	if _, err := git.Add(ctx, repo, &AddOptions{Files: []string{"README.md"}}); err != nil {
		t.Fatalf("add scratch change: %v", err)
	}
	if _, err := git.Restore(ctx, repo, &RestoreOptions{
		Paths:  []string{"README.md"},
		Staged: boolPtr(true),
	}); err != nil {
		t.Fatalf("restore staged scratch change: %v", err)
	}
	if _, err := git.Restore(ctx, repo, &RestoreOptions{
		Paths:    []string{"README.md"},
		Worktree: boolPtr(true),
	}); err != nil {
		t.Fatalf("restore worktree scratch change: %v", err)
	}

	if _, err := git.RemoteAdd(ctx, repo, "https-origin", "https://example.com/acme/repo.git", nil); err != nil {
		t.Fatalf("remote add https-origin: %v", err)
	}
	if _, err := git.Push(ctx, repo, &PushOptions{Branch: branches.CurrentBranch}); err == nil {
		t.Fatal("push branch-only with multiple remotes unexpectedly succeeded")
	}
	if _, err := git.Pull(ctx, repo, &PullOptions{Branch: branches.CurrentBranch}); err == nil {
		t.Fatal("pull branch-only with multiple remotes unexpectedly succeeded")
	}

	_, err = git.Push(ctx, repo, &PushOptions{
		Remote:      "https-origin",
		Branch:      branches.CurrentBranch,
		SetUpstream: boolPtr(false),
		Username:    "alice",
		Password:    "bad-token",
	})
	if err == nil {
		t.Fatal("push with bad credentials unexpectedly succeeded")
	}
	remoteURL, err := git.RemoteGet(ctx, repo, "https-origin")
	if err != nil {
		t.Fatalf("remote get after failed credential push: %v", err)
	}
	if strings.Contains(remoteURL, "alice") || strings.Contains(remoteURL, "bad-token") {
		t.Fatalf("remote URL retained credentials: %q", remoteURL)
	}

	if _, err := sb.Commands().Run(ctx, "git -C "+repo+" checkout --detach HEAD"); err != nil {
		t.Fatalf("detach HEAD: %v", err)
	}
	if _, err := git.Push(ctx, repo, &PushOptions{
		Remote: "origin",
		Branch: branches.CurrentBranch,
	}); err != nil {
		t.Fatalf("push explicit branch from detached HEAD: %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
