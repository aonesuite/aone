package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process/processconnect"

	connect "connectrpc.com/connect"
)

const envdPort = 49983

// DefaultUser is the sandbox user used by file, command, and PTY operations
// when the caller does not specify a user override.
const DefaultUser = "user"

// DefaultTemplate is the template used when callers do not choose one.
const DefaultTemplate = "base"

// DefaultMCPTemplate is the template used for MCP gateway sandboxes.
const DefaultMCPTemplate = "mcp-gateway"

const mcpPort = 50005

// Sandbox represents one sandbox instance and lazily initializes helpers for
// files, commands, and PTY sessions. A Sandbox value is safe to reuse across
// operations; helper clients are created on first use and then cached.
type Sandbox struct {
	sandboxID          string
	envdSandboxID      string
	templateID         string
	clientID           string
	alias              *string
	domain             *string
	trafficAccessToken *string
	mcpToken           *string

	envdTokenMu     sync.RWMutex
	envdAccessToken *string
	envdTokenLoaded bool

	client *Client

	processRPCOnce sync.Once
	processRPC     processconnect.ProcessClient

	filesOnce sync.Once
	files     *Filesystem

	commandsOnce sync.Once
	commands     *Commands

	ptyOnce sync.Once
	pty     *Pty
}

func newSandbox(c *Client, s *apis.Sandbox) *Sandbox {
	sb := &Sandbox{
		sandboxID:          stringValue(s.SandboxID),
		envdSandboxID:      stringValue(s.EnvdSandboxID),
		templateID:         stringValue(s.TemplateID),
		clientID:           stringValue(s.ClientID),
		domain:             s.Domain,
		trafficAccessToken: s.TrafficAccessToken,
		client:             c,
	}
	if s.EnvdAccessToken != nil {
		sb.envdAccessToken = s.EnvdAccessToken
		sb.envdTokenLoaded = true
	}
	if sb.envdSandboxID == "" {
		sb.envdSandboxID = sb.sandboxID
	}
	return sb
}

// ID returns the sandbox identifier used by API and CLI operations.
func (s *Sandbox) ID() string { return s.sandboxID }

// TemplateID returns the template identifier used to create the sandbox.
func (s *Sandbox) TemplateID() string { return s.templateID }

// Alias returns the optional template alias associated with the sandbox.
func (s *Sandbox) Alias() *string { return s.alias }

// Domain returns the optional sandbox routing domain used for envd and exposed ports.
func (s *Sandbox) Domain() *string { return s.domain }

func (s *Sandbox) processClient() processconnect.ProcessClient {
	s.processRPCOnce.Do(func() {
		s.processRPC = processconnect.NewProcessClient(
			s.client.config.HTTPClient,
			s.envdURL(),
			connect.WithInterceptors(keepaliveInterceptor{}),
		)
	})
	return s.processRPC
}

// Create starts a sandbox from a template and returns as soon as the API accepts
// the sandbox. Use CreateAndWait when the caller needs to wait until the sandbox
// reaches StateRunning.
func (c *Client) Create(ctx context.Context, params CreateParams) (*Sandbox, error) {
	if params.TemplateID == "" {
		params.TemplateID = DefaultTemplate
	}
	body, err := params.toAPI()
	if err != nil {
		return nil, err
	}
	resp, err := c.api.PostSandboxesWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	sb := newSandbox(c, resp.JSON201)
	if !sb.envdTokenLoaded {
		if err := sb.refreshEnvdToken(ctx); err != nil {
			return nil, fmt.Errorf("create sandbox %s: %w", sb.sandboxID, err)
		}
	}
	return sb, nil
}

// Connect attaches the SDK to an existing sandbox by ID. The returned Sandbox
// can be used for commands, files, PTY, metrics, logs, and lifecycle operations.
func (c *Client) Connect(ctx context.Context, sandboxID string, params ConnectParams) (*Sandbox, error) {
	resp, err := c.api.PostSandboxesSandboxIDConnectWithResponse(ctx, sandboxID, params.toAPI())
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	sb := newSandbox(c, resp.JSON200)
	if !sb.envdTokenLoaded {
		if err := sb.refreshEnvdToken(ctx); err != nil {
			return nil, fmt.Errorf("connect sandbox %s: %w", sandboxID, err)
		}
	}
	return sb, nil
}

// List returns sandboxes visible to the authenticated caller. Pass nil params to
// use the API defaults.
func (c *Client) List(ctx context.Context, params *ListParams) ([]ListedSandbox, error) {
	items, _, err := c.ListPage(ctx, params)
	return items, err
}

// ListPage returns one sandbox page plus the next page token, if any.
func (c *Client) ListPage(ctx context.Context, params *ListParams) ([]ListedSandbox, *string, error) {
	resp, err := c.api.GetV2SandboxesWithResponse(ctx, params.toAPI())
	if err != nil {
		return nil, nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	items := listedSandboxesFromAPI(*resp.JSON200)
	next := resp.HTTPResponse.Header.Get("x-next-token")
	if next == "" {
		return items, nil, nil
	}
	return items, &next, nil
}

// Kill permanently terminates the sandbox.
func (s *Sandbox) Kill(ctx context.Context) error {
	resp, err := s.client.api.DeleteSandboxesSandboxIDWithResponse(ctx, s.sandboxID)
	if err != nil {
		return err
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK && resp.HTTPResponse.StatusCode != http.StatusNoContent {
		return newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	return nil
}

// SetTimeout updates the sandbox time-to-live. The duration must be at least one
// second and must fit in the API's int32 seconds field.
func (s *Sandbox) SetTimeout(ctx context.Context, timeout time.Duration) error {
	if timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second, got %v", timeout)
	}
	secs := timeout.Seconds()
	if secs > float64(math.MaxInt32) {
		return fmt.Errorf("timeout %v exceeds maximum allowed value", timeout)
	}
	timeoutSec := int32(secs)
	resp, err := s.client.api.PostSandboxesSandboxIDTimeoutWithResponse(ctx, s.sandboxID, apis.UpdateSandboxTimeoutJSONRequestBody{
		Timeout: &timeoutSec,
	})
	if err != nil {
		return err
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK && resp.HTTPResponse.StatusCode != http.StatusNoContent {
		return newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	return nil
}

// refreshEnvdToken fetches sandbox detail when create/connect did not include
// the temporary envd token.
func (s *Sandbox) refreshEnvdToken(ctx context.Context) error {
	resp, err := s.client.api.GetSandboxesSandboxIDWithResponse(ctx, s.sandboxID)
	if err != nil {
		return fmt.Errorf("get sandbox %s for envd token: %w", s.sandboxID, err)
	}
	if resp.JSON200 == nil {
		return newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	token := resp.JSON200.EnvdAccessToken
	if token == nil || *token == "" {
		return fmt.Errorf("get sandbox %s for envd token: response does not include envd access token", s.sandboxID)
	}
	s.envdTokenMu.Lock()
	s.envdAccessToken = token
	s.envdTokenLoaded = true
	s.envdTokenMu.Unlock()
	if resp.JSON200.EnvdSandboxID != nil && *resp.JSON200.EnvdSandboxID != "" {
		s.envdSandboxID = *resp.JSON200.EnvdSandboxID
	}
	if resp.JSON200.Domain != nil && *resp.JSON200.Domain != "" {
		s.domain = resp.JSON200.Domain
	}
	return nil
}

// GetInfo fetches the latest detailed state for the sandbox.
func (s *Sandbox) GetInfo(ctx context.Context) (*SandboxInfo, error) {
	resp, err := s.client.api.GetSandboxesSandboxIDWithResponse(ctx, s.sandboxID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	return sandboxInfoFromAPI(resp.JSON200), nil
}

// IsRunning checks the sandbox envd health endpoint. A healthy envd response is
// treated as running, while a gateway error is treated as not running.
func (s *Sandbox) IsRunning(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.envdURL()+"/health", nil)
	if err != nil {
		return false, err
	}
	setReqidHeader(ctx, req)
	resp, err := s.client.config.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return true, nil
	}
	if resp.StatusCode == http.StatusBadGateway {
		return false, nil
	}
	return false, newAPIError(resp, nil)
}

// GetMetrics returns resource-usage samples for this sandbox. Pass nil params
// to let the API choose its default time range.
func (s *Sandbox) GetMetrics(ctx context.Context, params *GetMetricsParams) ([]SandboxMetric, error) {
	resp, err := s.client.api.GetSandboxesSandboxIDMetricsWithResponse(ctx, s.sandboxID, params.toAPI())
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	return sandboxMetricsFromAPI(*resp.JSON200), nil
}

// GetLogs returns raw and structured logs for this sandbox.
func (s *Sandbox) GetLogs(ctx context.Context, params *GetLogsParams) (*SandboxLogs, error) {
	resp, err := s.client.api.GetSandboxesSandboxIDLogsWithResponse(ctx, s.sandboxID, params.toAPI())
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceSandbox)
	}
	return sandboxLogsFromAPI(resp.JSON200), nil
}

// Pause pauses the sandbox when the backend supports pausing for the current state.
func (s *Sandbox) Pause(ctx context.Context) error {
	path := "/api/v1/sbx/sandboxes/" + url.PathEscape(s.sandboxID) + "/pause"
	resp, body, err := s.client.api.DoJSON(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return newAPIErrorFor(resp, body, resourceSandbox)
	}
	return nil
}

// ResumeParams configures a Resume call. Both fields are optional; nil values
// instruct the server to use its defaults.
type ResumeParams struct {
	// Timeout is the new TTL (seconds) for the resumed sandbox. Nil keeps
	// the server-side default.
	Timeout *int32
}

// Resume resumes a paused sandbox. A 201 response indicates success.
func (c *Client) Resume(ctx context.Context, sandboxID string, params ResumeParams) error {
	body := map[string]any{}
	if params.Timeout != nil {
		body["timeout"] = params.Timeout
	}
	path := "/api/v1/sbx/sandboxes/" + url.PathEscape(sandboxID) + "/resume"
	resp, respBody, err := c.api.DoJSON(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return newAPIErrorFor(resp, respBody, resourceSandbox)
	}
	return nil
}

// Refresh extends the sandbox lifetime using the duration in params.
func (s *Sandbox) Refresh(ctx context.Context, params RefreshParams) error {
	path := "/api/v1/sbx/sandboxes/" + url.PathEscape(s.sandboxID) + "/refreshes"
	resp, body, err := s.client.api.DoJSON(ctx, http.MethodPost, path, params.toAPI(), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return newAPIErrorFor(resp, body, resourceSandbox)
	}
	return nil
}

// WaitForReady polls GetInfo until the sandbox reaches StateRunning or ctx is
// canceled. PollOption values control interval, backoff, and progress callbacks.
func (s *Sandbox) WaitForReady(ctx context.Context, opts ...PollOption) (*SandboxInfo, error) {
	o := defaultPollOpts(time.Second)
	for _, fn := range opts {
		fn(o)
	}

	return pollLoop(ctx, o, func() (bool, *SandboxInfo, error) {
		info, err := s.GetInfo(ctx)
		if err != nil {
			return false, nil, fmt.Errorf("get sandbox %s: %w", s.sandboxID, err)
		}
		if info.State == StateRunning {
			return true, info, nil
		}
		return false, nil, nil
	})
}

// CreateAndWait creates a sandbox and waits until it reaches StateRunning. It
// returns both the Sandbox handle and the final SandboxInfo snapshot.
func (c *Client) CreateAndWait(ctx context.Context, params CreateParams, opts ...PollOption) (*Sandbox, *SandboxInfo, error) {
	sb, err := c.Create(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("create sandbox: %w", err)
	}
	info, err := sb.WaitForReady(ctx, opts...)
	if err != nil {
		return nil, nil, err
	}
	return sb, info, nil
}

// GetSandboxesMetrics returns the latest metrics for multiple sandboxes.
func (c *Client) GetSandboxesMetrics(ctx context.Context, params *GetSandboxesMetricsParams) (*SandboxesWithMetrics, error) {
	path := "/api/v1/sbx/sandboxes/metrics"
	if params != nil && len(params.SandboxIds) > 0 {
		q := url.Values{}
		for _, id := range params.SandboxIds {
			q.Add("sandboxIds", id)
		}
		path += "?" + q.Encode()
	}
	var out SandboxesWithMetrics
	resp, body, err := c.api.DoJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAPIErrorFor(resp, body, resourceSandbox)
	}
	return &out, nil
}

// Files returns the filesystem helper bound to this sandbox.
func (s *Sandbox) Files() *Filesystem {
	s.filesOnce.Do(func() {
		s.files = newFilesystem(s)
	})
	return s.files
}

// Commands returns the process execution helper bound to this sandbox.
func (s *Sandbox) Commands() *Commands {
	s.commandsOnce.Do(func() {
		s.commands = newCommands(s, s.processClient())
	})
	return s.commands
}

// Pty returns the pseudo-terminal helper bound to this sandbox.
func (s *Sandbox) Pty() *Pty {
	s.ptyOnce.Do(func() {
		s.pty = newPty(s, s.processClient())
	})
	return s.pty
}

// GetHost builds the public host name for a sandbox port. It returns an empty
// string when the sandbox does not have a routing domain.
func (s *Sandbox) GetHost(port int) string {
	if s.domain == nil || *s.domain == "" {
		return ""
	}
	sandboxID := s.envdSandboxID
	if sandboxID == "" {
		sandboxID = s.sandboxID
	}
	return fmt.Sprintf("%d-%s.%s", port, sandboxID, *s.domain)
}

// GetMCPURL returns the MCP gateway URL for sandboxes created with MCP enabled.
func (s *Sandbox) GetMCPURL() string {
	host := s.GetHost(mcpPort)
	if host == "" {
		return ""
	}
	return "https://" + host + "/mcp"
}

// GetMCPToken returns the MCP gateway token when available. If this SDK did not
// create the gateway in the current process, it attempts to read the token file.
func (s *Sandbox) GetMCPToken(ctx context.Context) (string, error) {
	if s.mcpToken != nil {
		return *s.mcpToken, nil
	}
	token, err := s.Files().ReadText(ctx, "/etc/mcp-gateway/.token", WithUser("root"))
	if err != nil {
		return "", err
	}
	s.mcpToken = &token
	return token, nil
}

func (s *Sandbox) envdURL() string {
	return fmt.Sprintf("https://%s", s.GetHost(envdPort))
}

func envdBasicAuth(user string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"))
}

func (s *Sandbox) setEnvdAuth(req interface{ Header() http.Header }, user string) {
	req.Header().Set("Authorization", envdBasicAuth(user))
	s.envdTokenMu.RLock()
	tok := s.envdAccessToken
	s.envdTokenMu.RUnlock()
	if tok != nil && *tok != "" {
		req.Header().Set("X-Access-Token", *tok)
	}
}

const keepalivePingIntervalSec = "50"

const keepalivePingHeader = "Keepalive-Ping-Interval"

type keepaliveInterceptor struct{}

// WrapUnary leaves unary requests unchanged because the keepalive header matters
// only for long-lived streaming RPCs.
func (keepaliveInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return next
}

// WrapStreamingClient adds the keepalive interval header to outgoing streaming
// RPCs so proxies do not close idle command or PTY streams too aggressively.
func (keepaliveInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set(keepalivePingHeader, keepalivePingIntervalSec)
		return conn
	}
}

// WrapStreamingHandler leaves server-side handlers unchanged; the SDK uses this
// interceptor only as a streaming client.
func (keepaliveInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func setReqidHeader(ctx context.Context, req *http.Request) {
	if id, ok := ReqidFromContext(ctx); ok {
		req.Header.Set("X-Reqid", id)
	}
}

// FileURLOption customizes signed file upload and download URLs.
type FileURLOption func(*fileURLOpts)

type fileURLOpts struct {
	user                string
	signatureExpiration int
}

// WithFileUser sets the sandbox user encoded into generated file URLs.
func WithFileUser(user string) FileURLOption {
	return func(o *fileURLOpts) { o.user = user }
}

// WithSignatureExpiration sets the file URL signature lifetime in seconds. When
// omitted, file URLs use the SDK default expiration.
func WithSignatureExpiration(seconds int) FileURLOption {
	return func(o *fileURLOpts) { o.signatureExpiration = seconds }
}

func fileSignature(path, operation, username, accessToken string, expiration int) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%d", path, operation, username, accessToken, expiration)
	hash := sha256.Sum256([]byte(raw))
	return "v1_" + fmt.Sprintf("%x", hash)
}

// DownloadURL builds a signed URL for downloading a file from the sandbox.
func (s *Sandbox) DownloadURL(path string, opts ...FileURLOption) string {
	return s.fileURL(path, "read", opts...)
}

// UploadURL builds a signed URL for uploading a file to the sandbox.
func (s *Sandbox) UploadURL(path string, opts ...FileURLOption) string {
	return s.fileURL(path, "write", opts...)
}

func (s *Sandbox) fileURL(path, operation string, opts ...FileURLOption) string {
	o := &fileURLOpts{user: DefaultUser}
	for _, fn := range opts {
		fn(o)
	}

	q := url.Values{}
	q.Set("path", path)
	q.Set("username", o.user)

	s.envdTokenMu.RLock()
	tok := s.envdAccessToken
	s.envdTokenMu.RUnlock()
	if tok != nil && *tok != "" {
		exp := o.signatureExpiration
		if exp == 0 {
			exp = 300
		}
		sig := fileSignature(path, operation, o.user, *tok, exp)
		q.Set("signature", sig)
		q.Set("signature_expiration", strconv.Itoa(exp))
	}

	return s.envdURL() + "/files?" + q.Encode()
}

func (s *Sandbox) batchUploadURL(user string) string {
	q := url.Values{}
	q.Set("username", user)
	return s.envdURL() + "/files?" + q.Encode()
}
