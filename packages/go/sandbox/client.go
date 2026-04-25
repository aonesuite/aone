package sandbox

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
)

// DefaultEndpoint is the public Sandbox API endpoint used when Config.Endpoint
// is empty. Applications can override it for private deployments, staging
// environments, or tests that point the SDK at a local HTTP server.
const DefaultEndpoint = "https://sandbox.aonesuite.com"

// AllTraffic is the CIDR that matches every IPv4 address; use it in
// NetworkConfig.AllowOut / DenyOut to express "all outbound traffic".
const AllTraffic = "0.0.0.0/0"

// Environment variable names read by Config when its corresponding field is
// empty. These mirror the CLI-layer names so a process that sets them once
// keeps working for both the CLI and direct SDK consumers.
const (
	// EnvAPIKey overrides Config.APIKey when the field is empty.
	EnvAPIKey = "AONE_API_KEY"
	// EnvEndpoint overrides Config.Endpoint when the field is empty.
	EnvEndpoint = "AONE_SANDBOX_API_URL"
	// EnvDebug toggles SDK-level debug behavior when set to "1" / "true".
	EnvDebug = "AONE_DEBUG"
)

// Config controls how a Client authenticates and sends HTTP requests to the
// Sandbox API. Leave optional fields empty to use the SDK defaults.
type Config struct {
	// APIKey is sent as the X-API-Key header on Sandbox API requests. When
	// empty, NewClient falls back to the AONE_API_KEY environment variable.
	APIKey string

	// Credentials is accepted for source compatibility with earlier SDK versions.
	// Injection-rule APIs are intentionally not implemented in this package.
	Credentials any

	// Endpoint overrides DefaultEndpoint. When empty, NewClient falls back to
	// the AONE_SANDBOX_API_URL environment variable and finally DefaultEndpoint.
	Endpoint string

	// HTTPClient is used for all API, file, and envd requests. If nil, the SDK
	// uses http.DefaultClient. Supplying a custom client is the recommended way
	// to configure transports, proxies, or test doubles. Per-request timeouts
	// should be set via RequestTimeout rather than HTTPClient.Timeout so they
	// do not interfere with long-running envd streams.
	HTTPClient *http.Client

	// RequestTimeout applies to individual control-plane API calls when the
	// caller's context.Context has no deadline. Zero disables the default and
	// leaves timeout management entirely to the caller. Long-lived envd
	// streams (commands, PTY, watch) are not affected.
	RequestTimeout time.Duration

	// Debug enables verbose SDK-level logging. When false, NewClient honors
	// the AONE_DEBUG env var (values "1", "true", "yes").
	Debug bool
}

// Client is the top-level Sandbox SDK entry point. It owns the shared API
// client and configuration used to create, connect to, list, and inspect
// sandboxes.
type Client struct {
	config *Config
	api    apis.ClientWithResponsesInterface
}

// NewClient constructs a Sandbox API client from Config. The function fills in
// SDK defaults for empty optional fields and validates that the generated API
// client can be initialized for the selected endpoint.
//
// Env-var fallbacks: when APIKey is empty, NewClient reads AONE_API_KEY; when
// Endpoint is empty it reads AONE_SANDBOX_API_URL and finally DefaultEndpoint;
// when Debug is false it reads AONE_DEBUG.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = &Config{}
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv(EnvAPIKey)
	}
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv(EnvEndpoint)
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	config.Endpoint = strings.TrimRight(endpoint, "/")
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	if !config.Debug {
		config.Debug = parseBoolEnv(os.Getenv(EnvDebug))
	}

	opts := []apis.ClientOption{
		apis.WithHTTPClient(config.HTTPClient),
		apis.WithRequestEditorFn(reqidEditor()),
	}
	if config.APIKey != "" {
		opts = append(opts, apis.WithRequestEditorFn(apiKeyEditor(config.APIKey)))
	}
	if config.RequestTimeout > 0 {
		opts = append(opts, apis.WithRequestEditorFn(requestTimeoutEditor(config.RequestTimeout)))
	}

	client, err := apis.NewClientWithResponses(config.Endpoint, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{config: config, api: client}, nil
}

func reqidEditor() apis.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		setReqidHeader(ctx, req)
		return nil
	}
}

// apiKeyEditor injects API key authentication headers. To stay compatible
// with gateways that accept either form, it sets both X-API-Key and
// Authorization: Bearer. If the caller already pre-set an Authorization
// header (for example a request signed with custom credentials), the
// editor leaves it alone and skips both headers so the caller's choice
// wins.
func apiKeyEditor(apiKey string) apis.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}
}

// requestTimeoutEditor injects a context deadline for control-plane calls that
// arrive with no caller-set deadline. Callers that supply their own deadline
// via ctx keep full control.
func requestTimeoutEditor(d time.Duration) apis.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if _, ok := ctx.Deadline(); ok {
			return nil
		}
		// Best-effort: attach a derived context. The generated client reuses
		// the request context, so adjusting req.Context is what oapi-codegen
		// honors downstream.
		derived, cancel := context.WithTimeout(ctx, d)
		// The cancel func is intentionally captured by AfterFunc so it fires
		// when the parent ctx or the request completes, avoiding a leak
		// without blocking the editor. Go 1.21+.
		context.AfterFunc(derived, cancel)
		*req = *req.WithContext(derived)
		return nil
	}
}

func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
