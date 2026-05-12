package config

import (
	"os"

	"github.com/aonesuite/aone/internal/log"
	"github.com/aonesuite/aone/packages/go/sandbox"
)

// DefaultEndpoint is the canonical control-plane URL used when no override
// is supplied via flag, env, or config file.
var DefaultEndpoint = sandbox.DefaultEndpoint

// Resolved holds the final API key + endpoint after applying the precedence
// chain, plus the source layers each value came from.
type Resolved struct {
	APIKey         string
	APIKeySource   Source
	Endpoint       string
	EndpointSource Source
}

// Resolver collects per-invocation overrides (typically from CLI flags) and
// applies them on top of the env + file layers. The precedence is:
//
//	flag > env > config file > default
//
// A zero-value Resolver behaves as "no flag overrides" and is safe to use.
type Resolver struct {
	// FlagAPIKey, if non-empty, wins over env and file.
	FlagAPIKey string
	// FlagEndpoint, if non-empty, wins over env and file.
	FlagEndpoint string
}

// Resolve returns the merged credentials with their sources. It never panics
// on a missing file: a clean install with no env vars simply yields a
// Resolved with empty APIKey and SourceNone.
func (r Resolver) Resolve() (*Resolved, error) {
	f, err := Load()
	if err != nil {
		return nil, err
	}

	// Path() can't fail here in any way Load() didn't already surface,
	// but we still log it so users know which config file was consulted.
	if p, perr := Path(); perr == nil {
		log.Debug("config resolved",
			"config_path", p,
			"file_has_api_key", f.APIKey != "",
			"file_has_endpoint", f.Endpoint != "",
		)
	}

	out := &Resolved{}

	switch {
	case r.FlagAPIKey != "":
		out.APIKey = r.FlagAPIKey
		out.APIKeySource = SourceFlag
	case os.Getenv(EnvAPIKey) != "":
		out.APIKey = os.Getenv(EnvAPIKey)
		out.APIKeySource = SourceEnv
	case f.APIKey != "":
		out.APIKey = f.APIKey
		out.APIKeySource = SourceFile
	default:
		out.APIKeySource = SourceNone
	}

	switch {
	case r.FlagEndpoint != "":
		out.Endpoint = r.FlagEndpoint
		out.EndpointSource = SourceFlag
	case os.Getenv(EnvEndpoint) != "":
		out.Endpoint = os.Getenv(EnvEndpoint)
		out.EndpointSource = SourceEnv
	case f.Endpoint != "":
		out.Endpoint = f.Endpoint
		out.EndpointSource = SourceFile
	default:
		out.Endpoint = DefaultEndpoint
		out.EndpointSource = SourceDefault
	}

	// Final picks; mask the API key so even debug logs don't leak it,
	// and run the endpoint through RedactURL in case it was configured
	// with embedded userinfo or query-string credentials.
	log.Debug("config picks",
		"api_key_source", string(out.APIKeySource),
		"api_key", maskKey(out.APIKey),
		"endpoint_source", string(out.EndpointSource),
		"endpoint", log.RedactURL(out.Endpoint),
	)

	return out, nil
}

// maskKey returns a redacted form of the API key suitable for logs:
// keep 4 char prefix + 4 char suffix, replace middle with ****. Mirrors
// sandbox.MaskAPIKey but defined locally to avoid an import cycle.
func maskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "********"
	}
	return k[:4] + "****" + k[len(k)-4:]
}
