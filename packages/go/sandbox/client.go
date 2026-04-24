package sandbox

import (
	"context"
	"net/http"
	"strings"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
)

// DefaultEndpoint is the public Sandbox API endpoint used when Config.Endpoint
// is empty. Applications can override it for private deployments, staging
// environments, or tests that point the SDK at a local HTTP server.
const DefaultEndpoint = "https://sandbox.aonesuite.com"

// Config controls how a Client authenticates and sends HTTP requests to the
// Sandbox API. Leave optional fields empty to use the SDK defaults.
type Config struct {
	// APIKey is sent as the X-API-Key header on Sandbox API requests.
	// It can be left empty when callers inject authentication with HTTPClient or
	// when a custom endpoint does not require API-key authentication.
	APIKey string

	// AccessToken, when set, is sent as "Authorization: Bearer <token>" on
	// control-plane requests. Template build / management endpoints
	// authenticate the user via access token rather than API key; supplying
	// both is the common case (API key for sandbox ops, access token for
	// template ops). The server picks whichever header the endpoint requires.
	AccessToken string

	// Credentials is accepted for source compatibility with earlier SDK versions.
	// Injection-rule APIs are intentionally not implemented in this package.
	Credentials any

	// Endpoint overrides DefaultEndpoint. The value should include the scheme,
	// for example "https://sandbox.aonesuite.com".
	Endpoint string

	// HTTPClient is used for all API, file, and envd requests. If nil, the SDK
	// uses http.DefaultClient. Supplying a custom client is the recommended way
	// to configure timeouts, transports, proxies, or test doubles.
	HTTPClient *http.Client
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
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = &Config{}
	}
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	config.Endpoint = strings.TrimRight(endpoint, "/")
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}

	opts := []apis.ClientOption{
		apis.WithHTTPClient(config.HTTPClient),
		apis.WithRequestEditorFn(reqidEditor()),
	}
	if config.APIKey != "" {
		opts = append(opts, apis.WithRequestEditorFn(apiKeyEditor(config.APIKey)))
	}
	if config.AccessToken != "" {
		opts = append(opts, apis.WithRequestEditorFn(accessTokenEditor(config.AccessToken)))
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

func apiKeyEditor(apiKey string) apis.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		req.Header.Set("X-API-Key", apiKey)
		return nil
	}
}

// accessTokenEditor sets an "Authorization: Bearer" header for requests that
// require user-scoped auth (template management / build). When both an API key
// and an access token are configured, the backend selects whichever header
// applies to the endpoint being called.
func accessTokenEditor(token string) apis.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}
