package instance

import (
	"net/http"
	"strings"
	"testing"
)

// TestConnect_RequiresSandboxID covers the early-return validation in Connect.
func TestConnect_RequiresSandboxID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Connect(ConnectInfo{})
		})
	})
	if !strings.Contains(stderr, "sandbox ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestConnect_ConnectFails verifies that a non-2xx response from the
// control-plane connect call surfaces as PrintError. We don't need to mock
// envd because the function bails out before reaching it.
func TestConnect_ConnectFails(t *testing.T) {
	srv := withMock(t)
	srv.Handle("POST", "/sandboxes/{id}/connect", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Connect(ConnectInfo{SandboxID: "sbx-test"})
		})
	})
	if !strings.Contains(stderr, "connect to sandbox sbx-test failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestExec_RequiresSandboxID covers the first validation branch in Exec.
func TestExec_RequiresSandboxID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Exec(ExecInfo{Command: []string{"echo", "hi"}})
		})
	})
	if !strings.Contains(stderr, "sandbox ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestExec_RequiresCommand covers the second validation branch in Exec when
// SandboxID is supplied but the command list is empty.
func TestExec_RequiresCommand(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Exec(ExecInfo{SandboxID: "sbx-test"})
		})
	})
	if !strings.Contains(stderr, "command is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestExec_ConnectFails ensures the connect-failure path is surfaced before
// any envd interaction is attempted.
func TestExec_ConnectFails(t *testing.T) {
	srv := withMock(t)
	srv.Handle("POST", "/sandboxes/{id}/connect", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Exec(ExecInfo{
				SandboxID: "sbx-test",
				Command:   []string{"echo", "hi"},
			})
		})
	})
	if !strings.Contains(stderr, "connect to sandbox sbx-test failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestLogs_RequiresSandboxID covers the early-return validation in Logs.
func TestLogs_RequiresSandboxID(t *testing.T) {
	withMock(t)
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Logs(LogsInfo{})
		})
	})
	if !strings.Contains(stderr, "sandbox ID is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestLogs_PrettyHappyPath covers the non-follow pretty-print path. The
// default mock /logs response includes one log line + one entry; both should
// flow through printLogEntries.
func TestLogs_PrettyHappyPath(t *testing.T) {
	srv := withMock(t)
	out := captureStdout(t, func() {
		Logs(LogsInfo{SandboxID: "sbx-test"})
	})
	if !strings.Contains(out, "hello") {
		t.Fatalf("stdout missing log line: %q", out)
	}
	if !sawRequest(srv, "GET", "/sandboxes/sbx-test/logs") {
		t.Fatalf("logs route not hit; got %+v", srv.Requests())
	}
}

// TestLogs_JSONHappyPath covers the JSON branch — the call should print the
// raw struct via PrintJSON without a "No logs found" tail.
func TestLogs_JSONHappyPath(t *testing.T) {
	withMock(t)
	out := captureStdout(t, func() {
		Logs(LogsInfo{SandboxID: "sbx-test", Format: "json"})
	})
	if !strings.Contains(out, "Logs") && !strings.Contains(out, "logs") {
		t.Fatalf("stdout = %q", out)
	}
}

// TestLogs_PrettyEmptyShowsNotice exercises the no-results branch when both
// logs and logEntries arrays come back empty.
func TestLogs_PrettyEmptyShowsNotice(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/sandboxes/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"logs":[],"logEntries":[]}`))
	})
	out := captureStdout(t, func() {
		Logs(LogsInfo{SandboxID: "sbx-test"})
	})
	if !strings.Contains(out, "No logs found") {
		t.Fatalf("stdout = %q", out)
	}
}

// TestLogs_GetLogsFails surfaces the API error path.
func TestLogs_GetLogsFails(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/sandboxes/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			Logs(LogsInfo{SandboxID: "sbx-test"})
		})
	})
	if !strings.Contains(stderr, "get sandbox logs failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// TestLogs_LevelFilterDropsBelow covers the level-filter branch in
// printLogEntries: an entry below the configured level should not appear.
func TestLogs_LevelFilterDropsBelow(t *testing.T) {
	srv := withMock(t)
	srv.Handle("GET", "/sandboxes/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"logs":[],
			"logEntries":[
				{"level":"debug","message":"verbose-debug","fields":{},"timestamp":"2025-01-01T00:00:00Z"},
				{"level":"error","message":"loud-error","fields":{"logger":"app"},"timestamp":"2025-01-01T00:00:00Z"}
			]
		}`))
	})
	out := captureStdout(t, func() {
		Logs(LogsInfo{SandboxID: "sbx-test", Level: "ERROR"})
	})
	if strings.Contains(out, "verbose-debug") {
		t.Fatalf("debug entry leaked through ERROR filter: %q", out)
	}
	if !strings.Contains(out, "loud-error") {
		t.Fatalf("error entry missing: %q", out)
	}
}
