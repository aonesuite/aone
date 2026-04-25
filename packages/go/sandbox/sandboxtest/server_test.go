package sandboxtest

import (
	"context"
	"net/http"
	"testing"

	"github.com/aonesuite/aone/packages/go/sandbox"
)

// TestServer_DefaultsCoverSandboxRoundtrip verifies the default routes are
// sufficient for the SDK's standard create→get→kill cycle without overrides.
func TestServer_DefaultsCoverSandboxRoundtrip(t *testing.T) {
	srv := NewServer(t)

	c, err := sandbox.NewClient(&sandbox.Config{
		APIKey:     "test-key",
		Endpoint:   srv.URL(),
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	sb, err := c.Create(ctx, sandbox.CreateParams{TemplateID: "tpl-test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sb.ID() == "" {
		t.Fatalf("ID empty")
	}

	if _, err := sb.GetInfo(ctx); err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if err := sb.Kill(ctx); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	// Recorded request log should include create + at least one info get.
	reqs := srv.Requests()
	if len(reqs) < 3 {
		t.Fatalf("recorded %d requests, want >=3: %+v", len(reqs), reqs)
	}
}

// TestServer_HandleOverridesDefault confirms a registered handler replaces the
// stock response.
func TestServer_HandleOverridesDefault(t *testing.T) {
	srv := NewServer(t)

	called := false
	srv.Handle("DELETE", "/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	})

	c, err := sandbox.NewClient(&sandbox.Config{
		APIKey:     "k",
		Endpoint:   srv.URL(),
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Build a Sandbox bound to this client by going through Create.
	sb, err := c.Create(context.Background(), sandbox.CreateParams{TemplateID: "tpl"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := sb.Kill(context.Background()); err == nil {
		t.Fatal("expected Kill to surface 500")
	}
	if !called {
		t.Fatal("override not invoked")
	}
}

// TestMatchPath spot-checks the route-pattern matcher used by dispatch.
func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/sandboxes", "/sandboxes", true},
		{"/sandboxes/{id}", "/sandboxes/abc", true},
		{"/sandboxes/{id}", "/sandboxes/abc/extra", false},
		{"/sandboxes/{id}/pause", "/sandboxes/abc/pause", true},
		{"/sandboxes/{id}/pause", "/sandboxes/abc/resume", false},
		{"/v2/sandboxes", "/v2/sandboxes", true},
		{"/v2/sandboxes", "/sandboxes", false},
	}
	for _, tc := range cases {
		if got := matchPath(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchPath(%q,%q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
