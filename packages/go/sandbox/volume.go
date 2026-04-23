package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/volumeapi"
)

// VolumeFileType classifies entries inside a persistent volume.
type VolumeFileType string

const (
	VolumeFileTypeFile    VolumeFileType = "file"
	VolumeFileTypeDir     VolumeFileType = "directory"
	VolumeFileTypeSymlink VolumeFileType = "symlink"
	VolumeFileTypeUnknown VolumeFileType = "unknown"
)

// VolumeInfo is the public metadata for a persistent volume.
type VolumeInfo struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
}

// VolumeEntryStat is metadata for a file or directory inside a volume.
type VolumeEntryStat struct {
	Name   string         `json:"name"`
	Path   string         `json:"path"`
	Type   VolumeFileType `json:"type"`
	Size   int64          `json:"size"`
	Mode   uint32         `json:"mode"`
	UID    uint32         `json:"uid"`
	GID    uint32         `json:"gid"`
	Target *string        `json:"target,omitempty"`
	ATime  time.Time      `json:"atime"`
	MTime  time.Time      `json:"mtime"`
	CTime  time.Time      `json:"ctime"`
}

// VolumeWriteOptions configures volume writes, mkdir, and metadata updates.
type VolumeWriteOptions struct {
	UID   *uint32
	GID   *uint32
	Mode  *uint32
	Force *bool
}

// Volume represents a persistent volume and its content API token.
type Volume struct {
	client   *Client
	VolumeID string
	Name     string
	Token    string
}

// CreateVolume creates a new persistent volume.
func (c *Client) CreateVolume(ctx context.Context, name string) (*Volume, error) {
	resp, err := c.api.PostVolumesWithResponse(ctx, apis.PostVolumesJSONRequestBody{Name: name})
	if err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	return volumeFromAPI(c, *resp.JSON201), nil
}

// ConnectVolume returns a Volume handle for an existing volume.
func (c *Client) ConnectVolume(ctx context.Context, volumeID string) (*Volume, error) {
	return c.GetVolumeInfo(ctx, volumeID)
}

// GetVolumeInfo fetches volume metadata and content token.
func (c *Client) GetVolumeInfo(ctx context.Context, volumeID string) (*Volume, error) {
	resp, err := c.api.GetVolumesVolumeIDWithResponse(ctx, volumeID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	return volumeFromAPI(c, *resp.JSON200), nil
}

// ListVolumes lists persistent volumes.
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	resp, err := c.api.GetVolumesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := make([]VolumeInfo, len(*resp.JSON200))
	for i, volume := range *resp.JSON200 {
		out[i] = VolumeInfo{VolumeID: volume.VolumeID, Name: volume.Name}
	}
	return out, nil
}

// DestroyVolume destroys a volume. It returns false when the volume is missing.
func (c *Client) DestroyVolume(ctx context.Context, volumeID string) (bool, error) {
	resp, err := c.api.DeleteVolumesVolumeIDWithResponse(ctx, volumeID)
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

// List returns entries under path.
func (v *Volume) List(ctx context.Context, path string, depth *int32) ([]VolumeEntryStat, error) {
	params := &volumeapi.GetVolumecontentVolumeIDDirParams{Path: path}
	if depth != nil {
		depthValue := uint32(*depth)
		params.Depth = &depthValue
	}
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.GetVolumecontentVolumeIDDirWithResponse(ctx, v.VolumeID, params)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := make([]VolumeEntryStat, len(*resp.JSON200))
	for i, entry := range *resp.JSON200 {
		out[i] = volumeEntryFromAPI(entry)
	}
	return out, nil
}

// MakeDir creates a directory and returns its metadata.
func (v *Volume) MakeDir(ctx context.Context, path string, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.PostVolumecontentVolumeIDDirWithResponse(ctx, v.VolumeID, makeDirParams(path, opts))
	if err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := volumeEntryFromAPI(*resp.JSON201)
	return &out, nil
}

// GetInfo returns metadata for a volume path.
func (v *Volume) GetInfo(ctx context.Context, path string) (*VolumeEntryStat, error) {
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.GetVolumecontentVolumeIDPathWithResponse(ctx, v.VolumeID, &volumeapi.GetVolumecontentVolumeIDPathParams{Path: path})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := volumeEntryFromAPI(*resp.JSON200)
	return &out, nil
}

// Exists checks whether a volume path exists.
func (v *Volume) Exists(ctx context.Context, path string) (bool, error) {
	_, err := v.GetInfo(ctx, path)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdateMetadata updates uid, gid, or mode for a path.
func (v *Volume) UpdateMetadata(ctx context.Context, path string, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	if opts == nil {
		opts = &VolumeWriteOptions{}
	}
	body := volumeapi.PatchVolumecontentVolumeIDPathJSONRequestBody{
		UID:  opts.UID,
		GID:  opts.GID,
		Mode: opts.Mode,
	}
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.PatchVolumecontentVolumeIDPathWithResponse(ctx, v.VolumeID, &volumeapi.PatchVolumecontentVolumeIDPathParams{Path: path}, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := volumeEntryFromAPI(*resp.JSON200)
	return &out, nil
}

// ReadFile downloads a volume file into memory.
func (v *Volume) ReadFile(ctx context.Context, path string) ([]byte, error) {
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.GetVolumecontentVolumeIDFileWithResponse(ctx, v.VolumeID, &volumeapi.GetVolumecontentVolumeIDFileParams{Path: path})
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	return resp.Body, nil
}

// ReadFileText downloads a volume file as text.
func (v *Volume) ReadFileText(ctx context.Context, path string) (string, error) {
	data, err := v.ReadFile(ctx, path)
	return string(data), err
}

// WriteFile uploads a volume file.
func (v *Volume) WriteFile(ctx context.Context, path string, data []byte, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.PutVolumecontentVolumeIDFileWithBodyWithResponse(ctx, v.VolumeID, writeFileParams(path, opts), "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, newAPIError(resp.HTTPResponse, resp.Body)
	}
	out := volumeEntryFromAPI(*resp.JSON201)
	return &out, nil
}

// Remove deletes a file or directory from the volume.
func (v *Volume) Remove(ctx context.Context, path string) error {
	client, err := v.contentClient()
	if err != nil {
		return err
	}
	resp, err := client.DeleteVolumecontentVolumeIDPathWithResponse(ctx, v.VolumeID, &volumeapi.DeleteVolumecontentVolumeIDPathParams{Path: path})
	if err != nil {
		return err
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK && resp.HTTPResponse.StatusCode != http.StatusNoContent {
		return newAPIError(resp.HTTPResponse, resp.Body)
	}
	return nil
}

func volumeFromAPI(c *Client, volume apis.VolumeAndToken) *Volume {
	return &Volume{client: c, VolumeID: volume.VolumeID, Name: volume.Name, Token: volume.Token}
}

func (v *Volume) contentClient() (*volumeapi.ClientWithResponses, error) {
	return volumeapi.NewClientWithResponses(v.client.config.Endpoint,
		volumeapi.WithHTTPClient(v.client.config.HTTPClient),
		volumeapi.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			if v.Token != "" {
				req.Header.Set("Authorization", "Bearer "+v.Token)
			}
			setReqidHeader(ctx, req)
			return nil
		}),
	)
}

func makeDirParams(path string, opts *VolumeWriteOptions) *volumeapi.PostVolumecontentVolumeIDDirParams {
	params := &volumeapi.PostVolumecontentVolumeIDDirParams{Path: path}
	if opts != nil {
		params.UID = opts.UID
		params.GID = opts.GID
		params.Mode = opts.Mode
		params.Force = opts.Force
	}
	return params
}

func writeFileParams(path string, opts *VolumeWriteOptions) *volumeapi.PutVolumecontentVolumeIDFileParams {
	params := &volumeapi.PutVolumecontentVolumeIDFileParams{Path: path}
	if opts != nil {
		params.UID = opts.UID
		params.GID = opts.GID
		params.Mode = opts.Mode
		params.Force = opts.Force
	}
	return params
}

func volumeEntryFromAPI(entry volumeapi.VolumeEntryStat) VolumeEntryStat {
	return VolumeEntryStat{
		Name:   entry.Name,
		Path:   entry.Path,
		Type:   VolumeFileType(entry.Type),
		Size:   entry.Size,
		Mode:   entry.Mode,
		UID:    entry.UID,
		GID:    entry.GID,
		Target: entry.Target,
		ATime:  entry.Atime,
		MTime:  entry.Mtime,
		CTime:  entry.Ctime,
	}
}

func (v *Volume) String() string {
	return fmt.Sprintf("Volume(%s, %s)", v.VolumeID, v.Name)
}
