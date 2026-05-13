package sandbox

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func makeResp(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}}
}

func TestClassifySentinel(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		msg    string
		hint   resourceHint
		want   error
	}{
		{"sandbox 404", http.StatusNotFound, "", "", resourceSandbox, ErrSandboxNotFound},
		{"file 404", http.StatusNotFound, "", "", resourceFile, ErrFileNotFound},
		{"generic 404", http.StatusNotFound, "", "", resourceUnknown, ErrNotFound},
		{"401 auth", http.StatusUnauthorized, "", "", resourceUnknown, ErrAuthentication},
		{"403 auth", http.StatusForbidden, "", "", resourceUnknown, ErrAuthentication},
		{"429 rate", http.StatusTooManyRequests, "", "", resourceUnknown, ErrRateLimited},
		{"502 timeout", http.StatusBadGateway, "", "", resourceUnknown, ErrTimeout},
		{"507 space", http.StatusInsufficientStorage, "", "", resourceUnknown, ErrNotEnoughSpace},
		{"400 invalid", http.StatusBadRequest, "", "", resourceUnknown, ErrInvalidArgument},
		{"git auth msg", http.StatusBadRequest, "", "invalid git credentials", resourceUnknown, ErrGitAuth},
		{"git upstream msg", http.StatusBadGateway, "", "git remote unreachable", resourceUnknown, ErrGitUpstream},
		{"template version", http.StatusBadRequest, "", "template version mismatch", resourceUnknown, ErrTemplate},
		{"build hint fallback", http.StatusInternalServerError, "", "", resourceBuild, ErrBuild},
		{"file upload hint fallback", http.StatusInternalServerError, "", "", resourceFileUpload, ErrFileUpload},
		{"file upload wraps build", http.StatusInternalServerError, "", "", resourceFileUpload, ErrBuild},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifySentinel(tc.status, tc.code, tc.msg, tc.hint)
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAPIErrorUnwrap(t *testing.T) {
	apiErr := newAPIErrorFor(makeResp(http.StatusNotFound), nil, resourceSandbox)
	if !errors.Is(apiErr, ErrSandboxNotFound) {
		t.Fatal("expected ErrSandboxNotFound")
	}
	if !errors.Is(apiErr, ErrNotFound) {
		t.Fatal("expected ErrNotFound via parent wrap")
	}

	var target *APIError
	if !errors.As(apiErr, &target) {
		t.Fatal("expected errors.As to find *APIError")
	}
	if target.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", target.StatusCode)
	}
}

func TestIsNotFoundError(t *testing.T) {
	apiErr := newAPIErrorFor(makeResp(http.StatusNotFound), nil, resourceFile)
	if !isNotFoundError(apiErr) {
		t.Fatal("isNotFoundError should match APIError with 404")
	}
	if isNotFoundError(errors.New("other")) {
		t.Fatal("isNotFoundError should not match random error")
	}
}

func TestAPIErrorRetryAfter(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
	resp.Header.Set("Retry-After", "5")
	err := newAPIErrorFor(resp, nil, resourceUnknown)
	if err.RetryAfter.Seconds() != 5 {
		t.Fatalf("retry-after %v, want 5s", err.RetryAfter)
	}

	resp2 := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
	err2 := newAPIErrorFor(resp2, nil, resourceUnknown)
	if err2.RetryAfter != 0 {
		t.Fatalf("expected zero retry-after when header absent, got %v", err2.RetryAfter)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	// Future HTTP-date should produce a positive duration.
	future := time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat)
	d := parseRetryAfter(future)
	if d <= 0 || d > 3*time.Hour {
		t.Errorf("future HTTP-date duration = %v, want in (0, 3h]", d)
	}

	// Past HTTP-date should yield zero.
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(past); got != 0 {
		t.Errorf("past HTTP-date duration = %v, want 0", got)
	}

	// Malformed string should yield zero.
	if got := parseRetryAfter("not-a-date"); got != 0 {
		t.Errorf("malformed Retry-After duration = %v, want 0", got)
	}

	// Empty string should yield zero.
	if got := parseRetryAfter(""); got != 0 {
		t.Errorf("empty Retry-After duration = %v, want 0", got)
	}
}

func TestParseAPIErrorBody(t *testing.T) {
	code, msg := parseAPIErrorBody([]byte(`{"code":"E1","message":"bad"}`))
	if code != "E1" || msg != "bad" {
		t.Errorf("code=%q msg=%q", code, msg)
	}
	// Malformed JSON: both empty.
	code, msg = parseAPIErrorBody([]byte("notjson"))
	if code != "" || msg != "" {
		t.Errorf("malformed body should yield empty, got %q / %q", code, msg)
	}
	// Empty body.
	code, msg = parseAPIErrorBody(nil)
	if code != "" || msg != "" {
		t.Errorf("nil body should yield empty, got %q / %q", code, msg)
	}
}
