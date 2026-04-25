package sandbox

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReadyCmdStrings(t *testing.T) {
	cases := []struct {
		name    string
		got     string
		contain []string
	}{
		{"WaitForPort", WaitForPort(8080).String(), []string{"ss -tuln", ":8080"}},
		{"WaitForURL default 200", WaitForURL("http://localhost:3000/", 0).String(), []string{"curl", "http://localhost:3000/", `"200"`}},
		{"WaitForURL custom status", WaitForURL("http://x/", 204).String(), []string{"curl", `"204"`}},
		{"WaitForProcess", WaitForProcess("nginx").String(), []string{"pgrep", "'nginx'", "/dev/null"}},
		{"WaitForFile", WaitForFile("/tmp/ready").String(), []string{"[ -f '/tmp/ready' ]"}},
		{"WaitForTimeout", WaitForTimeout(5).String(), []string{"sleep 5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, frag := range tc.contain {
				if !strings.Contains(tc.got, frag) {
					t.Errorf("%q missing %q", tc.got, frag)
				}
			}
		})
	}
}

func TestTemplateBuilderChaining(t *testing.T) {
	tb := NewTemplate().
		FromUbuntuImage("22.04").
		SetWorkdir("/app").
		RunCmd("echo hi").
		SetEnvs(map[string]string{"FOO": "bar"}).
		AddMcpServer("server-a", "", "server-b")

	if tb.fromImage == nil || *tb.fromImage != "ubuntu:22.04" {
		t.Errorf("fromImage = %v, want ubuntu:22.04", tb.fromImage)
	}

	// Steps: WORKDIR, RUN, ENV, MCP_SERVER (2, empty skipped)
	types := make([]string, 0, len(tb.steps))
	for _, s := range tb.steps {
		types = append(types, s.Type)
	}
	wantCount := map[string]int{"WORKDIR": 1, "RUN": 1, "ENV": 1, "MCP_SERVER": 2}
	got := map[string]int{}
	for _, typ := range types {
		got[typ]++
	}
	for k, v := range wantCount {
		if got[k] != v {
			t.Errorf("step %q count = %d, want %d (got=%v)", k, got[k], v, types)
		}
	}
}

func TestTemplateBuilderForceBuildAndNextLayer(t *testing.T) {
	tb := NewTemplate().FromBaseImage()
	tb.RunCmd("a")
	if *tb.steps[0].Force {
		t.Error("step 0 should not be forced by default")
	}

	tb.ForceBuild()
	tb.RunCmd("b")
	if !*tb.steps[1].Force {
		t.Error("step 1 should be forced after ForceBuild()")
	}

	// ForceNextLayer should force only the inner step and restore state.
	prev := tb.force
	tb.force = false
	tb.ForceNextLayer(func(b *TemplateBuilder) {
		b.RunCmd("c")
	})
	if !*tb.steps[2].Force {
		t.Error("inner step should be forced")
	}
	if tb.force != false {
		t.Error("force state should restore after ForceNextLayer")
	}
	tb.RunCmd("d")
	if *tb.steps[3].Force {
		t.Error("step after ForceNextLayer should not be forced")
	}
	tb.force = prev
}

func TestTemplateBuilderFromImageExclusive(t *testing.T) {
	tb := NewTemplate().FromTemplate("parent")
	if tb.fromImage != nil || tb.fromTemplate == nil {
		t.Fatal("FromTemplate should clear fromImage")
	}
	tb.FromImage("alpine")
	if tb.fromTemplate != nil || tb.fromImage == nil || *tb.fromImage != "alpine" {
		t.Fatal("FromImage should clear fromTemplate")
	}
	tb.FromRegistry("private/x:1", "u", "p")
	if tb.fromImageRegistry == nil {
		t.Fatal("FromRegistry should set fromImageRegistry")
	}
	var payload map[string]string
	if err := json.Unmarshal(*tb.fromImageRegistry, &payload); err != nil {
		t.Fatalf("registry payload not JSON: %v", err)
	}
	if payload["type"] != "registry" || payload["username"] != "u" || payload["password"] != "p" {
		t.Errorf("registry payload = %v", payload)
	}
}

func TestTemplateBuilderFromAWSAndGCPRegistry(t *testing.T) {
	tb := NewTemplate().FromAWSRegistry("ecr/x:1", "AKID", "SECRET", "us-east-1")
	if tb.fromImageRegistry == nil {
		t.Fatal("AWS registry payload missing")
	}
	var aws map[string]string
	_ = json.Unmarshal(*tb.fromImageRegistry, &aws)
	if aws["type"] != "aws" || aws["awsRegion"] != "us-east-1" {
		t.Errorf("aws payload = %v", aws)
	}

	tb.FromGCPRegistry("gcr/x:1", `{"foo":"bar"}`)
	var gcp map[string]string
	_ = json.Unmarshal(*tb.fromImageRegistry, &gcp)
	if gcp["type"] != "gcp" {
		t.Errorf("gcp payload = %v", gcp)
	}
}

func TestTemplateBuilderToDockerfile(t *testing.T) {
	df, err := NewTemplate().
		FromUbuntuImage("latest").
		SetWorkdir("/app").
		RunCmd("echo hi").
		SetStartCmd("./run", WaitForTimeout(5)).
		ToDockerfile()
	if err != nil {
		t.Fatalf("ToDockerfile: %v", err)
	}
	for _, frag := range []string{"FROM ubuntu:latest", "WORKDIR /app", "RUN echo hi", "CMD ./run"} {
		if !strings.Contains(df, frag) {
			t.Errorf("Dockerfile missing %q:\n%s", frag, df)
		}
	}
}

func TestTemplateBuilderToDockerfileFromTemplateFails(t *testing.T) {
	_, err := NewTemplate().FromTemplate("parent").ToDockerfile()
	if err == nil {
		t.Fatal("expected error when converting template-based builder to Dockerfile")
	}
}

func TestSetContextPath(t *testing.T) {
	tb := NewTemplate()
	dir := t.TempDir()
	// create a .dockerignore
	if err := writeFile(dir+"/.dockerignore", "node_modules\n*.log\n"); err != nil {
		t.Fatalf("write ignore: %v", err)
	}
	tb.SetContextPath(dir)
	if tb.ContextPath() != dir {
		t.Errorf("ContextPath() = %q, want %q", tb.ContextPath(), dir)
	}
	pats := tb.IgnorePatterns()
	if len(pats) != 2 {
		t.Fatalf("IgnorePatterns len = %d, want 2 (%v)", len(pats), pats)
	}

	// Clearing should empty patterns.
	tb.SetContextPath("")
	if tb.ContextPath() != "" || tb.IgnorePatterns() != nil {
		t.Error("clearing SetContextPath should reset patterns")
	}
}

func TestLogsFromCursor(t *testing.T) {
	if logsFromCursor(nil) != nil {
		t.Error("nil cursor should produce nil params")
	}
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	p := logsFromCursor(&ts)
	if p == nil || p.Cursor == nil {
		t.Fatal("expected cursor params")
	}
	if *p.Cursor != ts.UnixMilli()+1 {
		t.Errorf("cursor = %d, want %d", *p.Cursor, ts.UnixMilli()+1)
	}
}

func TestFilterNewLogs(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := []BuildLogEntry{
		{Message: "a", Timestamp: t0},
		{Message: "b", Timestamp: t0.Add(time.Second)},
		{Message: "c", Timestamp: t0.Add(2 * time.Second)},
	}

	if got := filterNewLogs(entries, nil); len(got) != 3 {
		t.Errorf("nil cursor should return all entries, got %d", len(got))
	}

	cursor := t0.Add(time.Second)
	got := filterNewLogs(entries, &cursor)
	if len(got) != 1 || got[0].Message != "c" {
		t.Errorf("got %v, want [c]", got)
	}
}

// writeFile is a tiny test helper.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
