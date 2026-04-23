package sandbox

import (
	"encoding/json"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
)

// APIError represents a non-successful HTTP response from the Sandbox API or
// envd HTTP endpoints. It preserves both the raw response body and parsed error
// fields so callers can choose between user-facing messages and diagnostic data.
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

func newAPIError(resp *http.Response, body []byte) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		Body:       body,
		Reqid:      resp.Header.Get("X-Reqid"),
	}
	e.Code, e.Message = parseAPIErrorBody(body)
	return e
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

func isNotFoundError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	if connect.CodeOf(err) == connect.CodeNotFound {
		return true
	}
	return false
}
