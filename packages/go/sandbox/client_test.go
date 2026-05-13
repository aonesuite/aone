package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/aonesuite/aone/packages/go/internal/sdkconfig"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
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
	t.Setenv(sdkconfig.EnvAPIKey, "env-key")
	t.Setenv(sdkconfig.EnvEndpoint, "https://env.example.com/")
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

func TestNewClientDoesNotMutateInputConfig(t *testing.T) {
	t.Setenv(sdkconfig.EnvAPIKey, "env-key")
	t.Setenv(sdkconfig.EnvEndpoint, "https://env.example.com/")
	t.Setenv(EnvDebug, "true")

	cfg := &Config{}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if cfg.APIKey != "" || cfg.Endpoint != "" || cfg.HTTPClient != nil || cfg.Debug {
		t.Fatalf("input config was mutated: %+v", cfg)
	}
	if got, want := c.config.APIKey, "env-key"; got != want {
		t.Errorf("client APIKey = %q, want %q", got, want)
	}
	if got, want := c.config.Endpoint, "https://env.example.com"; got != want {
		t.Errorf("client Endpoint = %q, want %q", got, want)
	}
	if !c.config.Debug {
		t.Error("client Debug should be true from env")
	}
	if c.config.HTTPClient == nil {
		t.Error("client HTTPClient should default")
	}
}

func TestNewClientDefaultEndpoint(t *testing.T) {
	t.Setenv(sdkconfig.EnvAPIKey, "")
	t.Setenv(sdkconfig.EnvEndpoint, "")
	t.Setenv(EnvDebug, "")

	c, err := NewClient(&Config{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.config.Endpoint != sdkconfig.DefaultEndpoint {
		t.Errorf("Endpoint = %q, want %q", c.config.Endpoint, sdkconfig.DefaultEndpoint)
	}
}

func TestNewClientExplicitConfigWins(t *testing.T) {
	t.Setenv(sdkconfig.EnvAPIKey, "env-key")
	t.Setenv(sdkconfig.EnvEndpoint, "https://env.example.com")

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
	if got := req.Header.Get("Authorization"); got != "Bearer secret" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer secret")
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
	if got := req.Header.Get("Authorization"); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want caller value preserved", got)
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

func TestCreateRefreshesEnvdTokenFromDetail(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sbx/sandboxes":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sandbox_id":   "sbx-test",
				"template_id":  "tpl-test",
				"client_id":    "aone",
				"envd_version": "0.5.4",
				"domain":       "example.test",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sbx/sandboxes/sbx-test":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sandbox_id":        "sbx-test",
				"envd_sandbox_id":   "provider-sbx-test",
				"template_id":       "tpl-test",
				"client_id":         "aone",
				"envd_version":      "0.5.4",
				"envd_access_token": "envd-token",
				"state":             "running",
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))

	sb, err := c.Create(context.Background(), CreateParams{TemplateID: "tpl-test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sb.envdTokenMu.RLock()
	token := sb.envdAccessToken
	sb.envdTokenMu.RUnlock()
	if token == nil || *token != "envd-token" {
		t.Fatalf("envd token = %+v", token)
	}
	if got, want := sb.GetHost(49983), "49983-provider-sbx-test.example.test"; got != want {
		t.Fatalf("envd host = %q, want %q", got, want)
	}
}

func TestSandboxUsesEnvdSandboxIDForHosts(t *testing.T) {
	domain := "sandbox.example.test"
	sandboxID := "sbx-local"
	envdSandboxID := "provider-sandbox"
	sb := newSandbox(nil, &apis.Sandbox{
		SandboxID:     &sandboxID,
		EnvdSandboxID: &envdSandboxID,
		Domain:        &domain,
	})
	if got, want := sb.ID(), sandboxID; got != want {
		t.Fatalf("ID = %q, want %q", got, want)
	}
	if got, want := sb.GetHost(49983), "49983-provider-sandbox.sandbox.example.test"; got != want {
		t.Fatalf("host = %q, want %q", got, want)
	}
}
