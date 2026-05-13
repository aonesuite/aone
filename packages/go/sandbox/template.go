package sandbox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ListTemplates returns templates visible to the authenticated caller. Pass nil
// params to use the API defaults.
func (c *Client) ListTemplates(ctx context.Context, params *ListTemplatesParams) ([]Template, error) {
	resp, err := c.api.GetTemplatesWithResponse(ctx, params.toAPI())
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceTemplate)
	}
	return templatesFromAPI(*resp.JSON200), nil
}

// CreateTemplate creates a template record and returns the initial build
// identifiers and aliases assigned by the API.
func (c *Client) CreateTemplate(ctx context.Context, body CreateTemplateParams) (*TemplateCreateResponse, error) {
	resp, err := c.api.PostV3TemplatesWithResponse(ctx, body.toAPI())
	if err != nil {
		return nil, err
	}
	if resp.JSON202 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceTemplate)
	}
	return templateCreateResponseFromAPI(resp.JSON202), nil
}

// GetTemplate returns template metadata and build history for templateID.
func (c *Client) GetTemplate(ctx context.Context, templateID string, params *GetTemplateParams) (*TemplateWithBuilds, error) {
	resp, err := c.api.GetTemplatesTemplateIDWithResponse(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceTemplate)
	}
	return templateWithBuildsFromAPI(resp.JSON200), nil
}

// DeleteTemplate deletes a template by ID. The API may accept either 200 or 204
// as a successful deletion response.
func (c *Client) DeleteTemplate(ctx context.Context, templateID string) error {
	resp, err := c.api.DeleteTemplatesTemplateIDWithResponse(ctx, templateID)
	if err != nil {
		return err
	}
	sc := resp.HTTPResponse.StatusCode
	if sc != 200 && sc != 204 {
		return newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceTemplate)
	}
	return nil
}

// UpdateTemplate mutates template properties such as public visibility.
func (c *Client) UpdateTemplate(ctx context.Context, templateID string, body UpdateTemplateParams) error {
	resp, err := c.api.PatchTemplatesTemplateIDWithResponse(ctx, templateID, body.toAPI())
	if err != nil {
		return err
	}
	if resp.HTTPResponse.StatusCode != 200 {
		return newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceTemplate)
	}
	return nil
}

// GetTemplateBuildStatus returns the current status for one template build,
// optionally including a bounded log snippet.
func (c *Client) GetTemplateBuildStatus(ctx context.Context, templateID, buildID string, params *GetBuildStatusParams) (*TemplateBuildInfo, error) {
	resp, err := c.api.GetTemplatesTemplateIDBuildsBuildIDStatusWithResponse(ctx, templateID, buildID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceBuild)
	}
	return templateBuildInfoFromAPI(resp.JSON200), nil
}

// GetTemplateBuildLogs returns structured logs for one template build.
func (c *Client) GetTemplateBuildLogs(ctx context.Context, templateID, buildID string, params *GetBuildLogsParams) (*TemplateBuildLogs, error) {
	resp, err := c.api.GetTemplatesTemplateIDBuildsBuildIDLogsWithResponse(ctx, templateID, buildID, params.toAPI())
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceBuild)
	}
	return templateBuildLogsFromAPI(resp.JSON200), nil
}

// StartTemplateBuild starts or restarts a build for an existing template/build
// pair using the supplied source image, source template, commands, and steps.
func (c *Client) StartTemplateBuild(ctx context.Context, templateID, buildID string, body StartTemplateBuildParams) error {
	apiBody, err := body.toAPI()
	if err != nil {
		return err
	}
	path := "/api/v1/sbx/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID)
	resp, respBody, err := c.api.DoJSON(ctx, http.MethodPost, path, apiBody, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return newAPIErrorFor(resp, respBody, resourceBuild)
	}
	return nil
}

// GetTemplateFiles returns upload metadata for a template file bundle hash.
func (c *Client) GetTemplateFiles(ctx context.Context, templateID, hash string) (*TemplateBuildFileUpload, error) {
	var out TemplateBuildFileUpload
	path := "/api/v1/sbx/templates/" + url.PathEscape(templateID) + "/files/" + url.PathEscape(hash)
	resp, body, err := c.api.DoJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, newAPIErrorFor(resp, body, resourceFileUpload)
	}
	return &out, nil
}

// GetTemplateByAlias resolves a template alias to template metadata.
func (c *Client) GetTemplateByAlias(ctx context.Context, alias string) (*TemplateAliasResponse, error) {
	var out TemplateAliasResponse
	path := "/api/v1/sbx/templates/aliases/" + url.PathEscape(alias)
	resp, body, err := c.api.DoJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAPIErrorFor(resp, body, resourceTemplate)
	}
	return &out, nil
}

// TemplateAliasExists reports whether a template alias exists.
// It returns true when the caller is the owner (200) or the alias is owned by
// someone else (403); false when the alias is not found (404). Any other
// status is returned as an error.
func (c *Client) TemplateAliasExists(ctx context.Context, alias string) (bool, error) {
	var out TemplateAliasResponse
	path := "/api/v1/sbx/templates/aliases/" + url.PathEscape(alias)
	resp, body, err := c.api.DoJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return false, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusForbidden:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, newAPIErrorFor(resp, body, resourceTemplate)
	}
}

// AssignTemplateTags assigns tags to the target described by body.
func (c *Client) AssignTemplateTags(ctx context.Context, body ManageTagsParams) (*AssignedTemplateTags, error) {
	var out AssignedTemplateTags
	resp, respBody, err := c.api.DoJSON(ctx, http.MethodPost, "/api/v1/sbx/templates/tags", body.toAPI(), &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, newAPIErrorFor(resp, respBody, resourceTemplate)
	}
	return &out, nil
}

// DeleteTemplateTags removes tags from a template target.
func (c *Client) DeleteTemplateTags(ctx context.Context, body DeleteTagsParams) error {
	resp, respBody, err := c.api.DoJSON(ctx, http.MethodDelete, "/api/v1/sbx/templates/tags", body.toAPI(), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return newAPIErrorFor(resp, respBody, resourceTemplate)
	}
	return nil
}

// GetTemplateTags returns all tags currently attached to templateID.
func (c *Client) GetTemplateTags(ctx context.Context, templateID string) ([]TemplateTagInfo, error) {
	var out []TemplateTagInfo
	path := "/api/v1/sbx/templates/" + url.PathEscape(templateID) + "/tags"
	resp, body, err := c.api.DoJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAPIErrorFor(resp, body, resourceTemplate)
	}
	return out, nil
}

// AssignTags is a shorter alias for AssignTemplateTags. Prefer
// AssignTemplateTags when writing new code.
func (c *Client) AssignTags(ctx context.Context, body ManageTagsParams) (*AssignedTemplateTags, error) {
	return c.AssignTemplateTags(ctx, body)
}

// RemoveTags is a shorter alias for DeleteTemplateTags. Prefer
// DeleteTemplateTags when writing new code.
func (c *Client) RemoveTags(ctx context.Context, body DeleteTagsParams) error {
	return c.DeleteTemplateTags(ctx, body)
}

// GetTags is a shorter alias for GetTemplateTags.
func (c *Client) GetTags(ctx context.Context, templateID string) ([]TemplateTagInfo, error) {
	return c.GetTemplateTags(ctx, templateID)
}

// WaitForBuild polls build status until the build becomes ready, fails, or the
// context is canceled. PollOption values control interval, backoff, progress
// callbacks, and optional build-log streaming via WithOnBuildLogs.
func (c *Client) WaitForBuild(ctx context.Context, templateID, buildID string, opts ...PollOption) (*TemplateBuildInfo, error) {
	o := defaultPollOpts(2 * time.Second)
	for _, fn := range opts {
		fn(o)
	}

	// Track the latest log timestamp we've already delivered so we only
	// forward new entries on subsequent ticks.
	var cursor *time.Time

	return pollLoop(ctx, o, func() (bool, *TemplateBuildInfo, error) {
		if o.onBuildLogs != nil {
			logs, lerr := c.GetTemplateBuildLogs(ctx, templateID, buildID, logsFromCursor(cursor))
			if lerr == nil && logs != nil && len(logs.Logs) > 0 {
				fresh := filterNewLogs(logs.Logs, cursor)
				if len(fresh) > 0 {
					o.onBuildLogs(fresh)
					ts := fresh[len(fresh)-1].Timestamp
					cursor = &ts
				}
			}
		}
		info, err := c.GetTemplateBuildStatus(ctx, templateID, buildID, nil)
		if err != nil {
			return false, nil, fmt.Errorf("get build status %s/%s: %w", templateID, buildID, err)
		}
		switch info.Status {
		case BuildStatusReady:
			return true, info, nil
		case BuildStatusError:
			return true, info, fmt.Errorf("build %s/%s failed: %w", templateID, buildID, ErrBuild)
		}
		return false, nil, nil
	})
}

// logsFromCursor builds a GetBuildLogsParams advancing past the supplied
// timestamp so already-seen entries are not fetched again.
func logsFromCursor(cursor *time.Time) *GetBuildLogsParams {
	if cursor == nil {
		return nil
	}
	ts := cursor.UnixMilli() + 1
	return &GetBuildLogsParams{Cursor: &ts}
}

// filterNewLogs returns entries strictly newer than cursor, preserving order.
func filterNewLogs(logs []BuildLogEntry, cursor *time.Time) []BuildLogEntry {
	if cursor == nil {
		return logs
	}
	out := logs[:0:0]
	for _, e := range logs {
		if e.Timestamp.After(*cursor) {
			out = append(out, e)
		}
	}
	return out
}
