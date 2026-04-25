package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTimestamp(t *testing.T) {
	if got := FormatTimestamp(time.Time{}); got != "-" {
		t.Errorf("zero → %q, want '-'", got)
	}
	tt := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	got := FormatTimestamp(tt)
	if got != "2026-04-25T10:00:00Z" {
		t.Errorf("FormatTimestamp(%v) = %q", tt, got)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:                       "0 MiB",
		1024 * 1024:             "1 MiB",
		512 * 1024 * 1024:       "512 MiB",
		2 * 1024 * 1024 * 1024:  "2048 MiB",
		3*1024*1024 + 512*1024:  "3.5 MiB", // .5 mantissa
	}
	for in, want := range cases {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatBytesHuman(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
		{2 * 1024 * 1024 * 1024, "2.0 GiB"},
	}
	for _, tc := range cases {
		if got := FormatBytesHuman(tc.in); got != tc.want {
			t.Errorf("FormatBytesHuman(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatOptionalString(t *testing.T) {
	if got := FormatOptionalString(nil); got != "-" {
		t.Errorf("nil → %q", got)
	}
	empty := ""
	if got := FormatOptionalString(&empty); got != "-" {
		t.Errorf("empty pointer → %q", got)
	}
	v := "hello"
	if got := FormatOptionalString(&v); got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestLogLevelBadge_KnownLevelsPadded(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		got := LogLevelBadge(lvl)
		// Strip ANSI escape codes via simple heuristic: badge body must contain the upper-cased level.
		if !strings.Contains(got, strings.ToUpper(lvl)) {
			t.Errorf("LogLevelBadge(%q) missing upper-cased level: %q", lvl, got)
		}
	}
}

func TestLogLevelBadge_UnknownLevel(t *testing.T) {
	got := LogLevelBadge("trace")
	if !strings.Contains(got, "TRACE") {
		t.Errorf("LogLevelBadge(\"trace\") = %q, want to contain TRACE", got)
	}
}
