package sandbox

import (
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
)

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

// Metadata is caller-defined key-value data attached to a sandbox. Metadata is
// useful for filtering list results and for carrying application-level labels.
type Metadata map[string]string

// NetworkConfig describes outbound network controls for a sandbox.
type NetworkConfig struct {
	// AllowOut contains outbound destinations that are explicitly allowed.
	AllowOut *[]string

	// AllowPublicTraffic controls whether the sandbox can reach public networks.
	AllowPublicTraffic *bool

	// DenyOut contains outbound destinations that are explicitly denied.
	DenyOut *[]string

	// MaskRequestHost optionally rewrites or masks outbound request hosts.
	MaskRequestHost *string
}

// LifecycleAction controls what happens when a sandbox reaches its timeout.
type LifecycleAction string

const (
	// LifecycleActionKill terminates the sandbox when timeout is reached.
	LifecycleActionKill LifecycleAction = "kill"
	// LifecycleActionPause pauses the sandbox when timeout is reached.
	LifecycleActionPause LifecycleAction = "pause"
)

// SandboxLifecycle configures timeout behavior. It mirrors lifecycle options
// while keeping AutoPause available for source compatibility.
type SandboxLifecycle struct {
	OnTimeout  LifecycleAction
	AutoResume *bool
}

// SandboxInfoLifecycle is the lifecycle policy returned by sandbox info APIs.
type SandboxInfoLifecycle struct {
	OnTimeout  LifecycleAction
	AutoResume bool
}

// AutoResumeConfig is the wire representation used by the create sandbox API.
type AutoResumeConfig struct {
	Enabled bool `json:"enabled"`
}

// VolumeMount attaches a persistent volume to a sandbox path.
type VolumeMount struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// MCPConfig is a raw MCP gateway configuration map.
type MCPConfig map[string]any

// SandboxState is the lifecycle state reported by the Sandbox API.
type SandboxState string

// Known sandbox lifecycle states.
const (
	// StateRunning means the sandbox is currently active and can accept envd requests.
	StateRunning SandboxState = "running"
	// StatePaused means the sandbox is paused and must be resumed before use.
	StatePaused SandboxState = "paused"
)

// CreateParams contains the inputs for creating a sandbox from a template.
type CreateParams struct {
	// TemplateID identifies the template used to boot the sandbox.
	TemplateID string

	// Timeout is the sandbox time-to-live in seconds.
	Timeout *int32

	// AutoPause controls whether the sandbox pauses instead of terminating when
	// the timeout expires.
	AutoPause *bool

	// AllowInternetAccess controls broad outbound internet access.
	AllowInternetAccess *bool

	// Secure enables secure system communication with the sandbox when supported.
	Secure *bool

	// EnvVars contains environment variables injected into the sandbox.
	EnvVars *map[string]string

	// Metadata attaches caller-defined labels to the sandbox.
	Metadata *Metadata

	// Network applies outbound network policy for the sandbox.
	Network *NetworkConfig

	// Lifecycle configures whether the sandbox is killed or paused on timeout.
	Lifecycle *SandboxLifecycle

	// MCP enables MCP gateway configuration when supported by the template.
	MCP *MCPConfig

	// VolumeMounts maps sandbox mount paths to volume names.
	VolumeMounts map[string]string
}

// ConnectParams controls how an existing sandbox connection is established.
type ConnectParams struct {
	// Timeout is the connection timeout in seconds.
	Timeout int32
}

// RefreshParams extends the sandbox lifetime.
type RefreshParams struct {
	// Duration is the number of seconds to add to the sandbox lifetime.
	Duration *int
}

// ListParams filters and paginates sandbox list requests.
type ListParams struct {
	// Metadata filters by metadata query expression.
	Metadata *string

	// State restricts results to the given lifecycle states.
	State *[]SandboxState

	// NextToken continues a previous paginated list request.
	NextToken *string

	// Limit caps the number of sandboxes returned by one request.
	Limit *int32
}

// GetMetricsParams selects the time range for a single sandbox metrics query.
type GetMetricsParams struct {
	// Start is the inclusive Unix timestamp lower bound.
	Start *int64

	// End is the inclusive Unix timestamp upper bound.
	End *int64
}

func (p *GetMetricsParams) toAPI() *apis.GetSandboxMetricsParams {
	if p == nil {
		return nil
	}
	return &apis.GetSandboxMetricsParams{
		Start: p.Start,
		End:   p.End,
	}
}

// GetLogsParams selects the logs returned for a sandbox.
type GetLogsParams struct {
	// Start is the Unix timestamp or offset used by the API as the log cursor.
	Start *int64

	// Limit caps the number of log records returned by one request.
	Limit *int32
}

func (p *GetLogsParams) toAPI() *apis.GetSandboxLogsParams {
	if p == nil {
		return nil
	}
	return &apis.GetSandboxLogsParams{
		Start: p.Start,
		Limit: p.Limit,
	}
}

// GetSandboxesMetricsParams selects multiple sandboxes for a metrics query.
type GetSandboxesMetricsParams struct {
	// SandboxIds is the list of sandbox IDs to include in the response.
	SandboxIds []string
}

func (p *GetSandboxesMetricsParams) toAPI() *apis.GetSandboxesMetricsParams {
	if p == nil {
		return nil
	}
	return &apis.GetSandboxesMetricsParams{
		SandboxIds: p.SandboxIds,
	}
}

// SandboxInfo is the detailed state returned for one sandbox.
type SandboxInfo struct {
	// SandboxID is the stable identifier used by API and CLI operations.
	SandboxID string
	// TemplateID is the template that produced this sandbox.
	TemplateID string
	// ClientID identifies the client/session associated with the sandbox.
	ClientID string
	// Alias is an optional human-readable template alias.
	Alias *string
	// Domain is the base domain used to construct exposed sandbox hosts.
	Domain *string
	// State is the current sandbox lifecycle state.
	State SandboxState
	// CPUCount is the number of virtual CPUs assigned to the sandbox.
	CPUCount int32
	// MemoryMB is the memory allocation in megabytes.
	MemoryMB int32
	// DiskSizeMB is the disk allocation in megabytes.
	DiskSizeMB int32
	// EnvdVersion is the envd runtime version running in the sandbox.
	EnvdVersion string
	// StartedAt is the timestamp when the sandbox entered service.
	StartedAt time.Time
	// EndAt is the timestamp when the sandbox is scheduled to expire.
	EndAt time.Time
	// Metadata contains caller-defined labels attached at creation time.
	Metadata *Metadata
	// Name is the human-readable template name/alias when the API provides it.
	Name *string
	// AllowInternetAccess reports broad outbound internet access when provided.
	AllowInternetAccess *bool
	// Network is the sandbox network policy when provided.
	Network *NetworkConfig
	// Lifecycle is the timeout behavior when provided.
	Lifecycle *SandboxInfoLifecycle
	// VolumeMounts lists persistent volumes mounted in the sandbox.
	VolumeMounts []VolumeMount
}

// ListedSandbox is the compact sandbox representation returned by list calls.
type ListedSandbox struct {
	SandboxID    string
	TemplateID   string
	ClientID     string
	Alias        *string
	State        SandboxState
	CPUCount     int32
	MemoryMB     int32
	DiskSizeMB   int32
	EnvdVersion  string
	StartedAt    time.Time
	EndAt        time.Time
	Metadata     *Metadata
	Name         *string
	VolumeMounts []VolumeMount
}

// SandboxMetric is a single resource-usage sample for a sandbox.
type SandboxMetric struct {
	// CPUCount is the number of virtual CPUs available to the sandbox.
	CPUCount int32
	// CPUUsedPct is the sampled CPU utilization percentage.
	CPUUsedPct float32
	// MemTotal is total memory in bytes.
	MemTotal int64
	// MemUsed is used memory in bytes.
	MemUsed int64
	// DiskTotal is total disk space in bytes.
	DiskTotal int64
	// DiskUsed is used disk space in bytes.
	DiskUsed int64
	// Timestamp is the sample time.
	Timestamp time.Time
	// TimestampUnix is the sample time as a Unix timestamp.
	TimestampUnix int64
}

// SandboxLogs groups raw log lines and structured log entries returned by the API.
type SandboxLogs struct {
	Logs       []SandboxLog
	LogEntries []SandboxLogEntry
}

// SandboxLog is a raw sandbox log line with its timestamp.
type SandboxLog struct {
	Line      string
	Timestamp time.Time
}

// SandboxLogEntry is a structured log record with level, message, and fields.
type SandboxLogEntry struct {
	Level     LogLevel
	Message   string
	Fields    map[string]string
	Timestamp time.Time
}

// SandboxesWithMetrics maps sandbox IDs to their latest metrics sample.
type SandboxesWithMetrics struct {
	Sandboxes map[string]SandboxMetric
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

func sandboxInfoFromAPI(d *apis.SandboxDetail) *SandboxInfo {
	if d == nil {
		return nil
	}
	info := &SandboxInfo{
		SandboxID:           d.SandboxID,
		TemplateID:          d.TemplateID,
		ClientID:            d.ClientID,
		Alias:               d.Alias,
		Domain:              d.Domain,
		State:               SandboxState(d.State),
		CPUCount:            d.CPUCount,
		MemoryMB:            d.MemoryMB,
		DiskSizeMB:          d.DiskSizeMB,
		EnvdVersion:         d.EnvdVersion,
		StartedAt:           d.StartedAt,
		EndAt:               d.EndAt,
		AllowInternetAccess: d.AllowInternetAccess,
	}
	if d.Metadata != nil {
		m := Metadata(*d.Metadata)
		info.Metadata = &m
	}
	if d.Network != nil {
		info.Network = networkConfigFromAPI(d.Network)
	}
	if d.Lifecycle != nil {
		info.Lifecycle = &SandboxInfoLifecycle{
			OnTimeout:  LifecycleAction(d.Lifecycle.OnTimeout),
			AutoResume: d.Lifecycle.AutoResume,
		}
	}
	if d.VolumeMounts != nil {
		info.VolumeMounts = volumeMountsFromAPI(*d.VolumeMounts)
	}
	return info
}

func listedSandboxFromAPI(a apis.ListedSandbox) ListedSandbox {
	ls := ListedSandbox{
		SandboxID:   a.SandboxID,
		TemplateID:  a.TemplateID,
		ClientID:    a.ClientID,
		Alias:       a.Alias,
		State:       SandboxState(a.State),
		CPUCount:    a.CPUCount,
		MemoryMB:    a.MemoryMB,
		DiskSizeMB:  a.DiskSizeMB,
		EnvdVersion: a.EnvdVersion,
		StartedAt:   a.StartedAt,
		EndAt:       a.EndAt,
	}
	if a.Metadata != nil {
		m := Metadata(*a.Metadata)
		ls.Metadata = &m
	}
	if a.VolumeMounts != nil {
		ls.VolumeMounts = volumeMountsFromAPI(*a.VolumeMounts)
	}
	return ls
}

func listedSandboxesFromAPI(a []apis.ListedSandbox) []ListedSandbox {
	if a == nil {
		return nil
	}
	result := make([]ListedSandbox, len(a))
	for i, s := range a {
		result[i] = listedSandboxFromAPI(s)
	}
	return result
}

func sandboxMetricFromAPI(a apis.SandboxMetric) SandboxMetric {
	return SandboxMetric{
		CPUCount:      a.CPUCount,
		CPUUsedPct:    a.CPUUsedPct,
		MemTotal:      a.MemTotal,
		MemUsed:       a.MemUsed,
		DiskTotal:     a.DiskTotal,
		DiskUsed:      a.DiskUsed,
		Timestamp:     a.Timestamp,
		TimestampUnix: a.TimestampUnix,
	}
}

func sandboxMetricsFromAPI(a []apis.SandboxMetric) []SandboxMetric {
	if a == nil {
		return nil
	}
	result := make([]SandboxMetric, len(a))
	for i, m := range a {
		result[i] = sandboxMetricFromAPI(m)
	}
	return result
}

func sandboxLogsFromAPI(a *apis.SandboxLogs) *SandboxLogs {
	if a == nil {
		return nil
	}
	result := &SandboxLogs{
		Logs:       make([]SandboxLog, 0, len(a.Logs)),
		LogEntries: make([]SandboxLogEntry, 0, len(a.LogEntries)),
	}
	for _, l := range a.Logs {
		result.Logs = append(result.Logs, SandboxLog{Line: l.Line, Timestamp: l.Timestamp})
	}
	for _, e := range a.LogEntries {
		result.LogEntries = append(result.LogEntries, SandboxLogEntry{
			Level:     LogLevel(e.Level),
			Message:   e.Message,
			Fields:    e.Fields,
			Timestamp: e.Timestamp,
		})
	}
	return result
}

func sandboxesWithMetricsFromAPI(a *apis.SandboxesWithMetrics) *SandboxesWithMetrics {
	if a == nil {
		return nil
	}
	result := &SandboxesWithMetrics{Sandboxes: make(map[string]SandboxMetric, len(a.Sandboxes))}
	for k, v := range a.Sandboxes {
		result.Sandboxes[k] = sandboxMetricFromAPI(v)
	}
	return result
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

func (p *CreateParams) toAPI() (apis.CreateSandboxJSONRequestBody, error) {
	body := apis.CreateSandboxJSONRequestBody{
		TemplateID:          p.TemplateID,
		Timeout:             p.Timeout,
		AutoPause:           p.AutoPause,
		AllowInternetAccess: p.AllowInternetAccess,
		Secure:              p.Secure,
	}
	if p.EnvVars != nil {
		ev := apis.EnvVars(*p.EnvVars)
		body.EnvVars = &ev
	}
	if p.Metadata != nil {
		m := apis.SandboxMetadata(*p.Metadata)
		body.Metadata = &m
	}
	if p.Network != nil {
		body.Network = networkConfigToAPI(p.Network)
	}
	if p.Lifecycle != nil {
		autoPause := p.Lifecycle.OnTimeout == LifecycleActionPause
		body.AutoPause = &autoPause
		if p.Lifecycle.AutoResume != nil {
			body.AutoResume = &apis.SandboxAutoResumeConfig{Enabled: *p.Lifecycle.AutoResume}
		}
	}
	if p.MCP != nil {
		mcp := apis.Mcp(*p.MCP)
		body.Mcp = &mcp
	}
	if len(p.VolumeMounts) > 0 {
		mounts := make([]apis.SandboxVolumeMount, 0, len(p.VolumeMounts))
		for path, name := range p.VolumeMounts {
			mounts = append(mounts, apis.SandboxVolumeMount{Name: name, Path: path})
		}
		body.VolumeMounts = &mounts
	}
	return body, nil
}

func (p *ConnectParams) toAPI() apis.ConnectSandboxJSONRequestBody {
	return apis.ConnectSandboxJSONRequestBody{
		Timeout: p.Timeout,
	}
}

func (p *RefreshParams) toAPI() apis.RefreshSandboxJSONRequestBody {
	return apis.RefreshSandboxJSONRequestBody{
		Duration: p.Duration,
	}
}

func (p *ListParams) toAPI() *apis.ListSandboxesV2Params {
	if p == nil {
		return nil
	}
	params := &apis.ListSandboxesV2Params{
		Metadata:  p.Metadata,
		NextToken: p.NextToken,
		Limit:     p.Limit,
	}
	if p.State != nil {
		states := make([]apis.SandboxState, len(*p.State))
		for i, s := range *p.State {
			states[i] = apis.SandboxState(s)
		}
		params.State = &states
	}
	return params
}

func networkConfigToAPI(n *NetworkConfig) *apis.SandboxNetworkConfig {
	if n == nil {
		return nil
	}
	return &apis.SandboxNetworkConfig{
		AllowOut:           n.AllowOut,
		AllowPublicTraffic: n.AllowPublicTraffic,
		DenyOut:            n.DenyOut,
		MaskRequestHost:    n.MaskRequestHost,
	}
}

func networkConfigFromAPI(n *apis.SandboxNetworkConfig) *NetworkConfig {
	if n == nil {
		return nil
	}
	return &NetworkConfig{
		AllowOut:           n.AllowOut,
		AllowPublicTraffic: n.AllowPublicTraffic,
		DenyOut:            n.DenyOut,
		MaskRequestHost:    n.MaskRequestHost,
	}
}

func volumeMountsFromAPI(mounts []apis.SandboxVolumeMount) []VolumeMount {
	result := make([]VolumeMount, len(mounts))
	for i, m := range mounts {
		result[i] = VolumeMount{Name: m.Name, Path: m.Path}
	}
	return result
}
