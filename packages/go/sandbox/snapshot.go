package sandbox

import (
	"context"
	"net/http"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
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
	resp, err := c.api.PostSandboxesSandboxIDSnapshotsWithResponse(ctx, sandboxID, apis.PostSandboxesSandboxIDSnapshotsJSONRequestBody{})
	if err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	return snapshotInfoFromAPI(*resp.JSON201), nil
}

// CreateSnapshot creates a persistent snapshot from this sandbox.
func (s *Sandbox) CreateSnapshot(ctx context.Context) (*SnapshotInfo, error) {
	return s.client.CreateSnapshot(ctx, s.sandboxID)
}

// ListSnapshots returns one page of snapshots.
func (c *Client) ListSnapshots(ctx context.Context, params *SnapshotListParams) ([]SnapshotInfo, *string, error) {
	resp, err := c.api.GetSnapshotsWithResponse(ctx, snapshotListParamsToAPI(params))
	if err != nil {
		return nil, nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := make([]SnapshotInfo, len(*resp.JSON200))
	for i, snapshot := range *resp.JSON200 {
		out[i] = *snapshotInfoFromAPI(snapshot)
	}
	next := resp.HTTPResponse.Header.Get("x-next-token")
	if next == "" {
		return out, nil, nil
	}
	return out, &next, nil
}

func snapshotListParamsToAPI(params *SnapshotListParams) *apis.GetSnapshotsParams {
	if params == nil {
		return nil
	}
	return &apis.GetSnapshotsParams{
		SandboxID: params.SandboxID,
		Limit:     params.Limit,
		NextToken: params.NextToken,
	}
}

func snapshotInfoFromAPI(snapshot apis.SnapshotInfo) *SnapshotInfo {
	return &SnapshotInfo{
		SnapshotID: snapshot.SnapshotID,
		Names:      snapshot.Names,
	}
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
	resp, err := c.api.DeleteTemplatesTemplateIDWithResponse(ctx, snapshotID)
	if err != nil {
		return false, err
	}
	switch resp.HTTPResponse.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, newAPIError(resp.HTTPResponse, resp.Body)
	}
}
