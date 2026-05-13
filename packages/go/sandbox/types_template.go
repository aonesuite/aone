package sandbox

import (
	"time"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/apis"
)

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

// TemplateBuildStatus is the lifecycle state of a template build.
type TemplateBuildStatus string

// Known template build states returned by the API.
const (
	// BuildStatusReady means the build completed and can be used to create sandboxes.
	BuildStatusReady TemplateBuildStatus = "ready"
	// BuildStatusError means the build failed.
	BuildStatusError TemplateBuildStatus = "error"
	// BuildStatusBuilding means build steps are currently running.
	BuildStatusBuilding TemplateBuildStatus = "building"
	// BuildStatusWaiting means the build has been accepted but has not started.
	BuildStatusWaiting TemplateBuildStatus = "waiting"
	// BuildStatusUploaded means build files were uploaded and are awaiting processing.
	BuildStatusUploaded TemplateBuildStatus = "uploaded"
)

// LogLevel is the severity level used by sandbox and template build logs.
type LogLevel string

// Known log severity levels.
const (
	// LogLevelDebug is verbose diagnostic output.
	LogLevelDebug LogLevel = "debug"
	// LogLevelError is an error-level log entry.
	LogLevelError LogLevel = "error"
	// LogLevelInfo is an informational log entry.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is a warning log entry.
	LogLevelWarn LogLevel = "warn"
)

// LogsDirection controls whether log pagination moves forward or backward from
// a cursor.
type LogsDirection string

// Known directions for paginated log retrieval.
const (
	// LogsDirectionBackward reads logs before the cursor.
	LogsDirectionBackward LogsDirection = "backward"
	// LogsDirectionForward reads logs after the cursor.
	LogsDirectionForward LogsDirection = "forward"
)

// LogsSource selects the log storage stream to query.
type LogsSource string

// Known log sources for template build logs.
const (
	// LogsSourcePersistent reads persisted build logs.
	LogsSourcePersistent LogsSource = "persistent"
	// LogsSourceTemporary reads temporary live logs when available.
	LogsSourceTemporary LogsSource = "temporary"
)

// TemplateStep describes one Dockerfile step used by TemplateBuilder helpers.
type TemplateStep struct {
	// Args contains step-specific arguments, such as shell command tokens or copy paths.
	Args *[]string

	// FilesHash is used by file-copy steps to reference an uploaded file bundle.
	FilesHash *string

	// Force requests re-execution even when cached output might be available.
	Force *bool

	// Type is the build step kind, for example RUN, COPY, WORKDIR, USER, or ENV.
	Type string
}

// CreateTemplateParams contains the optional metadata and resource settings for
// creating a template record.
type CreateTemplateParams struct {
	Alias *string

	CPUCount *int32

	DiskSizeMB *int32

	Dockerfile *string

	MemoryMB *int32

	Name *string

	Public *bool

	ReadyCmd *string

	StartCmd *string
}

func (p *CreateTemplateParams) toAPI() apis.CreateTemplateV3JSONRequestBody {
	return apis.CreateTemplateV3JSONRequestBody{
		Alias:      p.Alias,
		CPUCount:   p.CPUCount,
		DiskSizeMb: p.DiskSizeMB,
		Dockerfile: p.Dockerfile,
		MemoryMb:   p.MemoryMB,
		Name:       p.Name,
		Public:     p.Public,
		ReadyCmd:   p.ReadyCmd,
		StartCmd:   p.StartCmd,
	}
}

// UpdateTemplateParams contains mutable template properties.
type UpdateTemplateParams struct {
	// Public controls whether the template is visible outside its owner scope.
	Public *bool
}

func (p *UpdateTemplateParams) toAPI() apis.UpdateTemplateJSONRequestBody {
	return apis.UpdateTemplateJSONRequestBody{
		Public: p.Public,
	}
}

// ListTemplatesParams filters template list requests.
type ListTemplatesParams struct {
	// APIKeyID filters by the API key that created the resource.
	APIKeyID *string

	// Name filters by template name.
	Name *string

	// BuildStatus filters by latest build status.
	BuildStatus *string

	// Public filters by visibility; empty means no visibility filter.
	Public *string

	// Cursor is the pagination cursor.
	Cursor *string

	// Limit is the maximum number of items to return.
	Limit *int32
}

func (p *ListTemplatesParams) toAPI() *apis.GetTemplatesParams {
	if p == nil {
		return nil
	}
	return &apis.GetTemplatesParams{
		APIKeyID:    p.APIKeyID,
		Name:        p.Name,
		BuildStatus: p.BuildStatus,
		Public:      p.Public,
		Cursor:      p.Cursor,
		Limit:       p.Limit,
	}
}

// GetBuildLogsParams controls paginated template build log retrieval.
type GetBuildLogsParams struct {
	Cursor *int64

	Limit *int32

	Direction *LogsDirection

	Level *LogLevel

	Source *LogsSource
}

func (p *GetBuildLogsParams) toAPI() *apis.GetTemplateBuildLogsParams {
	if p == nil {
		return nil
	}
	params := &apis.GetTemplateBuildLogsParams{
		Cursor: p.Cursor,
		Limit:  p.Limit,
	}
	if p.Direction != nil {
		dir := string(*p.Direction)
		params.Direction = &dir
	}
	if p.Level != nil {
		level := string(*p.Level)
		params.Level = &level
	}
	if p.Source != nil {
		src := string(*p.Source)
		params.Source = &src
	}
	return params
}

// Template is the summary representation returned by template list calls.
type Template struct {
	TemplateID  string
	Aliases     []string
	Names       []string
	BuildID     string
	BuildStatus TemplateBuildStatus
	CPUCount    int32
	MemoryMB    int32
	DiskSizeMB  int32
	EnvdVersion string
	Public      bool
	Source      string
	Editable    bool
	Deletable   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TemplateBuildInfo is the status response for a specific template build.
type TemplateBuildInfo struct {
	TemplateID  string
	BuildID     string
	Status      TemplateBuildStatus
	EnvdVersion string
	Logs        []string
	LogEntries  []BuildLogEntry
}

// TemplateBuildLogs contains structured logs for a template build.
type TemplateBuildLogs struct {
	Logs []BuildLogEntry
}

// BuildLogEntry is a single structured template-build log record.
type BuildLogEntry struct {
	Level     LogLevel
	Message   string
	Step      *string
	Timestamp time.Time
}

// TemplateCreateResponse is returned after creating a template.
type TemplateCreateResponse = Template

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

func templateFromAPI(a apis.Template) Template {
	aliases := []string(nil)
	if a.Aliases != nil {
		aliases = *a.Aliases
	}
	names := []string(nil)
	if a.Names != nil {
		names = *a.Names
	}
	return Template{
		TemplateID:  stringValue(a.TemplateID),
		Aliases:     aliases,
		Names:       names,
		BuildID:     stringValue(a.BuildID),
		BuildStatus: TemplateBuildStatus(stringValue(a.BuildStatus)),
		CPUCount:    int32Value(a.CPUCount),
		MemoryMB:    int32Value(a.MemoryMb),
		DiskSizeMB:  int32Value(a.DiskSizeMb),
		EnvdVersion: stringValue(a.EnvdVersion),
		Public:      boolValue(a.Public),
		Source:      stringValue(a.Source),
		Editable:    boolValue(a.Editable),
		Deletable:   boolValue(a.Deletable),
		CreatedAt:   timeValue(a.CreatedAt),
		UpdatedAt:   timeValue(a.UpdatedAt),
	}
}

func templatesFromAPI(a []apis.Template) []Template {
	if a == nil {
		return nil
	}
	result := make([]Template, len(a))
	for i, t := range a {
		result[i] = templateFromAPI(t)
	}
	return result
}

func templateDetailFromAPI(a *apis.Template) *Template {
	if a == nil {
		return nil
	}
	result := templateFromAPI(*a)
	return &result
}

func templateBuildInfoFromAPI(a *apis.TemplateBuildInfo) *TemplateBuildInfo {
	if a == nil {
		return nil
	}
	result := &TemplateBuildInfo{
		TemplateID:  stringValue(a.TemplateID),
		BuildID:     stringValue(a.BuildID),
		Status:      TemplateBuildStatus(stringValue(a.Status)),
		EnvdVersion: stringValue(a.EnvdVersion),
	}
	if a.Logs != nil {
		result.Logs = append(result.Logs, (*a.Logs)...)
	}
	if a.LogEntries != nil {
		result.LogEntries = buildLogEntriesFromAPI(*a.LogEntries)
	}
	return result
}

func templateBuildLogsFromAPI(a *apis.TemplateBuildLogsResponse) *TemplateBuildLogs {
	if a == nil {
		return nil
	}
	result := &TemplateBuildLogs{}
	if a.Logs != nil {
		result.Logs = buildLogEntriesFromAPI(*a.Logs)
	}
	return result
}

func buildLogEntriesFromAPI(entries []apis.ServiceBuildLogEntry) []BuildLogEntry {
	if entries == nil {
		return nil
	}
	result := make([]BuildLogEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, BuildLogEntry{
			Level:     LogLevel(stringValue(e.Level)),
			Message:   stringValue(e.Message),
			Step:      e.Source,
			Timestamp: parseTime(stringValue(e.Timestamp)),
		})
	}
	return result
}

func templateCreateResponseFromAPI(a *apis.TemplateRequestResponseV3) *TemplateCreateResponse {
	if a == nil {
		return nil
	}
	return templateDetailFromAPI(a)
}
