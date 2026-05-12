package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
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

	contentOnce   sync.Once
	cachedContent *volumeapi.ClientWithResponses
	contentErr    error
}

// CreateVolume creates a new persistent volume.
func (c *Client) CreateVolume(ctx context.Context, name string) (*Volume, error) {
	var out apis.VolumeAndToken
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodPost, "/api/v1/sbx/volumes", map[string]string{"name": name}, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, newAPIErrorFor(resp, body, resourceVolume)
	}
	return volumeFromAPI(c, out), nil
}

// ConnectVolume returns a Volume handle for an existing volume.
func (c *Client) ConnectVolume(ctx context.Context, volumeID string) (*Volume, error) {
	return c.GetVolumeInfo(ctx, volumeID)
}

// GetVolumeInfo fetches volume metadata and content token.
func (c *Client) GetVolumeInfo(ctx context.Context, volumeID string) (*Volume, error) {
	var out apis.VolumeAndToken
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodGet, "/api/v1/sbx/volumes/"+url.PathEscape(volumeID), nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAPIErrorFor(resp, body, resourceVolume)
	}
	return volumeFromAPI(c, out), nil
}

// ListVolumes lists persistent volumes.
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	var out []VolumeInfo
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodGet, "/api/v1/sbx/volumes", nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAPIErrorFor(resp, body, resourceVolume)
	}
	return out, nil
}

// DestroyVolume destroys a volume. It returns false when the volume is missing.
func (c *Client) DestroyVolume(ctx context.Context, volumeID string) (bool, error) {
	resp, body, err := c.api.DoLegacyJSON(ctx, http.MethodDelete, "/api/v1/sbx/volumes/"+url.PathEscape(volumeID), nil, nil)
	if err != nil {
		return false, err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, newAPIErrorFor(resp, body, resourceVolume)
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
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
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
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
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
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
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
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
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
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
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
		return nil, newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
	}
	out := volumeEntryFromAPI(*resp.JSON201)
	return &out, nil
}

// ReadFileStream streams a volume file without buffering it in memory. The
// caller MUST close the returned reader; failing to do so leaks the HTTP
// connection. For small files prefer ReadFile or ReadFileText.
func (v *Volume) ReadFileStream(ctx context.Context, path string) (io.ReadCloser, error) {
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.GetVolumecontentVolumeIDFile(ctx, v.VolumeID, &volumeapi.GetVolumecontentVolumeIDFileParams{Path: path})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, newAPIErrorFor(resp, body, resourceFile)
	}
	return resp.Body, nil
}

// WriteFileFromReader uploads a volume file from r without buffering it in
// memory. The reader is consumed to EOF; the caller retains ownership. This
// avoids the []byte round-trip of WriteFile for large payloads.
func (v *Volume) WriteFileFromReader(ctx context.Context, path string, r io.Reader, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	client, err := v.contentClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.PutVolumecontentVolumeIDFileWithBody(ctx, v.VolumeID, writeFileParams(path, opts), "application/octet-stream", r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, newAPIErrorFor(resp, body, resourceFile)
	}
	var entry volumeapi.VolumeEntryStat
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, fmt.Errorf("decode volume entry: %w", err)
	}
	out := volumeEntryFromAPI(entry)
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
		return newAPIErrorFor(resp.HTTPResponse, resp.Body, resourceFile)
	}
	return nil
}

func volumeFromAPI(c *Client, volume apis.VolumeAndToken) *Volume {
	return &Volume{client: c, VolumeID: volume.VolumeID, Name: volume.Name, Token: volume.Token}
}

func (v *Volume) contentClient() (*volumeapi.ClientWithResponses, error) {
	v.contentOnce.Do(func() {
		v.cachedContent, v.contentErr = volumeapi.NewClientWithResponses(v.client.config.Endpoint,
			volumeapi.WithHTTPClient(v.client.config.HTTPClient),
			volumeapi.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
				if v.Token != "" {
					req.Header.Set("Authorization", "Bearer "+v.Token)
				}
				setReqidHeader(ctx, req)
				return nil
			}),
		)
	})
	return v.cachedContent, v.contentErr
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
