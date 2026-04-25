package volume

import (
	"testing"
)

func TestParseRemoteRef(t *testing.T) {
	cases := []struct {
		in       string
		wantRem  bool
		wantPath string
	}{
		{"volume:/etc/hosts", true, "/etc/hosts"},
		{"volume:relative/path", true, "relative/path"},
		{"volume:", true, ""},
		{"./local/file", false, "./local/file"},
		{"/abs/local", false, "/abs/local"},
		{"volume", false, "volume"}, // missing ':' → not remote
		{"", false, ""},
	}
	for _, tc := range cases {
		gotRem, gotPath := parseRemoteRef(tc.in)
		if gotRem != tc.wantRem || gotPath != tc.wantPath {
			t.Errorf("parseRemoteRef(%q) = (%v, %q), want (%v, %q)", tc.in, gotRem, gotPath, tc.wantRem, tc.wantPath)
		}
	}
}
