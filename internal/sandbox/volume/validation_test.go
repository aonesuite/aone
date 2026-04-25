package volume

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aonesuite/aone/internal/config"
)

// captureStderr redirects os.Stderr while fn runs, returning what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	_ = w.Close()
	<-done
	os.Stderr = orig
	return buf.String()
}

// isolateConfig redirects user-config home and clears credentials so command
// entry points cannot accidentally read the real ~/.config/aone.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv(config.EnvConfigHome, t.TempDir())
	t.Setenv(config.EnvAPIKey, "")
	t.Setenv(config.EnvEndpoint, "")
}

func TestCp_RequiresSourceAndDestination(t *testing.T) {
	isolateConfig(t)

	out := captureStderr(t, func() { Cp(CpInfo{}) })
	if !strings.Contains(out, "source and destination are required") {
		t.Fatalf("stderr = %q", out)
	}
}

func TestCp_RequiresExactlyOneRemote(t *testing.T) {
	isolateConfig(t)

	// Both local.
	out := captureStderr(t, func() {
		Cp(CpInfo{Source: "./a", Destination: "./b"})
	})
	if !strings.Contains(out, "exactly one") {
		t.Fatalf("local→local: stderr = %q", out)
	}

	// Both remote.
	out = captureStderr(t, func() {
		Cp(CpInfo{Source: "volume:/a", Destination: "volume:/b"})
	})
	if !strings.Contains(out, "exactly one") {
		t.Fatalf("remote→remote: stderr = %q", out)
	}
}

func TestCp_ProceedsToClientOnValidArgs(t *testing.T) {
	isolateConfig(t)
	out := captureStderr(t, func() {
		Cp(CpInfo{Source: "volume:/a", Destination: "./b"})
	})
	// Validation passes; we land on the API-key-not-configured error path.
	if strings.Contains(out, "exactly one") || strings.Contains(out, "source and destination") {
		t.Fatalf("validation should pass; stderr = %q", out)
	}
	if !strings.Contains(out, "API key not configured") {
		t.Fatalf("expected API-key error; stderr = %q", out)
	}
}

func TestCat_Validation(t *testing.T) {
	isolateConfig(t)

	out := captureStderr(t, func() { Cat(CatInfo{}) })
	if !strings.Contains(out, "volume ID is required") {
		t.Errorf("empty VolumeID: %q", out)
	}

	out = captureStderr(t, func() { Cat(CatInfo{VolumeID: "vol-1"}) })
	if !strings.Contains(out, "file path is required") {
		t.Errorf("empty Path: %q", out)
	}
}

func TestCreate_RequiresName(t *testing.T) {
	isolateConfig(t)
	out := captureStderr(t, func() { Create(CreateInfo{}) })
	if !strings.Contains(out, "volume name is required") {
		t.Fatalf("stderr = %q", out)
	}
}

func TestInfo_RequiresVolumeID(t *testing.T) {
	isolateConfig(t)
	out := captureStderr(t, func() { Info(InfoInfo{}) })
	if !strings.Contains(out, "volume ID is required") {
		t.Fatalf("stderr = %q", out)
	}
}

func TestLs_RequiresVolumeID(t *testing.T) {
	isolateConfig(t)
	out := captureStderr(t, func() { Ls(LsInfo{}) })
	if !strings.Contains(out, "volume ID is required") {
		t.Fatalf("stderr = %q", out)
	}
}

func TestMkdir_Validation(t *testing.T) {
	isolateConfig(t)

	out := captureStderr(t, func() { Mkdir(MkdirInfo{}) })
	if !strings.Contains(out, "volume ID is required") {
		t.Errorf("empty VolumeID: %q", out)
	}

	out = captureStderr(t, func() { Mkdir(MkdirInfo{VolumeID: "vol-1"}) })
	if !strings.Contains(out, "path is required") {
		t.Errorf("empty Path: %q", out)
	}
}

func TestRm_Validation(t *testing.T) {
	isolateConfig(t)

	out := captureStderr(t, func() { Rm(RmInfo{}) })
	if !strings.Contains(out, "volume ID is required") {
		t.Errorf("empty VolumeID: %q", out)
	}

	out = captureStderr(t, func() { Rm(RmInfo{VolumeID: "vol-1"}) })
	if !strings.Contains(out, "path is required") {
		t.Errorf("empty Path: %q", out)
	}
}
