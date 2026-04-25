package dockerfile

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestReadDockerignore(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\nnode_modules\n\n*.log\n   \n  *.tmp  \n"
	if err := os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte(content), 0o644); err != nil {
		t.Fatalf("write ignore: %v", err)
	}
	pats := ReadDockerignore(dir)
	want := []string{"node_modules", "*.log", "*.tmp"}
	if len(pats) != len(want) {
		t.Fatalf("patterns = %v, want %v", pats, want)
	}
	for i, p := range want {
		if pats[i] != p {
			t.Errorf("pattern[%d] = %q, want %q", i, pats[i], p)
		}
	}
}

func TestReadDockerignoreMissing(t *testing.T) {
	dir := t.TempDir()
	if pats := ReadDockerignore(dir); pats != nil {
		t.Errorf("missing .dockerignore should return nil, got %v", pats)
	}
}

func TestComputeFilesHashDeterministic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, err := ComputeFilesHash(".", "/app", dir, nil)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	h2, err := ComputeFilesHash(".", "/app", dir, nil)
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hash not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d", len(h1))
	}

	// Changing content changes the hash.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h3, _ := ComputeFilesHash(".", "/app", dir, nil)
	if h1 == h3 {
		t.Error("hash should change when content changes")
	}
}

func TestComputeFilesHashRespectsIgnore(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("k"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "drop.log"), []byte("d"), 0o644)

	h1, err := ComputeFilesHash(".", "/app", dir, []string{"*.log"})
	if err != nil {
		t.Fatal(err)
	}

	// Touch only the ignored file; hash should remain identical.
	if err := os.WriteFile(filepath.Join(dir, "drop.log"), []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, err := ComputeFilesHash(".", "/app", dir, []string{"*.log"})
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("ignored file changes should not affect hash")
	}
}

func TestComputeFilesHashPathEscape(t *testing.T) {
	dir := t.TempDir()
	_, err := ComputeFilesHash("../outside", "/dest", dir, nil)
	if err == nil {
		t.Fatal("expected error for path escaping context")
	}
}

func TestCollectAndUpload(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644)

	var received []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/gzip" {
			t.Errorf("Content-Type = %s, want application/gzip", ct)
		}
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("gzip reader: %v", err)
			return
		}
		tr := tar.NewReader(gr)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("tar read: %v", err)
				return
			}
			received = append(received, h.Name)
			_, _ = io.Copy(io.Discard, tr)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := CollectAndUpload(context.Background(), srv.URL, ".", dir, nil); err != nil {
		t.Fatalf("upload: %v", err)
	}
	sort.Strings(received)
	want := []string{"a.txt", "b.txt"}
	if len(received) != len(want) {
		t.Fatalf("received = %v, want %v", received, want)
	}
	for i := range want {
		if received[i] != want[i] {
			t.Errorf("received[%d] = %q, want %q", i, received[i], want[i])
		}
	}
}

func TestCollectAndUploadServerError(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain body before returning error, otherwise the pipe writer may
		// race with the server closing the connection.
		_, _ = io.Copy(io.Discard, r.Body)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()
	err := CollectAndUpload(context.Background(), srv.URL, ".", dir, nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
