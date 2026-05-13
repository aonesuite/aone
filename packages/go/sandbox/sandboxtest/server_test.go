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
	srv.Handle("DELETE", "/api/v1/sbx/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
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
		{"/api/v1/sbx/sandboxes", "/api/v1/sbx/sandboxes", true},
		{"/api/v1/sbx/sandboxes/{id}", "/api/v1/sbx/sandboxes/abc", true},
		{"/api/v1/sbx/sandboxes/{id}", "/api/v1/sbx/sandboxes/abc/extra", false},
		{"/api/v1/sbx/sandboxes", "/api/v1/sbx/sandboxes", true},
		{"/api/v1/sbx/sandboxes", "/api/v1/sbx/templates", false},
	}
	for _, tc := range cases {
		if got := matchPath(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchPath(%q,%q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// TestSawAndRequestsFor verifies the convenience helpers added to dedupe the
// sawRequest function previously redefined in every CLI test package.
func TestSawAndRequestsFor(t *testing.T) {
	srv := NewServer(t)
	c, err := sandbox.NewClient(&sandbox.Config{
		APIKey:     "k",
		Endpoint:   srv.URL(),
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	sb, err := c.Create(context.Background(), sandbox.CreateParams{TemplateID: "tpl"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_ = sb.Kill(context.Background())

	if !srv.Saw("POST", "/api/v1/sbx/sandboxes") {
		t.Fatalf("Saw missed POST /sandboxes")
	}
	if !srv.Saw("delete", "/api/v1/sbx/sandboxes/sbx-test") { // case-insensitive method
		t.Fatalf("Saw missed DELETE /sandboxes/sbx-test (case-insensitive)")
	}
	if srv.Saw("GET", "/does-not-exist") {
		t.Fatalf("Saw matched a path that wasn't requested")
	}
	if got := srv.RequestsFor("POST", "/api/v1/sbx/sandboxes"); len(got) != 1 {
		t.Fatalf("RequestsFor POST /sandboxes len = %d, want 1", len(got))
	}
}

// TestWithUploadTarget covers the upload-target helper used by Dockerfile
// build tests. A PUT to the returned URL should bump the hits counter.
func TestWithUploadTarget(t *testing.T) {
	srv := NewServer(t)
	url, hits := srv.WithUploadTarget("/upload-target", http.StatusOK)
	if url == "" {
		t.Fatal("WithUploadTarget returned empty URL")
	}
	req, err := http.NewRequest(http.MethodPut, url, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if *hits != 1 {
		t.Fatalf("hits = %d, want 1", *hits)
	}
}
