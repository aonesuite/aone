package sandbox

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestParseBoolEnv(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"on", true},
		{" true ", true},
		{"", false},
		{"0", false},
		{"false", false},
		{"no", false},
		{"random", false},
	}
	for _, tc := range cases {
		if got := parseBoolEnv(tc.in); got != tc.want {
			t.Errorf("parseBoolEnv(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewClientEnvFallback(t *testing.T) {
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvEndpoint, "https://env.example.com/")
	t.Setenv(EnvDebug, "true")

	c, err := NewClient(nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.config.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", c.config.APIKey, "env-key")
	}
	if c.config.Endpoint != "https://env.example.com" {
		t.Errorf("Endpoint = %q, want trimmed trailing slash", c.config.Endpoint)
	}
	if !c.config.Debug {
		t.Error("Debug should be true from env")
	}
	if c.config.HTTPClient == nil {
		t.Error("HTTPClient should default")
	}
}

func TestNewClientDefaultEndpoint(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvEndpoint, "")
	t.Setenv(EnvDebug, "")

	c, err := NewClient(&Config{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.config.Endpoint != DefaultEndpoint {
		t.Errorf("Endpoint = %q, want %q", c.config.Endpoint, DefaultEndpoint)
	}
}

func TestNewClientExplicitConfigWins(t *testing.T) {
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvEndpoint, "https://env.example.com")

	c, err := NewClient(&Config{APIKey: "explicit-key", Endpoint: "https://explicit.example.com"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.config.APIKey != "explicit-key" {
		t.Errorf("APIKey = %q, want explicit value", c.config.APIKey)
	}
	if c.config.Endpoint != "https://explicit.example.com" {
		t.Errorf("Endpoint = %q, want explicit value", c.config.Endpoint)
	}
}

func TestAPIKeyEditorSetsHeader(t *testing.T) {
	edit := apiKeyEditor("secret")
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if err := edit(context.Background(), req); err != nil {
		t.Fatalf("editor error: %v", err)
	}
	if got := req.Header.Get("X-API-Key"); got != "secret" {
		t.Errorf("X-API-Key = %q, want %q", got, "secret")
	}
}

func TestAPIKeyEditorPreservesAuthorization(t *testing.T) {
	edit := apiKeyEditor("secret")
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	req.Header.Set("Authorization", "Bearer abc")
	if err := edit(context.Background(), req); err != nil {
		t.Fatalf("editor error: %v", err)
	}
	if req.Header.Get("X-API-Key") != "" {
		t.Error("X-API-Key should not override existing Authorization")
	}
}

func TestRequestTimeoutEditorRespectsExistingDeadline(t *testing.T) {
	edit := requestTimeoutEditor(10 * time.Millisecond)
	deadline := time.Now().Add(1 * time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://example.com", nil)
	if err := edit(ctx, req); err != nil {
		t.Fatalf("editor error: %v", err)
	}
	gotDeadline, ok := req.Context().Deadline()
	if !ok {
		t.Fatal("expected original deadline to remain")
	}
	if !gotDeadline.Equal(deadline) {
		t.Errorf("deadline = %v, want %v (editor must not override)", gotDeadline, deadline)
	}
}

// TestRequestTimeoutEditorAttachesTimeout checks the editor attaches a
// derived deadline when the caller did not provide one.
func TestRequestTimeoutEditorAttachesTimeout(t *testing.T) {
	edit := requestTimeoutEditor(50 * time.Millisecond)
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if err := edit(context.Background(), req); err != nil {
		t.Fatalf("editor error: %v", err)
	}
	if _, ok := req.Context().Deadline(); !ok {
		t.Fatal("expected derived deadline to be attached")
	}
}
