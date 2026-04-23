package sandbox

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/filesystem"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/filesystem/filesystemconnect"
)

// FileType classifies a filesystem entry returned by envd.
type FileType string

// Known filesystem entry types returned by the SDK.
const (
	// FileTypeFile identifies a regular file.
	FileTypeFile FileType = "file"
	// FileTypeDirectory identifies a directory.
	FileTypeDirectory FileType = "dir"
	// FileTypeUnknown is used when envd returns an entry type the SDK does not recognize.
	FileTypeUnknown FileType = "unknown"
)

// EntryInfo describes a file, directory, or symlink inside a sandbox. It mirrors
// the metadata returned by the envd filesystem service.
type EntryInfo struct {
	// Name is the base name of the entry.
	Name string
	// Type indicates whether the entry is a regular file, directory, or unknown.
	Type FileType
	// Path is the full sandbox path reported by envd.
	Path string
	// Size is the entry size in bytes.
	Size int64
	// Mode is the numeric POSIX mode for the entry.
	Mode uint32
	// Permissions is the human-readable permission string, such as "-rw-r--r--".
	Permissions string
	// Owner is the owning user name reported by the sandbox.
	Owner string
	// Group is the owning group name reported by the sandbox.
	Group string
	// ModifiedTime is the last modification timestamp when envd provides it.
	ModifiedTime time.Time
	// SymlinkTarget is set when the entry is a symbolic link and envd reports the target path.
	SymlinkTarget *string
}

func entryInfoFromProto(e *filesystem.EntryInfo) *EntryInfo {
	if e == nil {
		return nil
	}
	info := &EntryInfo{
		Name:        e.Name,
		Path:        e.Path,
		Size:        e.Size,
		Mode:        e.Mode,
		Permissions: e.Permissions,
		Owner:       e.Owner,
		Group:       e.Group,
	}
	switch e.Type {
	case filesystem.FileType_FILE_TYPE_FILE:
		info.Type = FileTypeFile
	case filesystem.FileType_FILE_TYPE_DIRECTORY:
		info.Type = FileTypeDirectory
	default:
		info.Type = FileTypeUnknown
	}
	if e.ModifiedTime != nil {
		info.ModifiedTime = e.ModifiedTime.AsTime()
	}
	if e.SymlinkTarget != nil {
		t := *e.SymlinkTarget
		info.SymlinkTarget = &t
	}
	return info
}

// EventType describes the kind of filesystem change emitted by WatchDir.
type EventType string

// Known filesystem event types emitted by directory watches.
const (
	// EventCreate means a file or directory was created.
	EventCreate EventType = "create"
	// EventWrite means an existing file was modified.
	EventWrite EventType = "write"
	// EventRemove means a file or directory was removed.
	EventRemove EventType = "remove"
	// EventRename means a file or directory was renamed.
	EventRename EventType = "rename"
	// EventChmod means permissions or mode bits changed.
	EventChmod EventType = "chmod"
)

// FilesystemEvent is a single change notification from WatchDir.
type FilesystemEvent struct {
	// Name is the path or entry name associated with the event.
	Name string
	// Type is the event category.
	Type EventType
}

func filesystemEventFromProto(e *filesystem.FilesystemEvent) FilesystemEvent {
	ev := FilesystemEvent{Name: e.Name}
	switch e.Type {
	case filesystem.EventType_EVENT_TYPE_CREATE:
		ev.Type = EventCreate
	case filesystem.EventType_EVENT_TYPE_WRITE:
		ev.Type = EventWrite
	case filesystem.EventType_EVENT_TYPE_REMOVE:
		ev.Type = EventRemove
	case filesystem.EventType_EVENT_TYPE_RENAME:
		ev.Type = EventRename
	case filesystem.EventType_EVENT_TYPE_CHMOD:
		ev.Type = EventChmod
	}
	return ev
}

// FilesystemOption customizes read, write, stat, and mutating filesystem calls.
type FilesystemOption func(*filesystemOpts)

type filesystemOpts struct {
	user string
	gzip bool
}

// WithUser performs the filesystem operation as the named sandbox user. When
// omitted, the SDK uses DefaultUser.
func WithUser(user string) FilesystemOption {
	return func(o *filesystemOpts) { o.user = user }
}

// WithGzip requests gzip download encoding or gzip-compresses uploads.
func WithGzip(enabled bool) FilesystemOption {
	return func(o *filesystemOpts) { o.gzip = enabled }
}

func applyFilesystemOpts(opts []FilesystemOption) *filesystemOpts {
	o := &filesystemOpts{user: DefaultUser}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// ListOption customizes directory listing behavior.
type ListOption func(*listOpts)

type listOpts struct {
	filesystemOpts
	depth uint32
}

// WithDepth controls how many directory levels List traverses. A depth of 1
// lists only the requested directory's direct children.
func WithDepth(depth uint32) ListOption {
	return func(o *listOpts) { o.depth = depth }
}

// WithListUser performs the directory listing as the named sandbox user.
func WithListUser(user string) ListOption {
	return func(o *listOpts) { o.user = user }
}

func applyListOpts(opts []ListOption) *listOpts {
	o := &listOpts{
		filesystemOpts: filesystemOpts{user: DefaultUser},
		depth:          1,
	}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WatchOption customizes directory watching behavior.
type WatchOption func(*watchOpts)

type watchOpts struct {
	filesystemOpts
	recursive bool
}

// WithRecursive controls whether WatchDir observes nested directories.
func WithRecursive(recursive bool) WatchOption {
	return func(o *watchOpts) { o.recursive = recursive }
}

// WithWatchUser performs the directory watch as the named sandbox user.
func WithWatchUser(user string) WatchOption {
	return func(o *watchOpts) { o.user = user }
}

func applyWatchOpts(opts []WatchOption) *watchOpts {
	o := &watchOpts{
		filesystemOpts: filesystemOpts{user: DefaultUser},
	}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WatchHandle represents a live directory watch. Call Stop when the caller no
// longer needs events so the underlying stream and goroutine can be released.
type WatchHandle struct {
	events chan FilesystemEvent
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

// Events returns the read-only event channel. The channel is closed when the
// watch stops, the context is canceled, or the server closes the stream.
func (w *WatchHandle) Events() <-chan FilesystemEvent {
	return w.events
}

// Err returns the terminal stream error, if the watch ended because of a server
// or network failure. A caller-initiated Stop usually leaves Err nil.
func (w *WatchHandle) Err() error {
	return w.err
}

// Stop cancels the watch and waits for the event goroutine to exit.
func (w *WatchHandle) Stop() {
	w.cancel()
	<-w.done
}

// Filesystem provides file upload, download, metadata, directory, and watch
// helpers for a single sandbox.
type Filesystem struct {
	sandbox *Sandbox
	rpc     filesystemconnect.FilesystemClient
}

func newFilesystem(s *Sandbox) *Filesystem {
	rpc := filesystemconnect.NewFilesystemClient(
		s.client.config.HTTPClient,
		s.envdURL(),
	)
	return &Filesystem{sandbox: s, rpc: rpc}
}

func checkHTTPResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return newAPIError(resp, body)
}

// Read downloads the complete file at path into memory. For large files, prefer
// ReadStream so the caller can stream bytes without buffering the whole payload.
func (fs *Filesystem) Read(ctx context.Context, path string, opts ...FilesystemOption) ([]byte, error) {
	resp, err := fs.doRead(ctx, path, opts...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return io.ReadAll(r)
}

// ReadText downloads a file and converts its bytes to a string. The SDK does
// not validate or transcode character encoding; callers should use it for text
// files that are already encoded as expected by the application.
func (fs *Filesystem) ReadText(ctx context.Context, path string, opts ...FilesystemOption) (string, error) {
	data, err := fs.Read(ctx, path, opts...)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadStream opens a streaming download for path. The caller is responsible for
// closing the returned reader.
func (fs *Filesystem) ReadStream(ctx context.Context, path string, opts ...FilesystemOption) (io.ReadCloser, error) {
	resp, err := fs.doRead(ctx, path, opts...)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (fs *Filesystem) doRead(ctx context.Context, path string, opts ...FilesystemOption) (*http.Response, error) {
	o := applyFilesystemOpts(opts)
	downloadURL := fs.sandbox.DownloadURL(path, WithFileUser(o.user))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if o.gzip {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	setReqidHeader(ctx, req)

	httpClient := fs.sandbox.client.config.HTTPClient
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}

	if err := checkHTTPResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// Write uploads data to path and returns metadata for the written file. Parent
// directory behavior is controlled by the sandbox filesystem service.
func (fs *Filesystem) Write(ctx context.Context, path string, data []byte, opts ...FilesystemOption) (*EntryInfo, error) {
	o := applyFilesystemOpts(opts)
	uploadURL := fs.sandbox.UploadURL(path, WithFileUser(o.user))
	if o.gzip {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(data); err != nil {
			_ = gz.Close()
			return nil, err
		}
		if err := gz.Close(); err != nil {
			return nil, err
		}
		data = buf.Bytes()
	}

	pr, pw := io.Pipe()
	writer := newMultipartWriter(pw)

	go func() {
		if err := writer.writeFile("file", path, data); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := writer.close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pr)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.contentType())
	if o.gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	setReqidHeader(ctx, req)

	httpClient := fs.sandbox.client.config.HTTPClient
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPResponse(resp); err != nil {
		return nil, err
	}

	return fs.GetInfo(ctx, path, opts...)
}

// WriteEntry is a single file payload used by WriteFiles.
type WriteEntry struct {
	// Path is the destination path inside the sandbox.
	Path string
	// Data is the complete file content to upload.
	Data []byte
}

// WriteFiles uploads multiple files in one multipart request. When one file is
// supplied, the SDK delegates to Write so the returned behavior stays identical
// to the single-file API.
func (fs *Filesystem) WriteFiles(ctx context.Context, files []WriteEntry, opts ...FilesystemOption) ([]*EntryInfo, error) {
	if len(files) == 0 {
		return nil, nil
	}

	if len(files) == 1 {
		info, err := fs.Write(ctx, files[0].Path, files[0].Data, opts...)
		if err != nil {
			return nil, err
		}
		return []*EntryInfo{info}, nil
	}

	o := applyFilesystemOpts(opts)
	uploadURL := fs.sandbox.batchUploadURL(o.user)

	pr, pw := io.Pipe()
	writer := newMultipartWriter(pw)

	go func() {
		for _, f := range files {
			data := f.Data
			if o.gzip {
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				if _, err := gz.Write(data); err != nil {
					_ = gz.Close()
					pw.CloseWithError(err)
					return
				}
				if err := gz.Close(); err != nil {
					pw.CloseWithError(err)
					return
				}
				data = buf.Bytes()
			}
			if err := writer.writeFileFullPath("file", f.Path, data); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		if err := writer.close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pr)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.contentType())
	if o.gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	setReqidHeader(ctx, req)

	httpClient := fs.sandbox.client.config.HTTPClient
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload files: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPResponse(resp); err != nil {
		return nil, err
	}

	infos := make([]*EntryInfo, 0, len(files))
	for _, f := range files {
		info, err := fs.GetInfo(ctx, f.Path, opts...)
		if err != nil {
			return nil, fmt.Errorf("get info for %s: %w", f.Path, err)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// List returns filesystem entries under path. Use WithDepth to control recursive
// traversal depth; the default depth lists direct children only.
func (fs *Filesystem) List(ctx context.Context, path string, opts ...ListOption) ([]EntryInfo, error) {
	o := applyListOpts(opts)
	req := connect.NewRequest(&filesystem.ListDirRequest{
		Path:  path,
		Depth: o.depth,
	})
	fs.sandbox.setEnvdAuth(req, o.user)

	resp, err := fs.rpc.ListDir(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list dir: %w", err)
	}

	entries := make([]EntryInfo, 0, len(resp.Msg.Entries))
	for _, e := range resp.Msg.Entries {
		info := entryInfoFromProto(e)
		if info == nil {
			continue
		}
		entries = append(entries, *info)
	}
	return entries, nil
}

// Exists reports whether path exists. A not-found response is converted to
// false with a nil error; other errors are returned unchanged.
func (fs *Filesystem) Exists(ctx context.Context, path string, opts ...FilesystemOption) (bool, error) {
	_, err := fs.GetInfo(ctx, path, opts...)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetInfo returns metadata for a single file, directory, or symlink.
func (fs *Filesystem) GetInfo(ctx context.Context, path string, opts ...FilesystemOption) (*EntryInfo, error) {
	o := applyFilesystemOpts(opts)
	req := connect.NewRequest(&filesystem.StatRequest{Path: path})
	fs.sandbox.setEnvdAuth(req, o.user)

	resp, err := fs.rpc.Stat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	return entryInfoFromProto(resp.Msg.Entry), nil
}

// MakeDir creates a directory and returns metadata for the resulting entry.
func (fs *Filesystem) MakeDir(ctx context.Context, path string, opts ...FilesystemOption) (*EntryInfo, error) {
	o := applyFilesystemOpts(opts)
	req := connect.NewRequest(&filesystem.MakeDirRequest{Path: path})
	fs.sandbox.setEnvdAuth(req, o.user)

	resp, err := fs.rpc.MakeDir(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return entryInfoFromProto(resp.Msg.Entry), nil
}

// Remove deletes a file or directory at path. The server decides whether
// non-empty directories are allowed.
func (fs *Filesystem) Remove(ctx context.Context, path string, opts ...FilesystemOption) error {
	o := applyFilesystemOpts(opts)
	req := connect.NewRequest(&filesystem.RemoveRequest{Path: path})
	fs.sandbox.setEnvdAuth(req, o.user)

	_, err := fs.rpc.Remove(ctx, req)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}

// Rename moves an entry from oldPath to newPath and returns metadata for the
// moved entry at its new location.
func (fs *Filesystem) Rename(ctx context.Context, oldPath, newPath string, opts ...FilesystemOption) (*EntryInfo, error) {
	o := applyFilesystemOpts(opts)
	req := connect.NewRequest(&filesystem.MoveRequest{
		Source:      oldPath,
		Destination: newPath,
	})
	fs.sandbox.setEnvdAuth(req, o.user)

	resp, err := fs.rpc.Move(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}
	return entryInfoFromProto(resp.Msg.Entry), nil
}

// WatchDir streams filesystem changes for path until the context is canceled,
// the returned handle is stopped, or the server closes the stream.
func (fs *Filesystem) WatchDir(ctx context.Context, path string, opts ...WatchOption) (*WatchHandle, error) {
	o := applyWatchOpts(opts)

	watchCtx, cancel := context.WithCancel(ctx)
	req := connect.NewRequest(&filesystem.WatchDirRequest{
		Path:      path,
		Recursive: o.recursive,
	})
	fs.sandbox.setEnvdAuth(req, o.user)

	stream, err := fs.rpc.WatchDir(watchCtx, req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watch dir: %w", err)
	}

	w := &WatchHandle{
		events: make(chan FilesystemEvent, 64),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(w.done)
		defer close(w.events)
		for stream.Receive() {
			msg := stream.Msg()
			if fsEvent := msg.GetFilesystem(); fsEvent != nil {
				ev := filesystemEventFromProto(fsEvent)
				select {
				case w.events <- ev:
				case <-watchCtx.Done():
					return
				}
			}
		}
		if err := stream.Err(); err != nil && watchCtx.Err() == nil {
			w.err = err
		}
	}()

	return w, nil
}
