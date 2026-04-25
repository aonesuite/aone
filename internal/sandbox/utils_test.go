package sandbox

import (
	"sort"
	"strings"
	"testing"

	sdkSandbox "github.com/aonesuite/aone/packages/go/sandbox"
)

func TestParseStates(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"running", []string{"running"}},
		{"running,paused", []string{"running", "paused"}},
		{" running , paused ", []string{"running", "paused"}},
		{",,running,,", []string{"running"}},
		{"", []string{}},
	}
	for _, tc := range cases {
		got := ParseStates(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("ParseStates(%q) len = %d, want %d", tc.in, len(got), len(tc.want))
			continue
		}
		for i, s := range got {
			if string(s) != tc.want[i] {
				t.Errorf("ParseStates(%q)[%d] = %v, want %v", tc.in, i, s, tc.want[i])
			}
		}
	}
}

func TestParseMetadata_QueryFormat(t *testing.T) {
	cases := map[string]string{
		"":                       "",
		"k=v":                    "k=v",
		"a=1,b=2":                "a=1&b=2",
		" a = 1 , b = 2 ":        "a=1&b=2",
		"a=1,,b=2":               "a=1&b=2",
		"a=1,malformed,b=2":      "a=1&b=2", // missing '=' is dropped
		"a=,b=2":                 "b=2",     // empty value dropped
		"=v,b=2":                 "b=2",     // empty key dropped
	}
	for in, want := range cases {
		if got := ParseMetadata(in); got != want {
			t.Errorf("ParseMetadata(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseMetadataMap(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]string
	}{
		{"", map[string]string{}},
		{"a=1,b=2", map[string]string{"a": "1", "b": "2"}},
		{" a = 1 , b = 2 ", map[string]string{"a": "1", "b": "2"}},
		{"a=,b=2", map[string]string{"a": "", "b": "2"}}, // empty value preserved
		{"=v,b=2", map[string]string{"b": "2"}},          // empty key dropped
		{"a=1,malformed,b=2", map[string]string{"a": "1", "b": "2"}},
	}
	for _, tc := range cases {
		got := ParseMetadataMap(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("ParseMetadataMap(%q) len = %d (%v), want %d (%v)", tc.in, len(got), got, len(tc.want), tc.want)
			continue
		}
		for k, v := range tc.want {
			if g, ok := got[k]; !ok || g != v {
				t.Errorf("ParseMetadataMap(%q)[%q] = %q (ok=%v), want %q", tc.in, k, g, ok, v)
			}
		}
	}
}

func TestIsLogLevelIncluded(t *testing.T) {
	cases := []struct {
		entry, min string
		want       bool
	}{
		{"debug", "info", false},
		{"info", "info", true},
		{"warn", "info", true},
		{"error", "info", true},
		{"info", "warn", false},
		{"warn", "warn", true},
		{"INFO", "info", true},   // case-insensitive
		{"info", "INFO", true},
		{"info", "", true},        // empty min → include all
		{"unknown", "info", true}, // unknown level → include
	}
	for _, tc := range cases {
		got := IsLogLevelIncluded(tc.entry, tc.min)
		if got != tc.want {
			t.Errorf("IsLogLevelIncluded(%q, %q) = %v, want %v", tc.entry, tc.min, got, tc.want)
		}
	}
}

func TestMatchesLoggerPrefix(t *testing.T) {
	cases := []struct {
		logger   string
		prefixes []string
		want     bool
	}{
		{"FooSvc", []string{"Foo"}, true},
		{"FooSvc", []string{"Bar", "Foo"}, true},
		{"BazSvc", []string{"Foo"}, false},
		{"FooSvc", nil, false},
		{"", []string{"Foo"}, false},
	}
	for _, tc := range cases {
		got := MatchesLoggerPrefix(tc.logger, tc.prefixes)
		if got != tc.want {
			t.Errorf("MatchesLoggerPrefix(%q, %v) = %v, want %v", tc.logger, tc.prefixes, got, tc.want)
		}
	}
}

func TestStripInternalFields(t *testing.T) {
	in := map[string]string{
		"traceID":    "abc",
		"sandboxID":  "sb-1",
		"user":       "alice",
		"requestID":  "r-1",
		"teamID":     "t-1",
		"source":     "envd",
		"service":    "api",
		"envID":      "e-1",
		"instanceID": "i-1",
	}
	got := StripInternalFields(in)
	if _, ok := got["user"]; !ok {
		t.Errorf("user field should be retained")
	}
	if _, ok := got["requestID"]; !ok {
		t.Errorf("requestID field should be retained")
	}
	for _, internal := range []string{"traceID", "sandboxID", "teamID", "source", "service", "envID", "instanceID"} {
		if _, ok := got[internal]; ok {
			t.Errorf("internal field %q should be stripped", internal)
		}
	}

	if got := StripInternalFields(nil); got != nil {
		t.Errorf("nil input should return nil; got %v", got)
	}

	allInternal := map[string]string{"traceID": "x", "envID": "y"}
	if got := StripInternalFields(allInternal); got != nil {
		t.Errorf("all-internal input should return nil; got %v", got)
	}
}

func TestCleanLoggerName(t *testing.T) {
	cases := map[string]string{
		"FooSvc":   "Foo",
		"BarSvc":   "Bar",
		"NoSuffix": "NoSuffix",
		"":         "",
		"Svc":      "",
	}
	for in, want := range cases {
		if got := CleanLoggerName(in); got != want {
			t.Errorf("CleanLoggerName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLoggers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ", []string{"a", "b"}},
		{",,", nil},
	}
	for _, tc := range cases {
		got := ParseLoggers(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("ParseLoggers(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i, s := range got {
			if s != tc.want[i] {
				t.Errorf("ParseLoggers(%q)[%d] = %q, want %q", tc.in, i, s, tc.want[i])
			}
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"abc":             "***",
		"abcdefgh":        "********",         // <=8 → all stars
		"abcdefghi":       "abcd****fghi",     // 9 chars: prefix abcd, suffix fghi
		"sk-1234567890ab": "sk-1****90ab",
	}
	for in, want := range cases {
		if got := MaskAPIKey(in); got != want {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// Confirm the SDK SandboxState alias roundtrips through ParseStates.
func TestParseStates_StateType(t *testing.T) {
	got := ParseStates("running,paused")
	want := []sdkSandbox.SandboxState{"running", "paused"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ParseStates state-type mismatch: got %v want %v", got, want)
	}
}

func TestFormatMetadata(t *testing.T) {
	if got := FormatMetadata(nil); got != "-" {
		t.Errorf("nil → %q, want '-'", got)
	}
	if got := FormatMetadata(map[string]string{}); got != "-" {
		t.Errorf("empty → %q, want '-'", got)
	}
	got := FormatMetadata(map[string]string{"a": "1", "b": "2"})
	// Map iteration order is randomized; sort the comma-separated pairs and
	// compare against the canonical form.
	parts := strings.Split(got, ", ")
	sort.Strings(parts)
	want := []string{"a=1", "b=2"}
	if len(parts) != len(want) || parts[0] != want[0] || parts[1] != want[1] {
		t.Errorf("FormatMetadata mismatch: got %v, want %v", parts, want)
	}
}
