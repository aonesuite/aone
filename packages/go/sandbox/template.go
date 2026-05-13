package sandbox

import (
	"context"
	"fmt"
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

// GetTemplate returns template metadata for templateID.
func (c *Client) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	resp, err := c.api.GetTemplatesTemplateIDWithResponse(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceTemplate)
	}
	return templateDetailFromAPI(resp.JSON200), nil
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

// GetTemplateBuildStatus returns the current status for one template build.
func (c *Client) GetTemplateBuildStatus(ctx context.Context, templateID, buildID string) (*TemplateBuildInfo, error) {
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
		info, err := c.GetTemplateBuildStatus(ctx, templateID, buildID)
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
