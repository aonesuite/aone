package instance

import (
	"errors"
	"testing"
)

func TestParseEnvPairs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want map[string]string
	}{
		{
			name: "basic",
			in:   []string{"FOO=bar", "BAZ=qux"},
			want: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name: "empty value preserved",
			in:   []string{"FOO="},
			want: map[string]string{"FOO": ""},
		},
		{
			name: "value contains equals",
			in:   []string{"URL=https://x.test/path?a=b"},
			want: map[string]string{"URL": "https://x.test/path?a=b"},
		},
		{
			name: "missing equals dropped",
			in:   []string{"FOO", "BAR=baz"},
			want: map[string]string{"BAR": "baz"},
		},
		{
			name: "empty key dropped",
			in:   []string{"=value", "OK=1"},
			want: map[string]string{"OK": "1"},
		},
		{
			name: "duplicate key keeps last",
			in:   []string{"K=1", "K=2"},
			want: map[string]string{"K": "2"},
		},
		{
			name: "empty input returns empty map",
			in:   nil,
			want: map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseEnvPairs(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if g, ok := got[k]; !ok || g != v {
					t.Errorf("got[%q] = %q (ok=%v), want %q", k, g, ok, v)
				}
			}
		})
	}
}

func TestDetectResize(t *testing.T) {
	prev := terminalSize{width: 80, height: 24}

	t.Run("error returns previous unchanged", func(t *testing.T) {
		got, changed := detectResize(prev, 100, 30, errors.New("oops"))
		if changed {
			t.Fatalf("changed=true on error")
		}
		if got != prev {
			t.Fatalf("got %+v, want %+v", got, prev)
		}
	})

	t.Run("same size returns previous unchanged", func(t *testing.T) {
		got, changed := detectResize(prev, 80, 24, nil)
		if changed {
			t.Fatalf("changed=true for same size")
		}
		if got != prev {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("different size returns new size and true", func(t *testing.T) {
		got, changed := detectResize(prev, 120, 40, nil)
		if !changed {
			t.Fatalf("changed=false on resize")
		}
		want := terminalSize{width: 120, height: 40}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
}
