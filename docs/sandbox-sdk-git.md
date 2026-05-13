# Sandbox SDK Git Notes

This document records the current aone Go SDK Git helper behavior.

## Current Behavior

The sandbox Git helper wraps common `git` commands through the sandbox command service. Every Git command runs with `GIT_TERMINAL_PROMPT=0`, while preserving caller-provided command options such as environment variables, user, working directory, callbacks, and timeout.

Non-zero Git exit codes are returned as errors with the command result attached internally, so callers do not need to inspect `CommandResult.ExitCode` before deciding whether the operation failed. Git authentication failures wrap `ErrGitAuth`; missing upstream configuration wraps `ErrGitUpstream`.

`Status` returns a structured `GitStatus` with branch, upstream, ahead/behind, detached-head, and file status details. `Branches` returns a structured `GitBranches` value with local branch names and the current branch.

## Credentials

Username and password credentials are accepted only for HTTPS Git URLs.

`Clone` injects credentials into the clone URL when requested. Unless `DangerouslyStoreCredentials` is true, the helper resets `origin` to the credential-free URL after clone. If that cleanup fails, the method returns an error even though clone itself succeeded.

`Push` and `Pull` support temporary username/password credentials through `PushOptions` and `PullOptions`. The helper temporarily rewrites the selected remote URL, runs the Git operation, and restores the original remote URL with an independent cleanup timeout.

`DangerouslyAuthenticate` stores credentials in the sandbox Git credential helper. It only accepts the `https` protocol.

## Push And Pull

`Push` accepts `PushOptions`. When no remote is specified and the repository has exactly one remote, that remote is selected automatically. `SetUpstream` defaults to true; set it explicitly to false to omit `--set-upstream`.

`Pull` accepts `PullOptions`. When no remote is specified and the repository has exactly one remote, that remote is selected automatically.

If a branch is specified but no remote can be selected, the helper returns an argument error instead of letting Git interpret the branch name as a repository.

For an initialized repository with no commits, `Branches` reports the unborn branch as `CurrentBranch` but leaves `Branches` empty because Git has not created an enumerable local branch yet.

## Local Repository Operations

Repository mutation helpers accept option structs instead of positional boolean arguments:

- `InitOptions` controls bare repository initialization and initial branch selection.
- `RemoteAddOptions` controls overwrite behavior and optional fetch after adding the remote.
- `AddOptions` controls pathspecs and whether `git add -A` is used. When `All` is omitted, it defaults to true.
- `CommitOptions` controls author override and `--allow-empty`.
- `DeleteBranchOptions` controls force deletion and rejects empty branch names before running Git.
- `ResetOptions` controls mode, target, and pathspecs. Reset modes are limited to the supported Git modes, and mode-based resets cannot be combined with pathspecs.
- `RestoreOptions` controls `--staged`, `--worktree`, source, and pathspecs. If both `Staged` and `Worktree` are set, at least one must be true.
- `ConfigOptions` controls config scope and repository path. Supported scopes are `GitConfigScopeLocal`, `GitConfigScopeGlobal`, `GitConfigScopeSystem`, and `GitConfigScopeWorktree`; unknown scopes are rejected before running Git. Local scope requires `RepoPath`.

`ConfigureUser` accepts `ConfigOptions` and writes both `user.name` and `user.email` in the selected config scope. Name and email are required.

Pointer booleans are used where the SDK needs to distinguish an omitted option from an explicit false value.

## Integration Coverage

`git_integration_test.go` is guarded by the `integration` build tag. Run it with:

```bash
go test -tags integration ./...
```

The test requires `AONE_API_KEY`. It creates a temporary sandbox, initializes a local bare Git repository inside the sandbox, verifies push/pull behavior, checks unborn and committed branch reporting, exercises add/reset/restore option combinations, and verifies remote URL cleanup after temporary credential injection fails.
