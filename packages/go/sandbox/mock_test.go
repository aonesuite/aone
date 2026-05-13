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

func TestListTemplatesParamsAreMapped(t *testing.T) {
	var gotQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sbx/templates" {
			http.NotFound(w, r)
			return
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))

	apiKeyID := "key-1"
	name := "demo"
	buildStatus := "ready"
	public := "true"
	cursor := "cur-1"
	limit := int32(25)
	if _, err := c.ListTemplates(context.Background(), &ListTemplatesParams{
		APIKeyID:    &apiKeyID,
		Name:        &name,
		BuildStatus: &buildStatus,
		Public:      &public,
		Cursor:      &cursor,
		Limit:       &limit,
	}); err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}

	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse query %q: %v", gotQuery, err)
	}
	for key, want := range map[string]string{
		"api_key_id":   apiKeyID,
		"name":         name,
		"build_status": buildStatus,
		"public":       public,
		"cursor":       cursor,
		"limit":        "25",
	} {
		if got := values.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q; raw query %q", key, got, want, gotQuery)
		}
	}
}

func TestTemplateResponseFieldsAreMapped(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sbx/templates/tpl-1":
			_, _ = io.WriteString(w, `{
				"template_id":"tpl-1",
				"build_id":"build-1",
				"build_status":"ready",
				"cpu_count":2,
				"memory_mb":1024,
				"disk_size_mb":8192,
				"envd_version":"0.0.1",
				"public":true,
				"source":"user",
				"editable":true,
				"deletable":false,
				"aliases":["alias-1"],
				"names":["name-1"],
				"created_at":"2025-01-01T00:00:00Z",
				"updated_at":"2025-01-02T00:00:00Z"
			}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sbx/templates":
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{
				"template_id":"tpl-2",
				"build_id":"build-2",
				"public":true,
				"aliases":["alias-2"],
				"names":["name-2"]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))

	tmpl, err := c.GetTemplate(context.Background(), "tpl-1")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if tmpl.Names[0] != "name-1" || tmpl.Source != "user" || !tmpl.Editable || tmpl.Deletable {
		t.Fatalf("template fields not mapped: %+v", tmpl)
	}

	created, err := c.CreateTemplate(context.Background(), CreateTemplateParams{})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if created.Aliases[0] != "alias-2" || created.Names[0] != "name-2" {
		t.Fatalf("create response fields not mapped: %+v", created)
	}
}

func TestTemplateBuildStatusFieldsAreMapped(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/sbx/templates/tpl-1/builds/build-1/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"template_id":"tpl-1",
			"build_id":"build-1",
			"status":"ready",
			"envd_version":"0.0.2",
			"logs":["legacy line"],
			"log_entries":[{
				"level":"info",
				"message":"structured line",
				"source":"builder",
				"timestamp":"2025-01-01T00:00:00Z"
			}]
		}`)
	}))

	status, err := c.GetTemplateBuildStatus(context.Background(), "tpl-1", "build-1")
	if err != nil {
		t.Fatalf("GetTemplateBuildStatus: %v", err)
	}
	if status.EnvdVersion != "0.0.2" || len(status.Logs) != 1 || status.Logs[0] != "legacy line" {
		t.Fatalf("status fields not mapped: %+v", status)
	}
	if len(status.LogEntries) != 1 || status.LogEntries[0].Message != "structured line" || status.LogEntries[0].Step == nil || *status.LogEntries[0].Step != "builder" {
		t.Fatalf("log entries not mapped: %+v", status.LogEntries)
	}
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
