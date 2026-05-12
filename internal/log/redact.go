package log

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// sensitiveHeaders lists header names whose values must never appear in
// logs verbatim. Comparison is case-insensitive (net/http already
// canonicalizes header keys, but env-driven test cases sometimes don't).
var sensitiveHeaders = map[string]struct{}{
	"x-api-key":           {},
	"authorization":       {},
	"cookie":              {},
	"set-cookie":          {},
	"proxy-authorization": {},
}

// sensitiveJSONFields lists JSON field names that should be replaced with
// "***" whenever they appear in a body dump. Matched case-insensitively.
var sensitiveJSONFields = map[string]struct{}{
	"apikey":      {},
	"api_key":     {},
	"password":    {},
	"secret":      {},
	"token":       {},
	"accesskey":   {},
	"access_key":  {},
	"credential":  {},
	"credentials": {},
}

// sensitiveQueryParams names URL query keys whose values double as access
// credentials. Signed file URLs in particular embed `signature` +
// `signature_expiration` and are valid until the expiration window passes
// — a debug log pasted into an issue would be a usable bearer token.
// Match is case-insensitive.
var sensitiveQueryParams = map[string]struct{}{
	"signature":            {},
	"signature_expiration": {},
	"token":                {},
	"access_token":         {},
	"accesstoken":          {},
	"api_key":              {},
	"apikey":               {},
	"authorization":        {},
	"x-amz-signature":      {},
	"x-amz-security-token": {},
	"x-goog-signature":     {},
}

// maxBodyLogBytes caps how much of a request/response body we log at
// LevelTrace. Anything beyond is truncated with a "(truncated …)" suffix
// so we still see the shape of large payloads without flooding the log.
const maxBodyLogBytes = 4096

// RedactURL returns a log-safe form of u. Sensitive query params (see
// sensitiveQueryParams) have their values replaced with "***"; the rest
// of the URL — scheme, host, path, non-sensitive params — passes through
// verbatim so the log line still tells you which endpoint was hit.
//
// userinfo (https://user:pass@host) is also redacted because pasting a
// debug log to an issue would otherwise leak embedded credentials.
//
// Inputs that fail to parse are returned as "<unparseable-url>" rather
// than echoed back, on the theory that a malformed URL is more likely a
// bug than something we want to surface verbatim.
func RedactURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<unparseable-url>"
	}
	if u.User != nil {
		u.User = url.User("***")
	}
	if q := u.RawQuery; q != "" {
		values, perr := url.ParseQuery(q)
		if perr != nil {
			// ParseQuery rejects inputs like `signature=SECRET%zz` (bad
			// percent-escape). Echoing RawQuery back would leak the
			// original secret verbatim, so we redact the whole query.
			// The path/host still tell the operator which endpoint was
			// hit; nothing useful is lost.
			u.RawQuery = "<redacted-unparseable-query>"
		} else {
			u.RawQuery = redactQueryValues(values).Encode()
		}
	}
	// Fragments can carry credentials too — OAuth implicit-grant callbacks
	// land tokens in `#access_token=...&token_type=...`. Net/http doesn't
	// send fragments on the wire, but configs and copy-pasted URLs do, so
	// we redact the same key set as the query string.
	if u.Fragment != "" || u.RawFragment != "" {
		// Prefer Fragment (already decoded) for parsing; fall back to
		// RawFragment when Fragment is empty (rare, but covered by
		// net/url's RawFragment-set-after-parse contract).
		src := u.Fragment
		if src == "" {
			src = u.RawFragment
		}
		values, perr := url.ParseQuery(src)
		var encoded string
		if perr != nil {
			encoded = "<redacted-unparseable-fragment>"
		} else {
			encoded = redactQueryValues(values).Encode()
		}
		// Set both fields so String()/EscapedFragment() produce the
		// redacted form regardless of which one it prefers.
		u.Fragment = encoded
		u.RawFragment = encoded
	}
	return u.String()
}

// redactQueryValues masks values for any key listed in
// sensitiveQueryParams. The input map is mutated and returned for fluent
// use; callers don't reuse it after this call.
func redactQueryValues(values url.Values) url.Values {
	for k, vs := range values {
		if _, sensitive := sensitiveQueryParams[strings.ToLower(k)]; sensitive {
			for i := range vs {
				vs[i] = "***"
			}
			values[k] = vs
		}
	}
	return values
}

// RedactHeaders returns a copy of h with sensitive values replaced by a
// masked form. The input is not mutated so callers can reuse it.
func RedactHeaders(h http.Header) http.Header {
	if len(h) == 0 {
		return h
	}
	out := make(http.Header, len(h))
	for k, vs := range h {
		if _, ok := sensitiveHeaders[strings.ToLower(k)]; ok {
			masked := make([]string, len(vs))
			for i, v := range vs {
				masked[i] = maskValue(v)
			}
			out[k] = masked
			continue
		}
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// RedactBody returns a log-safe representation of body. For JSON content
// it parses, masks known-sensitive fields, re-marshals, and truncates.
// For non-JSON it returns the raw bytes, truncated. content_type may be
// empty; in that case we attempt a best-effort JSON parse.
func RedactBody(body []byte, contentType string) string {
	if len(body) == 0 {
		return ""
	}

	if isJSONContentType(contentType) || looksLikeJSON(body) {
		if redacted, ok := redactJSON(body); ok {
			return truncate(redacted)
		}
	}
	return truncate(string(body))
}

// isJSONContentType returns true when ct (a Content-Type header value)
// describes a JSON payload. We accept the common variants because the
// SDK and intermediate proxies aren't always consistent about parameters.
func isJSONContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "+json")
}

// looksLikeJSON peeks at the first non-whitespace byte to decide whether
// to attempt JSON redaction even when the Content-Type header is missing.
func looksLikeJSON(b []byte) bool {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return false
	}
	return b[0] == '{' || b[0] == '['
}

// redactJSON walks the JSON tree and masks values for keys listed in
// sensitiveJSONFields. Returns (output, true) on success; (zero, false)
// when the input isn't valid JSON, so the caller can fall back to the
// raw body.
func redactJSON(body []byte) (string, bool) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return "", false
	}
	masked := maskJSONValue(v)
	out, err := json.Marshal(masked)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// maskJSONValue recursively descends into maps and slices, replacing
// sensitive scalar values with "***". Non-sensitive values pass through
// unchanged.
func maskJSONValue(v any) any {
	switch tv := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(tv))
		for k, val := range tv {
			if _, ok := sensitiveJSONFields[strings.ToLower(k)]; ok {
				out[k] = "***"
				continue
			}
			out[k] = maskJSONValue(val)
		}
		return out
	case []any:
		out := make([]any, len(tv))
		for i, val := range tv {
			out[i] = maskJSONValue(val)
		}
		return out
	default:
		return v
	}
}

// truncate clips s to maxBodyLogBytes and appends a marker showing how
// many bytes were dropped. Done in characters-not-bytes terms would risk
// breaking mid-rune; here we accept the rare possibility because the
// output is for human inspection, not machine consumption.
func truncate(s string) string {
	if len(s) <= maxBodyLogBytes {
		return s
	}
	dropped := len(s) - maxBodyLogBytes
	return s[:maxBodyLogBytes] + "...(truncated " + itoa(dropped) + " bytes)"
}

// itoa is a tiny replacement for strconv.Itoa so this file doesn't pull
// in strconv just for one truncation message.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// maskValue keeps a 4-char prefix and 4-char suffix, replacing the middle
// with asterisks. Short values collapse to all asterisks. This mirrors
// the user-visible MaskAPIKey but is duplicated here so the log package
// has no dependency on internal/sandbox.
func maskValue(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return strings.Repeat("*", len(v))
	}
	return v[:4] + strings.Repeat("*", 4) + v[len(v)-4:]
}
