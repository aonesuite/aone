package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
)

// Sentinel errors mirror the error taxonomy exposed by the E2B SDK so callers
// can discriminate failure modes with errors.Is / errors.As instead of parsing
// status codes or message strings. Most errors returned by this SDK are
// APIError values that wrap one of these sentinels.
var (
	// ErrNotFound is the parent of all "resource missing" errors. It is
	// wrapped by ErrSandboxNotFound and ErrFileNotFound. Callers that only
	// care about generic 404 semantics can match this one.
	ErrNotFound = errors.New("not found")

	// ErrSandboxNotFound is returned when a sandbox ID does not exist or is
	// no longer running. Wraps ErrNotFound.
	ErrSandboxNotFound = fmt.Errorf("sandbox %w", ErrNotFound)

	// ErrFileNotFound is returned when a file or directory inside a sandbox
	// or volume is missing. Wraps ErrNotFound.
	ErrFileNotFound = fmt.Errorf("file %w", ErrNotFound)

	// ErrAuthentication is returned when the API key / access token is
	// missing, invalid, or revoked.
	ErrAuthentication = errors.New("authentication failed")

	// ErrGitAuth is returned when git credentials are rejected by the
	// upstream. Wraps ErrAuthentication.
	ErrGitAuth = fmt.Errorf("git %w", ErrAuthentication)

	// ErrGitUpstream is returned when the git remote is unreachable or
	// misconfigured.
	ErrGitUpstream = errors.New("git upstream error")

	// ErrInvalidArgument is returned for 400-class validation failures.
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrNotEnoughSpace is returned when the server reports disk pressure
	// (507 Insufficient Storage, or parsed from error code).
	ErrNotEnoughSpace = errors.New("not enough space")

	// ErrRateLimited is returned when the caller exceeds the API rate limit
	// (HTTP 429).
	ErrRateLimited = errors.New("rate limited")

	// ErrTimeout is returned when a sandbox request times out, usually due
	// to the sandbox itself having timed out (HTTP 502/503).
	ErrTimeout = errors.New("request timed out")

	// ErrTemplate is returned for template version / compatibility errors
	// surfaced by envd or the control plane.
	ErrTemplate = errors.New("template error")

	// ErrBuild is returned when a template build fails. It is the parent of
	// ErrFileUpload.
	ErrBuild = errors.New("build failed")

	// ErrFileUpload is returned when uploading build context files fails.
	// Wraps ErrBuild.
	ErrFileUpload = fmt.Errorf("file upload: %w", ErrBuild)

	// ErrVolume is returned for volume-specific failures.
	ErrVolume = errors.New("volume error")
)

// APIError represents a non-successful HTTP response from the Sandbox API or
// envd HTTP endpoints. It preserves both the raw response body and parsed
// error fields so callers can choose between user-facing messages and
// diagnostic data. APIError also wraps a sentinel error (see the Err* vars
// above) classified from HTTP status and response body, enabling discrimination
// via errors.Is / errors.As.
type APIError struct {
	// StatusCode is the HTTP response status code.
	StatusCode int
	// Body is the raw response body returned by the server.
	Body []byte

	// Reqid is the server request ID, when the response includes one.
	Reqid string
	// Code is the machine-readable error code parsed from a JSON body.
	Code string
	// Message is the human-readable error message parsed from a JSON body.
	Message string

	// RetryAfter is the Retry-After delay the server asked the caller to
	// wait before retrying. Populated for 429 / 503 responses that include
	// the header; zero otherwise.
	RetryAfter time.Duration

	// sentinel is the typed error this APIError unwraps to. Enables
	// errors.Is(err, ErrSandboxNotFound) style checks.
	sentinel error
}

// Error formats the API error with status, request ID, and the best available
// message body so it is useful in logs and user-facing CLI output.
func (e *APIError) Error() string {
	prefix := fmt.Sprintf("api error: status %d", e.StatusCode)
	if e.Reqid != "" {
		prefix += ", reqid: " + e.Reqid
	}
	if e.Message != "" {
		return prefix + ": " + e.Message
	}
	if len(e.Body) > 0 {
		return prefix + ", body: " + string(e.Body)
	}
	return prefix
}

// Unwrap returns the sentinel error this APIError was classified as, so
// errors.Is can walk the chain. Returns nil when the response could not be
// classified.
func (e *APIError) Unwrap() error {
	return e.sentinel
}

// resourceHint describes what kind of resource the caller was interacting with
// when the request failed. It refines 404 classification so FileNotFound and
// SandboxNotFound can be distinguished from a generic NotFound.
type resourceHint int

const (
	resourceUnknown resourceHint = iota
	resourceSandbox
	resourceFile
	resourceVolume
	resourceTemplate
	resourceBuild
	resourceFileUpload
)

// newAPIError constructs an APIError with a classified sentinel. Use
// newAPIErrorFor to attach a resource hint when the caller knows what was
// being fetched (this upgrades a bare 404 to SandboxNotFound / FileNotFound).
func newAPIError(resp *http.Response, body []byte) *APIError {
	return newAPIErrorFor(resp, body, resourceUnknown)
}

func newAPIErrorFor(resp *http.Response, body []byte, hint resourceHint) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		Body:       body,
		Reqid:      resp.Header.Get("X-Reqid"),
	}
	e.Code, e.Message = parseAPIErrorBody(body)
	e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	e.sentinel = classifySentinel(e.StatusCode, e.Code, e.Message, hint)
	return e
}

// parseRetryAfter decodes the Retry-After header per RFC 7231: either an
// integer number of seconds or an HTTP-date. Returns zero when the header is
// absent or malformed so callers can fall back to their own backoff strategy.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// classifySentinel maps an HTTP response to a sentinel error. Hints from the
// call site refine 404s to a specific resource sentinel.
func classifySentinel(status int, code, message string, hint resourceHint) error {
	// Message-level hints can override status (e.g. 400 with "invalid git
	// credentials" should be ErrGitAuth, not ErrInvalidArgument).
	lower := strings.ToLower(message + " " + code)
	switch {
	case strings.Contains(lower, "git") && (strings.Contains(lower, "auth") || strings.Contains(lower, "credential")):
		return ErrGitAuth
	case strings.Contains(lower, "git") && (strings.Contains(lower, "upstream") || strings.Contains(lower, "remote")):
		return ErrGitUpstream
	case strings.Contains(lower, "template") && strings.Contains(lower, "version"):
		return ErrTemplate
	}

	switch status {
	case http.StatusBadRequest:
		return ErrInvalidArgument
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuthentication
	case http.StatusNotFound:
		switch hint {
		case resourceSandbox:
			return ErrSandboxNotFound
		case resourceFile:
			return ErrFileNotFound
		default:
			return ErrNotFound
		}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return ErrTimeout
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		// 502/503 from envd routes usually means the sandbox has timed
		// out. Surface as ErrTimeout to match E2B semantics.
		return ErrTimeout
	case http.StatusInsufficientStorage:
		return ErrNotEnoughSpace
	}

	if hint == resourceVolume {
		return ErrVolume
	}
	if hint == resourceTemplate {
		return ErrTemplate
	}
	if hint == resourceFileUpload {
		return ErrFileUpload
	}
	if hint == resourceBuild {
		return ErrBuild
	}
	return nil
}

func parseAPIErrorBody(body []byte) (code, message string) {
	if len(body) == 0 {
		return "", ""
	}
	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		return parsed.Code, parsed.Message
	}
	return "", ""
}

// isNotFoundError reports whether err is any 404-class error, including
// connect-go NotFound codes from gRPC streams. Preferred: errors.Is(err, ErrNotFound).
func isNotFoundError(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	if connect.CodeOf(err) == connect.CodeNotFound {
		return true
	}
	return false
}
