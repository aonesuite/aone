package config

import (
	"os"

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

	return out, nil
}
