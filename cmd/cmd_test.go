package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/config"
	logpkg "github.com/aonesuite/aone/internal/log"
)

// captureStdout redirects os.Stdout while fn runs and returns the captured bytes.
// Used because PrintSuccess and fmt.Print* write to stdout from cmd handlers.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String()
}

// captureStderr mirrors captureStdout for stderr (PrintError, PrintWarn).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	os.Stderr = orig
	return buf.String()
}

// isolateConfig points the user-config home and credential env vars at a
// fresh temp dir so cmd tests can mutate the config file without touching
// the real ~/.config/aone.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigHome, dir)
	t.Setenv(config.EnvAPIKey, "")
	t.Setenv(config.EnvEndpoint, "")
	return dir
}

// TestRoot_HelpListsAllSubcommandGroups exercises rootCmd by issuing --help
// and verifying both the core and sandbox groups + their members are
// printed. This catches accidental removal of subcommand wiring in init().
func TestRoot_HelpListsAllSubcommandGroups(t *testing.T) {
	out := bytes.Buffer{}
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute --help: %v", err)
	}
	got := out.String()
	for _, want := range []string{"auth", "sandbox", "Account & configuration", "Sandbox management"} {
		if !strings.Contains(got, want) {
			t.Errorf("--help output missing %q; got:\n%s", want, got)
		}
	}
}

// TestRoot_VersionPrintsValue confirms the --version flag dispatches via
// cobra's built-in handling and resolveVersion is wired in.
func TestRoot_VersionPrintsValue(t *testing.T) {
	out := bytes.Buffer{}
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute --version: %v", err)
	}
	if !strings.Contains(out.String(), "aone") {
		t.Fatalf("version output missing program name: %q", out.String())
	}
}

// TestRoot_DebugFlagSetsEnv verifies the --debug PersistentPreRun side
// effect: AONE_DEBUG=1 must end up in the environment so SDK clients
// constructed inside subcommands pick it up.
func TestRoot_DebugFlagSetsEnv(t *testing.T) {
	t.Setenv(config.EnvDebug, "")
	prev := debugFlag
	t.Cleanup(func() { debugFlag = prev })

	rootCmd.SetArgs([]string{"--debug", "auth", "info"})
	// auth info touches Resolver/Path/Load; isolate config so it can't fail
	// for unrelated reasons.
	isolateConfig(t)
	out := bytes.Buffer{}
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	_ = rootCmd.Execute() // we don't care about success, only the side effect
	if os.Getenv(config.EnvDebug) != "1" {
		t.Fatalf("AONE_DEBUG not set after --debug: %q", os.Getenv(config.EnvDebug))
	}
}

// TestRoot_VerbosityFlagSetsLogLevel exercises the new -v / -vv flags and
// confirms PersistentPreRun resolves them into the global logger level.
// We drive rootCmd end-to-end (rather than calling ResolveLevel directly)
// so any future regression in flag wiring is caught.
func TestRoot_VerbosityFlagSetsLogLevel(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want slog.Level
	}{
		{"single -v enables debug", []string{"-v", "auth", "info"}, slog.LevelDebug},
		{"double -vv enables trace", []string{"-vv", "auth", "info"}, logpkg.LevelTrace},
		{"--debug enables debug", []string{"--debug", "auth", "info"}, slog.LevelDebug},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			t.Setenv("AONE_DEBUG", "")
			t.Setenv("AONE_LOG_LEVEL", "")
			prevD, prevV := debugFlag, verbosityFlag
			t.Cleanup(func() {
				debugFlag = prevD
				verbosityFlag = prevV
			})

			rootCmd.SetArgs(tc.args)
			out := bytes.Buffer{}
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			_ = rootCmd.Execute()

			got := slog.Level(logpkg.CurrentLevel())
			if got != tc.want {
				t.Fatalf("level = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRoot_StdoutNotPollutedByDebugLogs verifies the "logs go to stderr"
// contract — pipelines like `aone ... | jq` keep working under --debug.
func TestRoot_StdoutNotPollutedByDebugLogs(t *testing.T) {
	isolateConfig(t)
	prev := debugFlag
	t.Cleanup(func() { debugFlag = prev })

	rootCmd.SetArgs([]string{"--debug", "auth", "info"})
	stdout := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			_ = rootCmd.Execute()
		})
	})
	// "level=DEBUG" is slog's text-handler signature; finding it in stdout
	// would mean we accidentally wired logging to the wrong stream.
	if strings.Contains(stdout, "level=DEBUG") {
		t.Fatalf("debug logs leaked to stdout: %q", stdout)
	}
}

// TestSandboxList_FlagsBindToInfo runs `aone sandbox list -f json -l 5 -s
// running` against a clean config (no API key). The command should fail at
// the resolver step, but cobra's flag parsing must succeed without error
// and pass values through to instance.List.
func TestSandboxList_FlagsBindToInfo(t *testing.T) {
	isolateConfig(t)
	rootCmd.SetArgs([]string{"sandbox", "list", "-f", "json", "-l", "5", "-s", "running"})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			_ = rootCmd.Execute()
		})
	})
	// With no API key the inner List call fails early — cobra parsing was
	// fine (no "unknown flag" message) and we got the resolver error.
	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("flags did not bind correctly: %q", stderr)
	}
}

// TestTemplateInit_ShortNameFlagBinds verifies `aone sandbox template init -n`
// is accepted as the short form of --name.
func TestTemplateInit_ShortNameFlagBinds(t *testing.T) {
	cmd := newTemplateInitCmd()
	if err := cmd.ParseFlags([]string{"-n", "demo", "-l", "go"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if got := cmd.Flags().Lookup("name"); got == nil || got.Value.String() != "demo" {
		if got == nil {
			t.Fatalf("name flag not registered")
		}
		t.Fatalf("name flag = %q, want demo", got.Value.String())
	}
}

func TestTemplateMigrate_ConfigFlagBinds(t *testing.T) {
	cmd := newTemplateMigrateCmd()
	if err := cmd.ParseFlags([]string{"--config", "/tmp/aone.sandbox.toml", "-l", "go"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if got := cmd.Flags().Lookup("config"); got == nil || got.Value.String() != "/tmp/aone.sandbox.toml" {
		if got == nil {
			t.Fatalf("config flag not registered")
		}
		t.Fatalf("config flag = %q, want /tmp/aone.sandbox.toml", got.Value.String())
	}
}

// TestAuthLogin_PersistsKey covers the non-prompt happy path: --api-key and
// --no-verify together skip both the interactive prompt and the network
// verification, leaving a saved key on disk.
func TestAuthLogin_PersistsKey(t *testing.T) {
	isolateConfig(t)
	rootCmd.SetArgs([]string{"auth", "login", "--api-key", "k-test", "--no-verify"})
	stdout := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("login: %v", err)
			}
		})
	})
	_ = stdout

	f, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.APIKey != "k-test" {
		t.Fatalf("saved api-key = %q, want k-test", f.APIKey)
	}
	if f.LastLoginAt == nil {
		t.Errorf("LastLoginAt not populated")
	}
}

// TestAuthLogout_ClearsKey writes a key, then runs logout, then asserts the
// key is gone. Touches the empty-key fast path implicitly: a follow-up
// logout should print the no-op warning instead of failing.
func TestAuthLogout_ClearsKey(t *testing.T) {
	isolateConfig(t)
	if err := config.Save(&config.File{APIKey: "old"}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	rootCmd.SetArgs([]string{"auth", "logout"})
	_ = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("logout: %v", err)
			}
		})
	})

	f, _ := config.Load()
	if f.APIKey != "" {
		t.Fatalf("APIKey still set after logout: %q", f.APIKey)
	}

	// Second logout: empty-key warn branch. We just need it not to error.
	rootCmd.SetArgs([]string{"auth", "logout"})
	_ = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("second logout: %v", err)
			}
		})
	})
}

// TestAuthInfo_ReportsNoKey runs `auth info` without any saved key and
// confirms it tells the user the key is unset rather than crashing.
func TestAuthInfo_ReportsNoKey(t *testing.T) {
	isolateConfig(t)
	rootCmd.SetArgs([]string{"auth", "info"})
	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("info: %v", err)
			}
		})
	})
	if !strings.Contains(out, "<not set>") {
		t.Fatalf("expected '<not set>' marker in info output: %q", out)
	}
	if !strings.Contains(out, "Endpoint:") {
		t.Fatalf("info output missing Endpoint line: %q", out)
	}
}

// TestAuthConfigure_NonInteractiveFlags drives configure with --api-key and
// --endpoint, exercising the early-return path that bypasses the prompts.
func TestAuthConfigure_NonInteractiveFlags(t *testing.T) {
	isolateConfig(t)
	rootCmd.SetArgs([]string{"auth", "configure", "--api-key", "kk", "--endpoint", "https://staging.example/"})
	_ = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("configure: %v", err)
			}
		})
	})
	f, _ := config.Load()
	if f.APIKey != "kk" {
		t.Fatalf("APIKey = %q", f.APIKey)
	}
	// Trailing slash trimmed.
	if f.Endpoint != "https://staging.example" {
		t.Fatalf("Endpoint = %q", f.Endpoint)
	}
}

// TestCoalesce covers the small helper that picks the first non-empty
// value. Stable behavior matters here because configure's prompts use it
// to display defaults.
func TestCoalesce(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"", "", "a"}, "a"},
		{[]string{"first", "second"}, "first"},
		{[]string{"", ""}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := coalesce(tc.in...); got != tc.want {
			t.Errorf("coalesce(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSandboxConnectArgs_RequiresOneArg covers cobra's ExactArgs(1)
// validation for `sandbox connect` — running without an ID must fail
// before reaching the handler.
func TestSandboxConnectArgs_RequiresOneArg(t *testing.T) {
	isolateConfig(t)
	rootCmd.SetArgs([]string{"sandbox", "connect"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	t.Cleanup(func() {
		rootCmd.SilenceUsage = false
		rootCmd.SilenceErrors = false
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected ExactArgs validation error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("unexpected error: %v", err)
	}
}
