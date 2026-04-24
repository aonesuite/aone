package sandbox

import (
	"errors"
	"net/http"
	"testing"
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
		{"volume hint fallback", http.StatusInternalServerError, "", "", resourceVolume, ErrVolume},
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
