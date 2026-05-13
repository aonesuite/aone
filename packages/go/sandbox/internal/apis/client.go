package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/aoneapi"
)

type ClientOption = aoneapi.ClientOption
type RequestEditorFn = aoneapi.RequestEditorFn

var WithHTTPClient = aoneapi.WithHTTPClient
var WithRequestEditorFn = aoneapi.WithRequestEditorFn

type ClientWithResponses struct {
	client *aoneapi.ClientWithResponses
	raw    *aoneapi.Client
}

func NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error) {
	raw, err := aoneapi.NewClient(server, opts...)
	if err != nil {
		return nil, err
	}
	client := &aoneapi.ClientWithResponses{ClientInterface: raw}
	return &ClientWithResponses{client: client, raw: raw}, nil
}

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

// Aliases keep the hand-written SDK layer readable while the generated client
// follows operation IDs from the OpenAPI spec.
type CreateSandboxJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxJSONRequestBody
type ConnectSandboxJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxJSONRequestBody
type UpdateSandboxTimeoutJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutJSONRequestBody
type ListSandboxesV2Params = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListSandboxesParams
type GetSandboxLogsParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxLogsParams
type GetSandboxMetricsParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxMetricsParams

type Sandbox = aoneapi.HandlerSandboxResponse
type SandboxDetail = aoneapi.HandlerSandboxDetailResponse
type ListedSandbox = aoneapi.HandlerSandboxDetailResponse
type SandboxMetric = aoneapi.HandlerSandboxMetricResponse
type SandboxLogs = aoneapi.HandlerSandboxLogsResponse
type SandboxNetworkConfig = aoneapi.HandlerSandboxNetworkRouteConfig
type ServiceSandboxLogEntry = aoneapi.ServiceSandboxLogEntry
type EnvVars = map[string]string
type SandboxMetadata = map[string]string
type Mcp = map[string]interface{}
type SandboxState = string

type SandboxVolumeMount struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type GetSandboxesMetricsParams struct {
	SandboxIds []string `form:"sandboxIds,omitempty" json:"sandboxIds,omitempty"`
}

type SandboxesWithMetrics struct {
	Sandboxes map[string]SandboxMetric `json:"sandboxes"`
}

type GetSnapshotsParams struct{}

type SnapshotInfo struct {
	Names      []string `json:"names"`
	SnapshotID string   `json:"snapshotID"`
}

type Template = aoneapi.HandlerTemplateResponse
type TemplateBuild = aoneapi.HandlerTemplateBuildStatusResponse
type TemplateWithBuilds = aoneapi.HandlerTemplateResponse
type TemplateBuildInfo = aoneapi.HandlerTemplateBuildStatusResponse
type TemplateBuildLogsResponse = aoneapi.HandlerTemplateBuildLogsResponse
type TemplateRequestResponseV3 = aoneapi.HandlerTemplateResponse

type TemplateBuildFileUpload struct {
	Present bool   `json:"present"`
	URL     string `json:"url"`
}

type TemplateAliasResponse struct {
	TemplateID string `json:"templateID"`
	Public     bool   `json:"public"`
}

type AssignedTemplateTags struct {
	BuildID string   `json:"buildID"`
	Tags    []string `json:"tags"`
}

type TemplateTagInfo struct {
	BuildID   string    `json:"buildID"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"createdAt"`
}

type VolumeAndToken struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}

type GeneralRegistry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Type     string `json:"type"`
}

type AWSRegistry struct {
	AwsAccessKeyID     string `json:"awsAccessKeyId"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey"`
	AwsRegion          string `json:"awsRegion"`
	Type               string `json:"type"`
}

type GCPRegistry struct {
	ServiceAccountJSON string `json:"serviceAccountJson"`
	Type               string `json:"type"`
}

type UpdateTemplateJSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateJSONRequestBody
type CreateTemplateV3JSONRequestBody = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateJSONRequestBody
type GetTemplateBuildLogsParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildLogsParams

type RefreshSandboxJSONRequestBody struct {
	Duration *int `json:"duration,omitempty"`
}

type GetTemplatesParams = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListTemplatesParams

type GetTemplateParams struct {
	NextToken *string `form:"nextToken,omitempty" json:"nextToken,omitempty"`
	Limit     *int32  `form:"limit,omitempty" json:"limit,omitempty"`
}

type GetTemplateBuildStatusParams struct {
	LogsOffset *int32    `form:"logsOffset,omitempty" json:"logsOffset,omitempty"`
	Limit      *int32    `form:"limit,omitempty" json:"limit,omitempty"`
	Level      *LogLevel `form:"level,omitempty" json:"level,omitempty"`
}

type LogLevel string

type FromImageRegistry json.RawMessage

func (f *FromImageRegistry) UnmarshalJSON(data []byte) error {
	*f = append((*f)[:0], data...)
	return nil
}

type TemplateStep struct {
	Args      *[]string `json:"args,omitempty"`
	FilesHash *string   `json:"filesHash,omitempty"`
	Force     *bool     `json:"force,omitempty"`
	Type      string    `json:"type"`
}

type StartTemplateBuildV2JSONRequestBody struct {
	Force             *bool              `json:"force,omitempty"`
	FromImage         *string            `json:"fromImage,omitempty"`
	FromImageRegistry *FromImageRegistry `json:"fromImageRegistry,omitempty"`
	FromTemplate      *string            `json:"fromTemplate,omitempty"`
	ReadyCmd          *string            `json:"readyCmd,omitempty"`
	StartCmd          *string            `json:"startCmd,omitempty"`
	Steps             *[]TemplateStep    `json:"steps,omitempty"`
}

type AssignTemplateTagsJSONRequestBody struct {
	Tags   []string `json:"tags"`
	Target string   `json:"target"`
}

type DeleteTemplateTagsJSONRequestBody struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type GetV2SandboxesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListSandboxesResponse
type PostSandboxesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxResponse
type DeleteSandboxesSandboxIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteSandboxResponse
type GetSandboxesSandboxIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxResponse
type PostSandboxesSandboxIDConnectResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxResponse
type GetSandboxesSandboxIDLogsResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxLogsResponse
type GetSandboxesSandboxIDMetricsResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxMetricsResponse
type PostSandboxesSandboxIDTimeoutResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutResponse
type GetTemplatesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListTemplatesResponse
type PostV3TemplatesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateResponse
type GetDefaultTemplatesResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListDefaultTemplatesResponse
type DeleteTemplatesTemplateIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteTemplateResponse
type GetTemplatesTemplateIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateResponse
type PatchTemplatesTemplateIDResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateResponse
type GetTemplatesTemplateIDBuildsBuildIDLogsResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildLogsResponse
type GetTemplatesTemplateIDBuildsBuildIDStatusResponse = aoneapi.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildStatusResponse

func (c *ClientWithResponses) GetV2SandboxesWithResponse(ctx context.Context, params *ListSandboxesV2Params, reqEditors ...RequestEditorFn) (*GetV2SandboxesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListSandboxesWithResponse(ctx, params, reqEditors...)
}

func (c *ClientWithResponses) PostSandboxesWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostSandboxesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxWithBodyWithResponse(ctx, contentType, body, reqEditors...)
}

func (c *ClientWithResponses) PostSandboxesWithResponse(ctx context.Context, body CreateSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*PostSandboxesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateSandboxWithResponse(ctx, body, reqEditors...)
}

func (c *ClientWithResponses) DeleteSandboxesSandboxIDWithResponse(ctx context.Context, sandboxID string, reqEditors ...RequestEditorFn) (*DeleteSandboxesSandboxIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteSandboxWithResponse(ctx, sandboxID, reqEditors...)
}

func (c *ClientWithResponses) GetSandboxesSandboxIDWithResponse(ctx context.Context, sandboxID string, reqEditors ...RequestEditorFn) (*GetSandboxesSandboxIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxWithResponse(ctx, sandboxID, reqEditors...)
}

func (c *ClientWithResponses) PostSandboxesSandboxIDConnectWithBodyWithResponse(ctx context.Context, sandboxID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDConnectResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxWithBodyWithResponse(ctx, sandboxID, contentType, body, reqEditors...)
}

func (c *ClientWithResponses) PostSandboxesSandboxIDConnectWithResponse(ctx context.Context, sandboxID string, body ConnectSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDConnectResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleConnectSandboxWithResponse(ctx, sandboxID, body, reqEditors...)
}

func (c *ClientWithResponses) GetSandboxesSandboxIDLogsWithResponse(ctx context.Context, sandboxID string, params *GetSandboxLogsParams, reqEditors ...RequestEditorFn) (*GetSandboxesSandboxIDLogsResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxLogsWithResponse(ctx, sandboxID, params, reqEditors...)
}

func (c *ClientWithResponses) GetSandboxesSandboxIDMetricsWithResponse(ctx context.Context, sandboxID string, params *GetSandboxMetricsParams, reqEditors ...RequestEditorFn) (*GetSandboxesSandboxIDMetricsResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetSandboxMetricsWithResponse(ctx, sandboxID, params, reqEditors...)
}

func (c *ClientWithResponses) PostSandboxesSandboxIDTimeoutWithBodyWithResponse(ctx context.Context, sandboxID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDTimeoutResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutWithBodyWithResponse(ctx, sandboxID, contentType, body, reqEditors...)
}

func (c *ClientWithResponses) PostSandboxesSandboxIDTimeoutWithResponse(ctx context.Context, sandboxID string, body UpdateSandboxTimeoutJSONRequestBody, reqEditors ...RequestEditorFn) (*PostSandboxesSandboxIDTimeoutResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleSetSandboxTimeoutWithResponse(ctx, sandboxID, body, reqEditors...)
}

func (c *ClientWithResponses) GetTemplatesWithResponse(ctx context.Context, params *GetTemplatesParams, reqEditors ...RequestEditorFn) (*GetTemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListTemplatesWithResponse(ctx, params, reqEditors...)
}

func (c *ClientWithResponses) PostV3TemplatesWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PostV3TemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateWithBodyWithResponse(ctx, contentType, body, reqEditors...)
}

func (c *ClientWithResponses) PostV3TemplatesWithResponse(ctx context.Context, body CreateTemplateV3JSONRequestBody, reqEditors ...RequestEditorFn) (*PostV3TemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleCreateTemplateWithResponse(ctx, body, reqEditors...)
}

func (c *ClientWithResponses) GetDefaultTemplatesWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*GetDefaultTemplatesResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleListDefaultTemplatesWithResponse(ctx, reqEditors...)
}

func (c *ClientWithResponses) DeleteTemplatesTemplateIDWithResponse(ctx context.Context, templateID string, reqEditors ...RequestEditorFn) (*DeleteTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleDeleteTemplateWithResponse(ctx, templateID, reqEditors...)
}

func (c *ClientWithResponses) GetTemplatesTemplateIDWithResponse(ctx context.Context, templateID string, reqEditors ...RequestEditorFn) (*GetTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateWithResponse(ctx, templateID, reqEditors...)
}

func (c *ClientWithResponses) PatchTemplatesTemplateIDWithBodyWithResponse(ctx context.Context, templateID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*PatchTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateWithBodyWithResponse(ctx, templateID, contentType, body, reqEditors...)
}

func (c *ClientWithResponses) PatchTemplatesTemplateIDWithResponse(ctx context.Context, templateID string, body UpdateTemplateJSONRequestBody, reqEditors ...RequestEditorFn) (*PatchTemplatesTemplateIDResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleUpdateTemplateWithResponse(ctx, templateID, body, reqEditors...)
}

func (c *ClientWithResponses) GetTemplatesTemplateIDBuildsBuildIDLogsWithResponse(ctx context.Context, templateID string, buildID string, params *GetTemplateBuildLogsParams, reqEditors ...RequestEditorFn) (*GetTemplatesTemplateIDBuildsBuildIDLogsResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildLogsWithResponse(ctx, templateID, buildID, params, reqEditors...)
}

func (c *ClientWithResponses) GetTemplatesTemplateIDBuildsBuildIDStatusWithResponse(ctx context.Context, templateID string, buildID string, reqEditors ...RequestEditorFn) (*GetTemplatesTemplateIDBuildsBuildIDStatusResponse, error) {
	return c.client.GithubComAonesuiteInfraInternalProductsSandboxHandlerModuleGetTemplateBuildStatusWithResponse(ctx, templateID, buildID, reqEditors...)
}
