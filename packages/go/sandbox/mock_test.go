package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/filesystem"
)

// rewriteTransport redirects every outgoing request to target while preserving
// the request path and query so a httptest.Server can stand in for the
// dynamically generated envd host.
type rewriteTransport struct {
	target *url.URL
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = rt.target.Scheme
	req2.URL.Host = rt.target.Host
	req2.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req2)
}

// newTestClient wires Client to a test HTTP server, returning the configured
// client. The server handler decides per-request behavior.
func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(&Config{
		APIKey:     "test-key",
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

// newTestSandbox returns a Sandbox whose envd host is rewritten to srv via a
// custom RoundTripper. The returned sandbox has a non-nil access token so the
// signed file URL helpers populate signature query params.
func newTestSandbox(t *testing.T, srv *httptest.Server) *Sandbox {
	t.Helper()
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv.URL: %v", err)
	}
	httpClient := &http.Client{Transport: &rewriteTransport{target: target}}
	client := &Client{config: &Config{HTTPClient: httpClient}}
	domain := "example.test"
	token := "envd-token"
	sb := &Sandbox{
		sandboxID:       "sbx-123",
		templateID:      "tpl-1",
		domain:          &domain,
		envdAccessToken: &token,
		envdTokenLoaded: true,
		client:          client,
	}
	return sb
}

// ---------------------------------------------------------------------------
// TemplateAliasExists
// ---------------------------------------------------------------------------

func TestTemplateAliasExists(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
		want       bool
		wantErr    bool
	}{
		{
			name:       "owner returns true",
			statusCode: http.StatusOK,
			body:       `{"templateID":"tpl_123","public":false}`,
			want:       true,
		},
		{
			name:       "forbidden treated as exists",
			statusCode: http.StatusForbidden,
			body:       `{"code":403,"message":"forbidden"}`,
			want:       true,
		},
		{
			name:       "not found returns false",
			statusCode: http.StatusNotFound,
			body:       `{"code":404,"message":"missing"}`,
			want:       false,
		},
		{
			name:       "server error surfaces error",
			statusCode: http.StatusInternalServerError,
			body:       `{"code":500,"message":"boom"}`,
			wantErr:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if want := "/templates/aliases/my-alias"; r.URL.Path != want {
					t.Errorf("path = %s, want %s", r.URL.Path, want)
				}
				if got := r.Header.Get("X-API-Key"); got != "test-key" {
					t.Errorf("X-API-Key = %q, want test-key", got)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				_, _ = io.WriteString(w, tc.body)
			})
			c, _ := newTestClient(t, handler)

			got, err := c.TemplateAliasExists(context.Background(), "my-alias")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("exists = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetTemplateTags
// ---------------------------------------------------------------------------

func TestGetTemplateTags(t *testing.T) {
	t.Run("returns tags", func(t *testing.T) {
		buildID := "11111111-1111-1111-1111-111111111111"
		created := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		body := `[
			{"buildID":"` + buildID + `","tag":"latest","createdAt":"` + created.Format(time.RFC3339Nano) + `"},
			{"buildID":"` + buildID + `","tag":"prod","createdAt":"` + created.Format(time.RFC3339Nano) + `"}
		]`
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if want := "/templates/tpl_42/tags"; r.URL.Path != want {
				t.Errorf("path = %s, want %s", r.URL.Path, want)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		})
		c, _ := newTestClient(t, handler)

		tags, err := c.GetTemplateTags(context.Background(), "tpl_42")
		if err != nil {
			t.Fatalf("GetTemplateTags: %v", err)
		}
		if len(tags) != 2 {
			t.Fatalf("len(tags) = %d, want 2", len(tags))
		}
		if tags[0].Tag != "latest" || tags[1].Tag != "prod" {
			t.Errorf("unexpected tag names: %+v", tags)
		}
		if tags[0].BuildID != buildID {
			t.Errorf("BuildID = %q, want %q", tags[0].BuildID, buildID)
		}
		if !tags[0].CreatedAt.Equal(created) {
			t.Errorf("CreatedAt = %v, want %v", tags[0].CreatedAt, created)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[]`)
		})
		c, _ := newTestClient(t, handler)
		tags, err := c.GetTemplateTags(context.Background(), "tpl_empty")
		if err != nil {
			t.Fatalf("GetTemplateTags: %v", err)
		}
		if len(tags) != 0 {
			t.Errorf("len(tags) = %d, want 0", len(tags))
		}
	})

	t.Run("non-200 surfaces error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"code":502,"message":"bad gateway"}`)
		})
		c, _ := newTestClient(t, handler)
		if _, err := c.GetTemplateTags(context.Background(), "tpl_x"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// WriteStream
// ---------------------------------------------------------------------------

// writeStreamHandler simulates the envd /files upload endpoint and the
// connect-rpc Stat call invoked by WriteStream's follow-up GetInfo.
func writeStreamHandler(t *testing.T, capturedBody *[]byte, capturedHeaders http.Header) http.Handler {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("upload method = %s, want POST", r.Method)
		}
		// Capture select headers and the streamed multipart body so tests
		// can assert on them.
		for k := range r.Header {
			capturedHeaders.Set(k, r.Header.Get(k))
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("parse media type: %v", err)
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			t.Errorf("media type = %s, want multipart/*", mediaType)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part body: %v", err)
		}
		*capturedBody = data
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	mux.HandleFunc("/filesystem.Filesystem/Stat", func(w http.ResponseWriter, r *http.Request) {
		// Reply with a connect unary protobuf response describing the file.
		resp := &filesystem.StatResponse{
			Entry: &filesystem.EntryInfo{
				Name:        "hello.txt",
				Type:        filesystem.FileType_FILE_TYPE_FILE,
				Path:        "/work/hello.txt",
				Size:        11,
				Mode:        0o644,
				Permissions: "-rw-r--r--",
				Owner:       "user",
				Group:       "user",
			},
		}
		body, err := proto.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal stat response: %v", err)
		}
		w.Header().Set("Content-Type", "application/proto")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	return mux
}

func TestWriteStream(t *testing.T) {
	var captured []byte
	capturedHeaders := http.Header{}
	srv := httptest.NewServer(writeStreamHandler(t, &captured, capturedHeaders))
	t.Cleanup(srv.Close)

	sb := newTestSandbox(t, srv)
	fs := newFilesystem(sb)

	payload := "hello world"
	info, err := fs.WriteStream(context.Background(), "/work/hello.txt", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("WriteStream: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.Name != "hello.txt" || info.Path != "/work/hello.txt" {
		t.Errorf("unexpected info: %+v", info)
	}
	if info.Type != FileTypeFile {
		t.Errorf("Type = %v, want %v", info.Type, FileTypeFile)
	}
	if string(captured) != payload {
		t.Errorf("uploaded body = %q, want %q", captured, payload)
	}
	if got := capturedHeaders.Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want empty when WithGzip not set", got)
	}
}

func TestWriteStreamWithGzip(t *testing.T) {
	var captured []byte
	capturedHeaders := http.Header{}
	srv := httptest.NewServer(writeStreamHandler(t, &captured, capturedHeaders))
	t.Cleanup(srv.Close)

	sb := newTestSandbox(t, srv)
	fs := newFilesystem(sb)

	payload := strings.Repeat("abc", 100)
	info, err := fs.WriteStream(context.Background(), "/work/hello.txt", strings.NewReader(payload), WithGzip(true))
	if err != nil {
		t.Fatalf("WriteStream: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if got := capturedHeaders.Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", got)
	}
	// The first two bytes of a gzip stream are the magic number 0x1f 0x8b.
	if len(captured) < 2 || captured[0] != 0x1f || captured[1] != 0x8b {
		t.Errorf("body did not look gzipped: % x", captured[:min(8, len(captured))])
	}
}

func TestWriteStreamUploadError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		// Drain the body so the streaming uploader does not block.
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"code":500,"message":"upload failed"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	sb := newTestSandbox(t, srv)
	fs := newFilesystem(sb)
	_, err := fs.WriteStream(context.Background(), "/work/file.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected upload error, got nil")
	}
}

// ensure json package linked in case other files don't import it from tests
var _ = json.Marshal
