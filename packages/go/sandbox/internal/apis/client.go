package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aonesuite/aone/packages/go/internal/aoneapi"
)

// ClientOption configures the generated control-plane client.
type ClientOption = aoneapi.ClientOption

// RequestEditorFn edits outgoing generated-client requests.
type RequestEditorFn = aoneapi.RequestEditorFn

// WithHTTPClient configures the HTTP client used by generated requests.
var WithHTTPClient = aoneapi.WithHTTPClient

// WithRequestEditorFn configures a request editor for generated requests.
var WithRequestEditorFn = aoneapi.WithRequestEditorFn

// ClientWithResponses wraps the generated client with shorter SDK-facing names.
type ClientWithResponses struct {
	client *aoneapi.ClientWithResponses
	raw    *aoneapi.Client
}

// NewClientWithResponses constructs a generated client wrapper for server.
func NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error) {
	raw, err := aoneapi.NewClient(server, opts...)
	if err != nil {
		return nil, err
	}
	client := &aoneapi.ClientWithResponses{ClientInterface: raw}
	return &ClientWithResponses{client: client, raw: raw}, nil
}

// DoJSON sends a JSON request through the wrapped generated client transport.
func (c *ClientWithResponses) DoJSON(ctx context.Context, method string, path string, body any, out any) (*http.Response, []byte, error) {
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, nil, err
		}
		reader = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.raw.Server, "/")+path, reader)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, editor := range c.raw.RequestEditors {
		if err := editor(ctx, req); err != nil {
			return nil, nil, err
		}
	}
	resp, err := c.raw.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp, data, err
		}
	}
	return resp, data, nil
}

// CreateSandboxJSONRequestBody aliases the generated sandbox create request.
type CreateSandboxJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxJSONRequestBody

// ConnectSandboxJSONRequestBody aliases the generated sandbox connect request.
type ConnectSandboxJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxJSONRequestBody

// UpdateSandboxTimeoutJSONRequestBody aliases the generated sandbox timeout request.
type UpdateSandboxTimeoutJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutJSONRequestBody

// ListSandboxesV2Params aliases the generated sandbox list query parameters.
type ListSandboxesV2Params = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListSandboxesParams

// GetSandboxLogsParams aliases the generated sandbox log query parameters.
type GetSandboxLogsParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxLogsParams

// GetSandboxMetricsParams aliases the generated sandbox metrics query parameters.
type GetSandboxMetricsParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxMetricsParams

// Sandbox aliases the generated sandbox response.
type Sandbox = aoneapi.HandlerSandboxResponse

// SandboxDetail aliases the generated sandbox detail response.
type SandboxDetail = aoneapi.HandlerSandboxDetailResponse

// ListedSandbox aliases the generated sandbox list item response.
type ListedSandbox = aoneapi.HandlerSandboxDetailResponse

// SandboxMetric aliases the generated sandbox metric response.
type SandboxMetric = aoneapi.HandlerSandboxMetricResponse

// SandboxLogs aliases the generated sandbox logs response.
type SandboxLogs = aoneapi.HandlerSandboxLogsResponse

// SandboxNetworkConfig aliases the generated sandbox network route config.
type SandboxNetworkConfig = aoneapi.HandlerSandboxNetworkRouteConfig

// ServiceSandboxLogEntry aliases one generated sandbox log entry.
type ServiceSandboxLogEntry = aoneapi.ServiceSandboxLogEntry

// EnvVars represents environment variables in API payloads.
type EnvVars = map[string]string

// SandboxMetadata represents sandbox metadata in API payloads.
type SandboxMetadata = map[string]string

// Mcp represents MCP configuration in API payloads.
type Mcp = map[string]interface{}

// SandboxState is a sandbox lifecycle state value.
type SandboxState = string

// SandboxVolumeMount describes a volume mount returned by the API.
type SandboxVolumeMount struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetSandboxesMetricsParams contains query parameters for batch metrics.
type GetSandboxesMetricsParams struct {
	SandboxIds []string `form:"sandboxIds,omitempty" json:"sandboxIds,omitempty"`
}

// SandboxesWithMetrics maps sandbox IDs to metrics.
type SandboxesWithMetrics struct {
	Sandboxes map[string]SandboxMetric `json:"sandboxes"`
}

// GetSnapshotsParams contains query parameters for listing snapshots.
type GetSnapshotsParams struct{}

// SnapshotInfo identifies a sandbox snapshot.
type SnapshotInfo struct {
	Names      []string `json:"names"`
	SnapshotID string   `json:"snapshotID"`
}

// Template aliases the generated template response.
type Template = aoneapi.HandlerTemplateResponse

// TemplateBuild aliases the generated template build response.
type TemplateBuild = aoneapi.HandlerTemplateBuildStatusResponse

// TemplateWithBuilds aliases the generated template-with-builds response.
type TemplateWithBuilds = aoneapi.HandlerTemplateResponse

// TemplateBuildInfo aliases the generated template build status response.
type TemplateBuildInfo = aoneapi.HandlerTemplateBuildStatusResponse

// TemplateBuildLogsResponse aliases the generated template build logs response.
type TemplateBuildLogsResponse = aoneapi.HandlerTemplateBuildLogsResponse

// TemplateRequestResponseV3 aliases the generated template request response.
type TemplateRequestResponseV3 = aoneapi.HandlerTemplateResponse

// TemplateBuildFileUpload describes an upload URL for template build files.
type TemplateBuildFileUpload struct {
	Present bool   `json:"present"`
	URL     string `json:"url"`
}

// TemplateAliasResponse identifies a template alias lookup result.
type TemplateAliasResponse struct {
	TemplateID string `json:"templateID"`
	Public     bool   `json:"public"`
}

// AssignedTemplateTags describes tags assigned to a template build.
type AssignedTemplateTags struct {
	BuildID string   `json:"buildID"`
	Tags    []string `json:"tags"`
}

// TemplateTagInfo describes one template tag assignment.
type TemplateTagInfo struct {
	BuildID   string    `json:"buildID"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"createdAt"`
}

// VolumeAndToken contains a volume plus its access token.
type VolumeAndToken struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}

// GeneralRegistry contains username/password registry credentials.
type GeneralRegistry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Type     string `json:"type"`
}

// AWSRegistry contains AWS ECR registry credentials.
type AWSRegistry struct {
	AwsAccessKeyID     string `json:"awsAccessKeyId"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey"`
	AwsRegion          string `json:"awsRegion"`
	Type               string `json:"type"`
}

// GCPRegistry contains Google registry service-account credentials.
type GCPRegistry struct {
	ServiceAccountJSON string `json:"serviceAccountJson"`
	Type               string `json:"type"`
}

// UpdateTemplateJSONRequestBody aliases the generated template update request.
type UpdateTemplateJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateJSONRequestBody

// CreateTemplateV3JSONRequestBody aliases the generated template create request.
type CreateTemplateV3JSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateJSONRequestBody

// GetTemplateBuildLogsParams aliases the generated template build log parameters.
type GetTemplateBuildLogsParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildLogsParams

// RefreshSandboxJSONRequestBody contains the requested sandbox timeout duration.
type RefreshSandboxJSONRequestBody struct {
	Duration *int `json:"duration,omitempty"`
}

// GetTemplatesParams aliases the generated template list query parameters.
type GetTemplatesParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListTemplatesParams

// GetTemplateParams contains template lookup pagination parameters.
type GetTemplateParams struct {
	NextToken *string `form:"nextToken,omitempty" json:"nextToken,omitempty"`
	Limit     *int32  `form:"limit,omitempty" json:"limit,omitempty"`
}

// GetTemplateBuildStatusParams contains template build status query parameters.
type GetTemplateBuildStatusParams struct {
	LogsOffset *int32    `form:"logsOffset,omitempty" json:"logsOffset,omitempty"`
	Limit      *int32    `form:"limit,omitempty" json:"limit,omitempty"`
	Level      *LogLevel `form:"level,omitempty" json:"level,omitempty"`
}

// LogLevel is a template build log severity filter.
type LogLevel string

// FromImageRegistry stores raw registry credential JSON.
type FromImageRegistry json.RawMessage

// UnmarshalJSON preserves registry credential JSON without interpreting it.
func (f *FromImageRegistry) UnmarshalJSON(data []byte) error {
	*f = append((*f)[:0], data...)
	return nil
}

// TemplateStep describes one template build step.
type TemplateStep struct {
	Args      *[]string `json:"args,omitempty"`
	FilesHash *string   `json:"filesHash,omitempty"`
	Force     *bool     `json:"force,omitempty"`
	Type      string    `json:"type"`
}

// StartTemplateBuildV2JSONRequestBody contains the template build start payload.
type StartTemplateBuildV2JSONRequestBody struct {
	Force             *bool              `json:"force,omitempty"`
	FromImage         *string            `json:"fromImage,omitempty"`
	FromImageRegistry *FromImageRegistry `json:"fromImageRegistry,omitempty"`
	FromTemplate      *string            `json:"fromTemplate,omitempty"`
	ReadyCmd          *string            `json:"readyCmd,omitempty"`
	StartCmd          *string            `json:"startCmd,omitempty"`
	Steps             *[]TemplateStep    `json:"steps,omitempty"`
}

// AssignTemplateTagsJSONRequestBody contains a template tag assignment request.
type AssignTemplateTagsJSONRequestBody struct {
	Tags   []string `json:"tags"`
	Target string   `json:"target"`
}

// DeleteTemplateTagsJSONRequestBody contains a template tag deletion request.
type DeleteTemplateTagsJSONRequestBody struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// GetV2SandboxesResponse aliases the generated sandbox list response.
type GetV2SandboxesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListSandboxesResponse

// PostSandboxesResponse aliases the generated sandbox create response.
type PostSandboxesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxResponse

// DeleteSandboxesSandboxIDResponse aliases the generated sandbox delete response.
type DeleteSandboxesSandboxIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteSandboxResponse

// GetSandboxesSandboxIDResponse aliases the generated sandbox get response.
type GetSandboxesSandboxIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxResponse

// PostSandboxesSandboxIDConnectResponse aliases the generated sandbox connect response.
type PostSandboxesSandboxIDConnectResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxResponse

// GetSandboxesSandboxIDLogsResponse aliases the generated sandbox logs response.
type GetSandboxesSandboxIDLogsResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxLogsResponse

// GetSandboxesSandboxIDMetricsResponse aliases the generated sandbox metrics response.
type GetSandboxesSandboxIDMetricsResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxMetricsResponse

// PostSandboxesSandboxIDTimeoutResponse aliases the generated sandbox timeout response.
type PostSandboxesSandboxIDTimeoutResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutResponse

// GetTemplatesResponse aliases the generated template list response.
type GetTemplatesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListTemplatesResponse

// PostV3TemplatesResponse aliases the generated template create response.
type PostV3TemplatesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateResponse

// GetDefaultTemplatesResponse aliases the generated default-template list response.
type GetDefaultTemplatesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListDefaultTemplatesResponse

// DeleteTemplatesTemplateIDResponse aliases the generated template delete response.
type DeleteTemplatesTemplateIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteTemplateResponse

// GetTemplatesTemplateIDResponse aliases the generated template get response.
type GetTemplatesTemplateIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateResponse

// PatchTemplatesTemplateIDResponse aliases the generated template update response.
type PatchTemplatesTemplateIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateResponse

// GetTemplatesTemplateIDBuildsBuildIDLogsResponse aliases the generated template build logs response.
type GetTemplatesTemplateIDBuildsBuildIDLogsResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildLogsResponse

// GetTemplatesTemplateIDBuildsBuildIDStatusResponse aliases the generated template build status response.
type GetTemplatesTemplateIDBuildsBuildIDStatusResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildStatusResponse

// GetV2SandboxesWithResponse calls the generated sandbox list operation.
func (c *ClientWithResponses) GetV2SandboxesWithResponse(ctx context.Context, params *ListSandboxesV2Params, reqEditors ...RequestEditorFn) (*GetV2SandboxesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListSandboxesWithResponse(ctx, params, reqEditors...)
}

// PostSandboxesWithBodyWithResponse calls the generated sandbox create operation with a raw body.
func (c *ClientWithResponses) PostSandboxesWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostSandboxesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxWithBodyWithResponse(ctx, contentType, body, reqEditors...)
}

// PostSandboxesWithResponse calls the generated sandbox create operation.
func (c *ClientWithResponses) PostSandboxesWithResponse(ctx context.Context, body CreateSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*PostSandboxesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxWithResponse(ctx, body, reqEditors...)
}

// DeleteSandboxesSandboxIDWithResponse calls the generated sandbox delete operation.
func (c *ClientWithResponses) DeleteSandboxesSandboxIDWithResponse(ctx context.Context, sandboxID string, reqEditors ...RequestEditorFn) (*DeleteSandboxesSandboxIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteSandboxWithResponse(ctx, sandboxID, reqEditors...)
}

// GetSandboxesSandboxIDWithResponse calls the generated sandbox get operation.
func (c *ClientWithResponses) GetSandboxesSandboxIDWithResponse(ctx context.Context, sandboxID string, reqEditors ...RequestEditorFn) (*GetSandboxesSandboxIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxWithResponse(ctx, sandboxID, reqEditors...)
}

// PostSandboxesSandboxIDConnectWithBodyWithResponse calls the generated sandbox connect operation with a raw body.
func (c *ClientWithResponses) PostSandboxesSandboxIDConnectWithBodyWithResponse(ctx context.Context, sandboxID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDConnectResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxWithBodyWithResponse(ctx, sandboxID, contentType, body, reqEditors...)
}

// PostSandboxesSandboxIDConnectWithResponse calls the generated sandbox connect operation.
func (c *ClientWithResponses) PostSandboxesSandboxIDConnectWithResponse(ctx context.Context, sandboxID string, body ConnectSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDConnectResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxWithResponse(ctx, sandboxID, body, reqEditors...)
}

// GetSandboxesSandboxIDLogsWithResponse calls the generated sandbox logs operation.
func (c *ClientWithResponses) GetSandboxesSandboxIDLogsWithResponse(ctx context.Context, sandboxID string, params *GetSandboxLogsParams, reqEditors ...RequestEditorFn) (*GetSandboxesSandboxIDLogsResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxLogsWithResponse(ctx, sandboxID, params, reqEditors...)
}

// GetSandboxesSandboxIDMetricsWithResponse calls the generated sandbox metrics operation.
func (c *ClientWithResponses) GetSandboxesSandboxIDMetricsWithResponse(ctx context.Context, sandboxID string, params *GetSandboxMetricsParams, reqEditors ...RequestEditorFn) (*GetSandboxesSandboxIDMetricsResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxMetricsWithResponse(ctx, sandboxID, params, reqEditors...)
}

// PostSandboxesSandboxIDTimeoutWithBodyWithResponse calls the generated sandbox timeout operation with a raw body.
func (c *ClientWithResponses) PostSandboxesSandboxIDTimeoutWithBodyWithResponse(ctx context.Context, sandboxID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDTimeoutResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutWithBodyWithResponse(ctx, sandboxID, contentType, body, reqEditors...)
}

// PostSandboxesSandboxIDTimeoutWithResponse calls the generated sandbox timeout operation.
func (c *ClientWithResponses) PostSandboxesSandboxIDTimeoutWithResponse(ctx context.Context, sandboxID string, body UpdateSandboxTimeoutJSONRequestBody, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDTimeoutResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutWithResponse(ctx, sandboxID, body, reqEditors...)
}

// GetTemplatesWithResponse calls the generated template list operation.
func (c *ClientWithResponses) GetTemplatesWithResponse(ctx context.Context, params *GetTemplatesParams, reqEditors ...RequestEditorFn) (*GetTemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListTemplatesWithResponse(ctx, params, reqEditors...)
}

// PostV3TemplatesWithBodyWithResponse calls the generated template create operation with a raw body.
func (c *ClientWithResponses) PostV3TemplatesWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostV3TemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateWithBodyWithResponse(ctx, contentType, body, reqEditors...)
}

// PostV3TemplatesWithResponse calls the generated template create operation.
func (c *ClientWithResponses) PostV3TemplatesWithResponse(ctx context.Context, body CreateTemplateV3JSONRequestBody, reqEditors ...RequestEditorFn) (*PostV3TemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateWithResponse(ctx, body, reqEditors...)
}

// GetDefaultTemplatesWithResponse calls the generated default-template list operation.
func (c *ClientWithResponses) GetDefaultTemplatesWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*GetDefaultTemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListDefaultTemplatesWithResponse(ctx, reqEditors...)
}

// DeleteTemplatesTemplateIDWithResponse calls the generated template delete operation.
func (c *ClientWithResponses) DeleteTemplatesTemplateIDWithResponse(ctx context.Context, templateID string, reqEditors ...RequestEditorFn) (*DeleteTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteTemplateWithResponse(ctx, templateID, reqEditors...)
}

// GetTemplatesTemplateIDWithResponse calls the generated template get operation.
func (c *ClientWithResponses) GetTemplatesTemplateIDWithResponse(ctx context.Context, templateID string, reqEditors ...RequestEditorFn) (*GetTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateWithResponse(ctx, templateID, reqEditors...)
}

// PatchTemplatesTemplateIDWithBodyWithResponse calls the generated template update operation with a raw body.
func (c *ClientWithResponses) PatchTemplatesTemplateIDWithBodyWithResponse(ctx context.Context, templateID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PatchTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateWithBodyWithResponse(ctx, templateID, contentType, body, reqEditors...)
}

// PatchTemplatesTemplateIDWithResponse calls the generated template update operation.
func (c *ClientWithResponses) PatchTemplatesTemplateIDWithResponse(ctx context.Context, templateID string, body UpdateTemplateJSONRequestBody, reqEditors ...RequestEditorFn) (*PatchTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateWithResponse(ctx, templateID, body, reqEditors...)
}

// GetTemplatesTemplateIDBuildsBuildIDLogsWithResponse calls the generated template build logs operation.
func (c *ClientWithResponses) GetTemplatesTemplateIDBuildsBuildIDLogsWithResponse(ctx context.Context, templateID string, buildID string, params *GetTemplateBuildLogsParams, reqEditors ...RequestEditorFn) (*GetTemplatesTemplateIDBuildsBuildIDLogsResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildLogsWithResponse(ctx, templateID, buildID, params, reqEditors...)
}

// GetTemplatesTemplateIDBuildsBuildIDStatusWithResponse calls the generated template build status operation.
func (c *ClientWithResponses) GetTemplatesTemplateIDBuildsBuildIDStatusWithResponse(ctx context.Context, templateID string, buildID string, reqEditors ...RequestEditorFn) (*GetTemplatesTemplateIDBuildsBuildIDStatusResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildStatusWithResponse(ctx, templateID, buildID, reqEditors...)
}
