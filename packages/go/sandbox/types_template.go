package sandbox

import (
	"encoding/json"
	"fmt"
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

// TemplateStep describes one build-system step used to construct a template.
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

func templateStepsToAPI(steps *[]TemplateStep) *[]apis.TemplateStep {
	if steps == nil {
		return nil
	}
	result := make([]apis.TemplateStep, len(*steps))
	for i, s := range *steps {
		result[i] = apis.TemplateStep{
			Args:      s.Args,
			FilesHash: s.FilesHash,
			Force:     s.Force,
			Type:      s.Type,
		}
	}
	return &result
}

// FromImageRegistry stores the registry-specific image source payload as raw
// JSON because the API accepts a discriminated union of registry providers.
type FromImageRegistry = json.RawMessage

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

	Tags *[]string
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

// StartTemplateBuildParams describes how to start a new template build.
type StartTemplateBuildParams struct {
	// Force bypasses cache when the backend supports forced rebuilds.
	Force *bool

	// FromImage starts the build from a container image reference.
	FromImage *string

	// FromImageRegistry starts the build from a registry-specific image source.
	FromImageRegistry *FromImageRegistry

	// FromTemplate starts the build from an existing template.
	FromTemplate *string

	// ReadyCmd is run to determine when sandboxes created from the template are ready.
	ReadyCmd *string

	// StartCmd is the command launched when a sandbox boots from the template.
	StartCmd *string

	// Steps contains the ordered build steps.
	Steps *[]TemplateStep
}

func (p *StartTemplateBuildParams) toAPI() (apis.StartTemplateBuildV2JSONRequestBody, error) {
	body := apis.StartTemplateBuildV2JSONRequestBody{
		Force:        p.Force,
		FromImage:    p.FromImage,
		FromTemplate: p.FromTemplate,
		ReadyCmd:     p.ReadyCmd,
		StartCmd:     p.StartCmd,
		Steps:        templateStepsToAPI(p.Steps),
	}
	if p.FromImageRegistry != nil {
		reg := apis.FromImageRegistry{}
		if err := reg.UnmarshalJSON(*p.FromImageRegistry); err != nil {
			return body, fmt.Errorf("unmarshal from_image_registry: %w", err)
		}
		body.FromImageRegistry = &reg
	}
	return body, nil
}

// ListTemplatesParams filters template list requests.
type ListTemplatesParams struct {
}

func (p *ListTemplatesParams) toAPI() *apis.GetTemplatesParams {
	if p == nil {
		return nil
	}
	return &apis.GetTemplatesParams{}
}

// GetTemplateParams controls pagination when retrieving template builds.
type GetTemplateParams struct {
	// NextToken continues a previous paginated request.
	NextToken *string

	// Limit caps the number of builds returned.
	Limit *int32
}

// GetBuildStatusParams controls build-status log snippets returned with status.
type GetBuildStatusParams struct {
	// LogsOffset starts reading logs from the given offset.
	LogsOffset *int32

	// Limit caps the number of log entries returned.
	Limit *int32

	// Level filters log entries by severity.
	Level *LogLevel
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

// ManageTagsParams assigns tags to a template build or template target.
type ManageTagsParams struct {
	Tags []string

	Target string
}

func (p *ManageTagsParams) toAPI() apis.AssignTemplateTagsJSONRequestBody {
	return apis.AssignTemplateTagsJSONRequestBody{
		Tags:   p.Tags,
		Target: p.Target,
	}
}

// DeleteTagsParams removes tags from a named template.
type DeleteTagsParams struct {
	Name string

	Tags []string
}

func (p *DeleteTagsParams) toAPI() apis.DeleteTemplateTagsJSONRequestBody {
	return apis.DeleteTemplateTagsJSONRequestBody{
		Name: p.Name,
		Tags: p.Tags,
	}
}

// Template is the summary representation returned by template list calls.
type Template struct {
	TemplateID    string
	Aliases       []string
	BuildID       string
	BuildStatus   TemplateBuildStatus
	BuildCount    int32
	CPUCount      int32
	MemoryMB      int32
	DiskSizeMB    int32
	EnvdVersion   string
	Public        bool
	SpawnCount    int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastSpawnedAt *time.Time
}

// TemplateBuild describes one build associated with a template.
type TemplateBuild struct {
	BuildID     string
	Status      TemplateBuildStatus
	CPUCount    int32
	MemoryMB    int32
	DiskSizeMB  *int32
	EnvdVersion *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	FinishedAt  *time.Time
}

// TemplateWithBuilds combines template metadata with its build history.
type TemplateWithBuilds struct {
	TemplateID    string
	Aliases       []string
	Public        bool
	SpawnCount    int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastSpawnedAt *time.Time
	Builds        []TemplateBuild
}

// TemplateBuildInfo is the status response for a specific template build.
type TemplateBuildInfo struct {
	TemplateID string
	BuildID    string
	Status     TemplateBuildStatus
	Logs       []string
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
type TemplateCreateResponse struct {
	TemplateID string
	BuildID    string
	Aliases    []string
	Names      []string
	Tags       []string
	Public     bool
}

// TemplateBuildFileUpload describes whether a build upload URL is available.
type TemplateBuildFileUpload struct {
	Present bool
	URL     *string
}

// TemplateAliasResponse contains alias/public metadata for a template.
type TemplateAliasResponse struct {
	TemplateID string
	Public     bool
}

// AssignedTemplateTags reports tags assigned to a template build.
type AssignedTemplateTags struct {
	BuildID string
	Tags    []string
}

// TemplateTagInfo describes a single tag attached to a template build.
type TemplateTagInfo struct {
	// BuildID is the build the tag points to.
	BuildID string
	// Tag is the tag name.
	Tag string
	// CreatedAt is the time the tag was assigned.
	CreatedAt time.Time
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------

func templateFromAPI(a apis.Template) Template {
	aliases := []string(nil)
	if a.Aliases != nil {
		aliases = *a.Aliases
	}
	return Template{
		TemplateID:  stringValue(a.TemplateID),
		Aliases:     aliases,
		BuildID:     stringValue(a.BuildID),
		BuildStatus: TemplateBuildStatus(stringValue(a.BuildStatus)),
		CPUCount:    int32Value(a.CPUCount),
		MemoryMB:    int32Value(a.MemoryMb),
		DiskSizeMB:  int32Value(a.DiskSizeMb),
		EnvdVersion: stringValue(a.EnvdVersion),
		Public:      boolValue(a.Public),
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

func templateWithBuildsFromAPI(a *apis.TemplateWithBuilds) *TemplateWithBuilds {
	if a == nil {
		return nil
	}
	result := &TemplateWithBuilds{
		TemplateID: stringValue(a.TemplateID),
		Public:     boolValue(a.Public),
		CreatedAt:  timeValue(a.CreatedAt),
		UpdatedAt:  timeValue(a.UpdatedAt),
		Builds:     []TemplateBuild{},
	}
	if a.Aliases != nil {
		result.Aliases = *a.Aliases
	}
	if a.BuildID != nil || a.BuildStatus != nil {
		result.Builds = append(result.Builds, TemplateBuild{
			BuildID:     stringValue(a.BuildID),
			Status:      TemplateBuildStatus(stringValue(a.BuildStatus)),
			CPUCount:    int32Value(a.CPUCount),
			MemoryMB:    int32Value(a.MemoryMb),
			DiskSizeMB:  a.DiskSizeMb,
			EnvdVersion: a.EnvdVersion,
			CreatedAt:   timeValue(a.CreatedAt),
			UpdatedAt:   timeValue(a.UpdatedAt),
		})
	}
	return result
}

func templateBuildInfoFromAPI(a *apis.TemplateBuildInfo) *TemplateBuildInfo {
	if a == nil {
		return nil
	}
	return &TemplateBuildInfo{
		TemplateID: stringValue(a.TemplateID),
		BuildID:    stringValue(a.BuildID),
		Status:     TemplateBuildStatus(stringValue(a.Status)),
	}
}

func templateBuildLogsFromAPI(a *apis.TemplateBuildLogsResponse) *TemplateBuildLogs {
	if a == nil {
		return nil
	}
	result := &TemplateBuildLogs{Logs: []BuildLogEntry{}}
	if a.Logs != nil {
		for _, e := range *a.Logs {
			result.Logs = append(result.Logs, BuildLogEntry{
				Level:     LogLevel(stringValue(e.Level)),
				Message:   stringValue(e.Message),
				Step:      e.Source,
				Timestamp: parseTime(stringValue(e.Timestamp)),
			})
		}
	}
	return result
}

func templateCreateResponseFromAPI(a *apis.TemplateRequestResponseV3) *TemplateCreateResponse {
	if a == nil {
		return nil
	}
	return &TemplateCreateResponse{
		TemplateID: stringValue(a.TemplateID),
		BuildID:    stringValue(a.BuildID),
		Public:     boolValue(a.Public),
	}
}
