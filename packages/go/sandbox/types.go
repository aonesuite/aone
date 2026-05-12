package sandbox

import (
	"strconv"
	"strings"
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

// SandboxLifecycle configures timeout behavior. OnTimeout selects whether the
// sandbox is killed or paused at expiry; AutoResume re-arms automatic resume.
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
	Metadata *Metadata
	Name     *string
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
		SandboxID:   stringValue(d.SandboxID),
		TemplateID:  stringValue(d.TemplateID),
		ClientID:    stringValue(d.ClientID),
		State:       SandboxState(stringValue(d.State)),
		EnvdVersion: stringValue(d.EnvdVersion),
	}
	if d.StartedAt != nil {
		info.StartedAt = *d.StartedAt
	}
	if d.EndAt != nil {
		info.EndAt = *d.EndAt
	}
	return info
}

func listedSandboxFromAPI(a apis.ListedSandbox) ListedSandbox {
	ls := ListedSandbox{
		SandboxID:   stringValue(a.SandboxID),
		TemplateID:  stringValue(a.TemplateID),
		ClientID:    stringValue(a.ClientID),
		State:       SandboxState(stringValue(a.State)),
		EnvdVersion: stringValue(a.EnvdVersion),
	}
	if a.StartedAt != nil {
		ls.StartedAt = *a.StartedAt
	}
	if a.EndAt != nil {
		ls.EndAt = *a.EndAt
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
	ts := timeFromUnix(a.TimestampUnix)
	if a.Timestamp != nil {
		if parsed, err := time.Parse(time.RFC3339Nano, *a.Timestamp); err == nil {
			ts = parsed
		}
	}
	return SandboxMetric{
		CPUCount:      int32Value(a.CPUCount),
		CPUUsedPct:    float32Value(a.CPUUsedPct),
		MemTotal:      int64Value(a.MemTotal),
		MemUsed:       int64Value(a.MemUsed),
		DiskTotal:     int64Value(a.DiskTotal),
		DiskUsed:      int64Value(a.DiskUsed),
		Timestamp:     ts,
		TimestampUnix: int64Value(a.TimestampUnix),
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
		Logs:       make([]SandboxLog, 0),
		LogEntries: make([]SandboxLogEntry, 0),
	}
	if a.Logs != nil {
		for _, e := range *a.Logs {
			entry := sandboxLogEntryFromAPI(e)
			result.Logs = append(result.Logs, SandboxLog{Line: entry.Message, Timestamp: entry.Timestamp})
		}
	}
	if a.LogEntries != nil {
		for _, e := range *a.LogEntries {
			result.LogEntries = append(result.LogEntries, sandboxLogEntryFromAPI(e))
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

func (p *CreateParams) toAPI() (apis.CreateSandboxJSONRequestBody, error) {
	body := apis.CreateSandboxJSONRequestBody{
		TemplateID:          &p.TemplateID,
		Timeout:             p.Timeout,
		AllowInternetAccess: p.AllowInternetAccess,
		Secure:              p.Secure,
	}
	if p.EnvVars != nil {
		body.EnvVars = (*map[string]string)(p.EnvVars)
	}
	if p.Metadata != nil {
		m := map[string]string(*p.Metadata)
		body.Metadata = &m
	}
	if p.Network != nil {
		body.Network = networkConfigToAPI(p.Network)
	}
	return body, nil
}

func (p *ConnectParams) toAPI() apis.ConnectSandboxJSONRequestBody {
	return apis.ConnectSandboxJSONRequestBody{
		Timeout: &p.Timeout,
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
		Cursor: p.NextToken,
		Limit:  p.Limit,
	}
	if p.State != nil {
		state := joinSandboxStates(*p.State)
		params.State = &state
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

func sandboxLogEntryFromAPI(e apis.ServiceSandboxLogEntry) SandboxLogEntry {
	ts := parseTime(stringValue(e.Timestamp))
	fields := map[string]string(nil)
	if e.Fields != nil {
		fields = *e.Fields
	}
	return SandboxLogEntry{
		Level:     LogLevel(stringValue(e.Level)),
		Message:   stringValue(e.Message),
		Fields:    fields,
		Timestamp: ts,
	}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func int64Value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func float32Value(v *float32) float32 {
	if v == nil {
		return 0
	}
	return *v
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func timeValue(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}

func timeFromUnix(v *int64) time.Time {
	if v == nil || *v == 0 {
		return time.Time{}
	}
	return time.Unix(*v, 0).UTC()
}

func parseTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return parsed
	}
	if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC()
	}
	return time.Time{}
}

func joinSandboxStates(states []SandboxState) string {
	if len(states) == 0 {
		return ""
	}
	parts := make([]string, len(states))
	for i, s := range states {
		parts[i] = string(s)
	}
	return strings.Join(parts, ",")
}
