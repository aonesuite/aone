// Package sdkconfig contains shared Go SDK configuration defaults.
package sdkconfig

const (
	// DefaultEndpoint is the public Aone API endpoint used when package-specific
	// Config.Endpoint values are empty.
	DefaultEndpoint = "https://api.aonesuite.com"

	// EnvEndpoint is the shared environment variable overriding the Aone API
	// endpoint.
	EnvEndpoint = "AONE_API_URL"

	// EnvAPIKey is the shared environment variable holding the Aone API key.
	EnvAPIKey = "AONE_API_KEY"
)
