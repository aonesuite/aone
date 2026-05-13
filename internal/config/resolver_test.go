package config

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/log"
)

// resolverEnv isolates a resolver test by pointing the config home at a temp
// dir and clearing the credential env vars unless the test sets them itself.
func resolverEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvEndpoint, "")
	return dir
}

func TestResolve_DefaultsWhenNothingProvided(t *testing.T) {
	resolverEnv(t)

	got, err := Resolver{}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != "" || got.APIKeySource != SourceNone {
		t.Fatalf("APIKey = %q (%s), want empty/SourceNone", got.APIKey, got.APIKeySource)
	}
	if got.Endpoint != "https://api.aonesuite.com" || got.EndpointSource != SourceDefault {
		t.Fatalf("Endpoint = %q (%s), want default https://api.aonesuite.com", got.Endpoint, got.EndpointSource)
	}
}

func TestResolve_FileLayerWins(t *testing.T) {
	resolverEnv(t)
	if err := Save(&File{APIKey: "k-file", Endpoint: "https://file"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Resolver{}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != "k-file" || got.APIKeySource != SourceFile {
		t.Fatalf("APIKey = %q (%s)", got.APIKey, got.APIKeySource)
	}
	if got.Endpoint != "https://file" || got.EndpointSource != SourceFile {
		t.Fatalf("Endpoint = %q (%s)", got.Endpoint, got.EndpointSource)
	}
}

func TestResolve_EnvBeatsFile(t *testing.T) {
	resolverEnv(t)
	if err := Save(&File{APIKey: "k-file", Endpoint: "https://file"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv(EnvAPIKey, "k-env")
	t.Setenv(EnvEndpoint, "https://env")

	got, err := Resolver{}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != "k-env" || got.APIKeySource != SourceEnv {
		t.Fatalf("APIKey = %q (%s)", got.APIKey, got.APIKeySource)
	}
	if got.Endpoint != "https://env" || got.EndpointSource != SourceEnv {
		t.Fatalf("Endpoint = %q (%s)", got.Endpoint, got.EndpointSource)
	}
}

func TestResolve_FlagBeatsEverything(t *testing.T) {
	resolverEnv(t)
	if err := Save(&File{APIKey: "k-file", Endpoint: "https://file"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv(EnvAPIKey, "k-env")
	t.Setenv(EnvEndpoint, "https://env")

	got, err := Resolver{
		FlagAPIKey:   "k-flag",
		FlagEndpoint: "https://flag",
	}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != "k-flag" || got.APIKeySource != SourceFlag {
		t.Fatalf("APIKey = %q (%s)", got.APIKey, got.APIKeySource)
	}
	if got.Endpoint != "https://flag" || got.EndpointSource != SourceFlag {
		t.Fatalf("Endpoint = %q (%s)", got.Endpoint, got.EndpointSource)
	}
}

func TestResolve_APIKeyAndEndpointResolveIndependently(t *testing.T) {
	// Flag for API key, env for endpoint, file should be ignored on both.
	resolverEnv(t)
	if err := Save(&File{APIKey: "k-file", Endpoint: "https://file"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv(EnvEndpoint, "https://env")

	got, err := Resolver{FlagAPIKey: "k-flag"}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != "k-flag" || got.APIKeySource != SourceFlag {
		t.Fatalf("APIKey = %q (%s)", got.APIKey, got.APIKeySource)
	}
	if got.Endpoint != "https://env" || got.EndpointSource != SourceEnv {
		t.Fatalf("Endpoint = %q (%s)", got.Endpoint, got.EndpointSource)
	}
}

func TestResolve_EmptyFlagDoesNotShadowEnv(t *testing.T) {
	resolverEnv(t)
	t.Setenv(EnvAPIKey, "k-env")

	got, err := Resolver{FlagAPIKey: ""}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != "k-env" || got.APIKeySource != SourceEnv {
		t.Fatalf("APIKey = %q (%s); empty flag must not shadow env", got.APIKey, got.APIKeySource)
	}
}

// TestResolve_DebugLogRedactsEndpoint is the regression test for the
// endpoint leak: a misconfigured endpoint with userinfo or a query token
// must not appear verbatim in the DEBUG log, since `config picks` lines
// frequently end up in support tickets.
func TestResolve_DebugLogRedactsEndpoint(t *testing.T) {
	resolverEnv(t)
	t.Setenv(EnvEndpoint, "https://user:hunter2@api.example.com/v1?token=SECRETTOKEN")
	t.Setenv(EnvAPIKey, "k-env")

	var buf bytes.Buffer
	log.Init(log.InitOptions{
		ResolveOptions: log.ResolveOptions{DebugFlag: true, Env: func(string) string { return "" }},
		Stderr:         &buf,
	})
	t.Cleanup(func() {
		log.Init(log.InitOptions{ResolveOptions: log.ResolveOptions{Env: func(string) string { return "" }}})
	})

	if _, err := (Resolver{}).Resolve(); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "hunter2") {
		t.Fatalf("userinfo password leaked: %q", out)
	}
	if strings.Contains(out, "SECRETTOKEN") {
		t.Fatalf("query token leaked: %q", out)
	}
	if !strings.Contains(out, "api.example.com") {
		t.Fatalf("host should still be visible: %q", out)
	}
}
