package sandbox

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aonesuite/aone/internal/config"
	"github.com/aonesuite/aone/packages/go/sandbox"
	"github.com/subosito/gotenv"
)

// keepalivePingIntervalSec matches the upstream SDK's KEEPALIVE_PING_INTERVAL_SEC (50s).
// This header tells the envd server to send periodic keepalive pings on gRPC streams,
// preventing proxies/load balancers from closing idle connections.
const keepalivePingIntervalSec = "50"

// keepalivePingHeader is the HTTP header name for the keepalive ping interval.
const keepalivePingHeader = "Keepalive-Ping-Interval"

// keepaliveTransport wraps an http.RoundTripper to inject the Keepalive-Ping-Interval header.
type keepaliveTransport struct {
	base http.RoundTripper
}

// RoundTrip adds the keepalive header before delegating the request to the
// wrapped transport.
func (t *keepaliveTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(keepalivePingHeader, keepalivePingIntervalSec)
	return t.base.RoundTrip(req)
}

// Environment variable names retained for backward compatibility. New code
// should prefer the constants exported from internal/config.
const (
	// EnvAoneSandboxAPIURL is the legacy export of config.EnvEndpoint.
	EnvAoneSandboxAPIURL = config.EnvEndpoint
	// EnvAoneAPIKey is the legacy export of config.EnvAPIKey.
	EnvAoneAPIKey = config.EnvAPIKey
)

// loadDotEnv loads variables from the .env file in the current directory.
// Only variables not already set in the OS environment are loaded (OS takes priority).
// Missing or unreadable .env files are silently ignored.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	env, err := gotenv.StrictParse(f)
	if err != nil {
		return
	}
	for key, value := range env {
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, strings.TrimSpace(value))
		}
	}
}

// NewSandboxClient creates a new sandbox client by applying the standard
// credential precedence chain (flag > env > config file > default).
//
// Callers that need to honor CLI flags should use NewSandboxClientWithResolver
// directly; this helper resolves with no flag overrides for the common path.
func NewSandboxClient() (*sandbox.Client, error) {
	return NewSandboxClientWithResolver(config.Resolver{})
}

// NewSandboxClientWithResolver builds a client using the given Resolver,
// surfacing flag-level overrides on top of env + config file layers.
func NewSandboxClientWithResolver(r config.Resolver) (*sandbox.Client, error) {
	// Honor a project-local .env so users don't have to export every time
	// they switch repos. OS env still wins if both are set.
	loadDotEnv()

	resolved, err := r.Resolve()
	if err != nil {
		return nil, err
	}
	if resolved.APIKey == "" {
		return nil, fmt.Errorf("API key not configured: run `aone auth login --api-key <key>` or set %s", config.EnvAPIKey)
	}

	return sandbox.NewClient(&sandbox.Config{
		APIKey:   resolved.APIKey,
		Endpoint: resolved.Endpoint,
		HTTPClient: &http.Client{
			Transport: &keepaliveTransport{base: http.DefaultTransport},
		},
	})
}

// MaskAPIKey returns a redacted form of the API key suitable for display in
// `aone auth info`: keep a 4-char prefix and a 4-char suffix, replace the
// middle with `****`. Short keys collapse to all asterisks.
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", 4) + key[len(key)-4:]
}
