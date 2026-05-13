package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aonesuite/aone/packages/go/internal/aoneapi"
	"github.com/aonesuite/aone/packages/go/internal/sdkconfig"
)

// Config controls how a Client authenticates and sends HTTP requests to the TTS
// API. Leave optional fields empty to use the SDK defaults.
type Config struct {
	// APIKey is sent as the X-API-Key header on TTS API requests. When empty,
	// NewClient falls back to the AONE_API_KEY environment variable.
	APIKey string

	// Endpoint overrides the SDK default endpoint. When empty, NewClient falls
	// back to the shared Aone endpoint environment variable and finally the
	// default endpoint.
	Endpoint string

	// HTTPClient is used for API requests. If nil, the SDK uses
	// http.DefaultClient.
	HTTPClient *http.Client

	// RequestTimeout applies to individual API calls when the caller's context
	// has no deadline. Zero disables the default and leaves timeout management
	// entirely to the caller.
	RequestTimeout time.Duration
}

// Client is the top-level TTS SDK entry point.
type Client struct {
	config *Config
	api    *aoneapi.ClientWithResponses
}

// NewClient constructs a TTS API client from Config. The function fills in SDK
// defaults for empty optional fields and validates that the generated API client
// can be initialized for the selected endpoint.
func NewClient(config *Config) (*Client, error) {
	cfg := Config{}
	if config == nil {
		config = &cfg
	} else {
		cfg = *config
		config = &cfg
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv(sdkconfig.EnvAPIKey)
	}
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv(sdkconfig.EnvEndpoint)
	}
	if endpoint == "" {
		endpoint = sdkconfig.DefaultEndpoint
	}
	config.Endpoint = strings.TrimRight(endpoint, "/")
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	if config.RequestTimeout > 0 {
		config.HTTPClient = httpClientWithRequestTimeout(config.HTTPClient, config.RequestTimeout)
	}

	opts := []aoneapi.ClientOption{
		aoneapi.WithHTTPClient(config.HTTPClient),
	}
	if config.APIKey != "" {
		opts = append(opts, aoneapi.WithRequestEditorFn(apiKeyEditor(config.APIKey)))
	}

	client, err := aoneapi.NewClientWithResponses(config.Endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{config: config, api: client}, nil
}

// SynthesizeParams contains the inputs for synthesizing text into audio.
type SynthesizeParams struct {
	// Text is the source text to synthesize.
	Text string
	// Voice is the provider voice identifier to synthesize with.
	Voice string
	// Format selects the requested audio format, such as mp3.
	Format *string
	// Speed controls relative speech speed where 1.0 is provider default.
	Speed *float32
}

// SynthesizeResponse describes synthesized audio returned by the API.
type SynthesizeResponse struct {
	// AudioURL is a URL containing synthesized audio.
	AudioURL string
	// DurationMs is the optional audio duration in milliseconds.
	DurationMs int32
}

// Voice describes one available TTS voice.
type Voice struct {
	// ID is the provider voice identifier used as SynthesizeParams.Voice.
	ID string
	// Name is the human-readable voice name.
	Name string
	// Language is the primary voice language.
	Language string
	// Gender is the provider-declared voice gender when available.
	Gender string
	// Scenario describes the voice use case or style when available.
	Scenario string
}

// APIError represents a non-successful HTTP response from the TTS API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
	RetryAfter time.Duration
	Body       []byte
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("tts api error: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("tts api error: status=%d message=%s", e.StatusCode, e.Message)
	}
	if e.Code != "" {
		return fmt.Sprintf("tts api error: status=%d code=%s", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("tts api error: status=%d", e.StatusCode)
}

// ListVoices returns the voices available to the authenticated caller.
func (c *Client) ListVoices(ctx context.Context) ([]Voice, error) {
	resp, err := c.api.GithubComAonesuiteInfraInternalProductsTtsHandlerModuleListVoicesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body, resp.JSONDefault)
	}
	if resp.JSON200.Voices == nil {
		return nil, nil
	}
	voices := make([]Voice, 0, len(*resp.JSON200.Voices))
	for _, v := range *resp.JSON200.Voices {
		voices = append(voices, voiceFromAPI(v))
	}
	return voices, nil
}

// Synthesize converts text to speech and returns the generated audio location.
func (c *Client) Synthesize(ctx context.Context, params SynthesizeParams) (*SynthesizeResponse, error) {
	if strings.TrimSpace(params.Text) == "" {
		return nil, fmt.Errorf("text is required")
	}
	if strings.TrimSpace(params.Voice) == "" {
		return nil, fmt.Errorf("voice is required")
	}
	body := aoneapi.GithubComAonesuiteInfraInternalProductsTtsHandlerModuleSynthesizeJSONRequestBody{
		Text:   params.Text,
		Voice:  params.Voice,
		Format: params.Format,
		Speed:  params.Speed,
	}
	resp, err := c.api.GithubComAonesuiteInfraInternalProductsTtsHandlerModuleSynthesizeWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body, resp.JSONDefault)
	}
	return synthesizeResponseFromAPI(resp.JSON200), nil
}

func apiKeyEditor(apiKey string) aoneapi.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}
}

func httpClientWithRequestTimeout(base *http.Client, timeout time.Duration) *http.Client {
	copy := *base
	transport := copy.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	copy.Transport = requestTimeoutTransport{base: transport, timeout: timeout}
	return &copy
}

type requestTimeoutTransport struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (t requestTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, ok := req.Context().Deadline(); ok || t.timeout <= 0 {
		return t.base.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.Body == nil {
		cancel()
		return resp, nil
	}
	resp.Body = cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

func newAPIError(resp *http.Response, body []byte, apiErr *aoneapi.HTTPError) *APIError {
	err := &APIError{Body: body}
	if resp != nil {
		err.StatusCode = resp.StatusCode
		err.RequestID = resp.Header.Get("X-Request-Id")
		err.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	}
	if apiErr != nil {
		err.Code = apiErr.Code
		err.Message = apiErr.Error
	}
	if err.Message == "" {
		err.Message = strings.TrimSpace(string(body))
	}
	return err
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if seconds, convErr := strconv.Atoi(v); convErr == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	t, parseErr := http.ParseTime(v)
	if parseErr != nil {
		return 0
	}
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func synthesizeResponseFromAPI(in *aoneapi.HandlerSynthesizeResponse) *SynthesizeResponse {
	if in == nil {
		return nil
	}
	return &SynthesizeResponse{
		AudioURL:   stringValue(in.AudioURL),
		DurationMs: int32Value(in.DurationMs),
	}
}

func voiceFromAPI(in aoneapi.HandlerVoiceResponse) Voice {
	return Voice{
		ID:       stringValue(in.ID),
		Name:     stringValue(in.Name),
		Language: stringValue(in.Language),
		Gender:   stringValue(in.Gender),
		Scenario: stringValue(in.Scenario),
	}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}
