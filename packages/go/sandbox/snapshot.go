package sandbox

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SnapshotInfo identifies a persistent sandbox snapshot.
type SnapshotInfo struct {
	SnapshotID string   `json:"snapshotID"`
	Names      []string `json:"names,omitempty"`
}

// SnapshotListParams filters and paginates snapshot listings.
type SnapshotListParams struct {
	SandboxID *string
	NextToken *string
	Limit     *int32
}

// CreateSnapshot creates a persistent snapshot from sandboxID.
func (c *Client) CreateSnapshot(ctx context.Context, sandboxID string) (*SnapshotInfo, error) {
	var out SnapshotInfo
	path := "/api/v1/sbx/sandboxes/" + url.PathEscape(sandboxID) + "/snapshots"
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodPost, path, map[string]any{}, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, newAPIError(resp, body)
	}
	return &out, nil
}

// CreateSnapshot creates a persistent snapshot from this sandbox.
func (s *Sandbox) CreateSnapshot(ctx context.Context) (*SnapshotInfo, error) {
	return s.client.CreateSnapshot(ctx, s.sandboxID)
}

// ListSnapshots returns one page of snapshots.
func (c *Client) ListSnapshots(ctx context.Context, params *SnapshotListParams) ([]SnapshotInfo, *string, error) {
	path := "/api/v1/sbx/snapshots"
	if params != nil {
		q := url.Values{}
		if params.SandboxID != nil {
			q.Set("sandboxID", *params.SandboxID)
		}
		if params.NextToken != nil {
			q.Set("nextToken", *params.NextToken)
		}
		if params.Limit != nil {
			q.Set("limit", strconv.FormatInt(int64(*params.Limit), 10))
		}
		if encoded := q.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}
	var out []SnapshotInfo
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, newAPIError(resp, body)
	}
	next := resp.Header.Get("x-next-token")
	if next == "" {
		return out, nil, nil
	}
	return out, &next, nil
}

// ListSnapshots returns one page of snapshots created from this sandbox.
func (s *Sandbox) ListSnapshots(ctx context.Context, params *SnapshotListParams) ([]SnapshotInfo, *string, error) {
	if params == nil {
		params = &SnapshotListParams{}
	}
	params.SandboxID = &s.sandboxID
	return s.client.ListSnapshots(ctx, params)
}

// DeleteSnapshot deletes a persistent snapshot. It returns false if the snapshot
// was not found.
func (c *Client) DeleteSnapshot(ctx context.Context, snapshotID string) (bool, error) {
	path := "/api/v1/sbx/snapshots/" + url.PathEscape(snapshotID)
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return false, err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, newAPIError(resp, body)
	}
}
