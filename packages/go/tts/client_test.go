package tts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aonesuite/aone/packages/go/internal/sdkconfig"
)

func TestNewClientAppliesDefaultsAndAuth(t *testing.T) {
	t.Setenv(sdkconfig.EnvAPIKey, "env-key")
	t.Setenv(sdkconfig.EnvEndpoint, "https://api.example.test/")

	c, err := NewClient(nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got, want := c.config.APIKey, "env-key"; got != want {
		t.Errorf("APIKey = %q, want %q", got, want)
	}
	if got, want := c.config.Endpoint, "https://api.example.test"; got != want {
		t.Errorf("Endpoint = %q, want %q", got, want)
	}
}

func TestNewClientDoesNotMutateInputConfig(t *testing.T) {
	t.Setenv(sdkconfig.EnvAPIKey, "env-key")
	t.Setenv(sdkconfig.EnvEndpoint, "https://api.example.test/")

	cfg := &Config{}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if cfg.APIKey != "" || cfg.Endpoint != "" || cfg.HTTPClient != nil {
		t.Fatalf("input config was mutated: %+v", cfg)
	}
	if got, want := c.config.APIKey, "env-key"; got != want {
		t.Errorf("client APIKey = %q, want %q", got, want)
	}
	if got, want := c.config.Endpoint, "https://api.example.test"; got != want {
		t.Errorf("client Endpoint = %q, want %q", got, want)
	}
	if c.config.HTTPClient == nil {
		t.Error("client HTTPClient should default")
	}
}

func TestListVoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodGet; got != want {
			t.Errorf("method = %s, want %s", got, want)
		}
		if got, want := r.URL.Path, "/api/v1/tts/voices"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		if got, want := r.Header.Get("X-API-Key"), "secret"; got != want {
			t.Errorf("X-API-Key = %q, want %q", got, want)
		}
		writeJSON(t, w, map[string]any{
			"voices": []map[string]any{
				{
					"id":       "voice-1",
					"name":     "Ava",
					"language": "en-US",
					"gender":   "female",
					"scenario": "assistant",
				},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(&Config{APIKey: "secret", Endpoint: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	voices, err := c.ListVoices(context.Background())
	if err != nil {
		t.Fatalf("ListVoices: %v", err)
	}
	if len(voices) != 1 {
		t.Fatalf("voices len = %d, want 1", len(voices))
	}
	if got, want := voices[0].ID, "voice-1"; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
	if got, want := voices[0].Scenario, "assistant"; got != want {
		t.Errorf("Scenario = %q, want %q", got, want)
	}
}

func TestListVoicesEmptyResponse(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{name: "missing voices", body: map[string]any{}},
		{name: "empty voices", body: map[string]any{"voices": []any{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, tc.body)
			}))
			defer srv.Close()

			c, err := NewClient(&Config{Endpoint: srv.URL, HTTPClient: srv.Client()})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			voices, err := c.ListVoices(context.Background())
			if err != nil {
				t.Fatalf("ListVoices: %v", err)
			}
			if len(voices) != 0 {
				t.Fatalf("voices len = %d, want 0", len(voices))
			}
		})
	}
}

func TestSynthesize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Errorf("method = %s, want %s", got, want)
		}
		if got, want := r.URL.Path, "/api/v1/tts/speech"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got, want := body["text"], "hello"; got != want {
			t.Errorf("text = %v, want %v", got, want)
		}
		if got, want := body["voice"], "voice-1"; got != want {
			t.Errorf("voice = %v, want %v", got, want)
		}
		if got, want := body["format"], "mp3"; got != want {
			t.Errorf("format = %v, want %v", got, want)
		}
		if got, want := body["speed"], 1.25; got != want {
			t.Errorf("speed = %v, want %v", got, want)
		}
		writeJSON(t, w, map[string]any{
			"audio_url":   "https://cdn.example.test/audio.mp3",
			"duration_ms": 1200,
		})
	}))
	defer srv.Close()

	c, err := NewClient(&Config{Endpoint: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	speed := float32(1.25)
	format := "mp3"
	out, err := c.Synthesize(context.Background(), SynthesizeParams{
		Text:   "hello",
		Voice:  "voice-1",
		Format: &format,
		Speed:  &speed,
	})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if got, want := out.AudioURL, "https://cdn.example.test/audio.mp3"; got != want {
		t.Errorf("AudioURL = %q, want %q", got, want)
	}
	if got, want := out.DurationMs, int32(1200); got != want {
		t.Errorf("DurationMs = %d, want %d", got, want)
	}
}

func TestListVoicesReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-123")
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":  "rate_limited",
			"error": "slow down",
		})
	}))
	defer srv.Close()

	c, err := NewClient(&Config{Endpoint: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.ListVoices(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusTooManyRequests)
	}
	if apiErr.Code != "rate_limited" || apiErr.Message != "slow down" {
		t.Errorf("Code/Message = %q/%q", apiErr.Code, apiErr.Message)
	}
	if apiErr.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want req-123", apiErr.RequestID)
	}
	if apiErr.RetryAfter != 3*time.Second {
		t.Errorf("RetryAfter = %v, want 3s", apiErr.RetryAfter)
	}
}

func TestListVoicesReturnsAPIErrorForPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("plain failure"))
	}))
	defer srv.Close()

	c, err := NewClient(&Config{Endpoint: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.ListVoices(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.Message != "plain failure" {
		t.Errorf("Message = %q, want plain failure", apiErr.Message)
	}
}

func TestAPIKeyEditorPreservesAuthorization(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer caller")
	if err := apiKeyEditor("secret")(context.Background(), req); err != nil {
		t.Fatalf("apiKeyEditor: %v", err)
	}
	if got := req.Header.Get("X-API-Key"); got != "" {
		t.Errorf("X-API-Key = %q, want empty", got)
	}
	if got, want := req.Header.Get("Authorization"), "Bearer caller"; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestRequestTimeoutTransport(t *testing.T) {
	rt := requestTimeoutTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected request deadline")
			}
			if time.Until(deadline) <= 0 {
				t.Fatal("deadline should be in the future")
			}
			return (&http.Transport{}).RoundTrip(req)
		}),
		timeout: time.Minute,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
}

func TestRequestTimeoutTransportRespectsExistingDeadline(t *testing.T) {
	deadline := time.Now().Add(2 * time.Hour)
	rt := requestTimeoutTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected request deadline")
			}
			if !got.Equal(deadline) {
				t.Fatalf("deadline = %v, want %v", got, deadline)
			}
			return (&http.Transport{}).RoundTrip(req)
		}),
		timeout: time.Minute,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
}

func TestSynthesizeRejectsMissingRequiredFields(t *testing.T) {
	c, err := NewClient(&Config{Endpoint: "https://tts.example.test"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.Synthesize(context.Background(), SynthesizeParams{Voice: "voice-1"}); err == nil {
		t.Fatal("Synthesize should reject empty text")
	}
	if _, err := c.Synthesize(context.Background(), SynthesizeParams{Text: "hello"}); err == nil {
		t.Fatal("Synthesize should reject empty voice")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}
