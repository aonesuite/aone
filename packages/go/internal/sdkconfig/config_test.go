package sdkconfig

import "testing"

func TestSharedAPIConfigConstants(t *testing.T) {
	if got, want := EnvAPIKey, "AONE_API_KEY"; got != want {
		t.Errorf("EnvAPIKey = %q, want %q", got, want)
	}
	if got, want := EnvEndpoint, "AONE_API_URL"; got != want {
		t.Errorf("EnvEndpoint = %q, want %q", got, want)
	}
	if got, want := DefaultEndpoint, "https://api.aonesuite.com"; got != want {
		t.Errorf("DefaultEndpoint = %q, want %q", got, want)
	}
}
