package pluginapi

// HookPhase is the lifecycle phase where a plugin is invoked.
type HookPhase string

const (
	HookCollect    HookPhase = "collect"
	HookValidate   HookPhase = "validate"
	HookEnrich     HookPhase = "enrich"
	HookFinalize   HookPhase = "finalize"
	HookPreCommit  HookPhase = "pre_commit"
	HookPostCommit HookPhase = "post_commit"
)

// FailureMode configures behavior when plugin execution fails.
type FailureMode string

const (
	FailClosed FailureMode = "fail_closed"
	FailOpen   FailureMode = "fail_open"
)

// ProtocolVersionV1 is the current JSON protocol version.
const ProtocolVersionV1 = "1.0"

// Trailer is a commit message trailer key/value pair.
type Trailer struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CommitDraft is the mutable commit payload exchanged with plugins.
type CommitDraft struct {
	Title    string                 `json:"title"`
	Body     string                 `json:"body"`
	Trailers []Trailer              `json:"trailers"`
	Metadata map[string]interface{} `json:"metadata"`
}

// RequestContext carries repository and runtime metadata.
type RequestContext struct {
	RepoRoot     string   `json:"repo_root"`
	Branch       string   `json:"branch,omitempty"`
	Head         string   `json:"head,omitempty"`
	StagedFiles  []string `json:"staged_files"`
	GitUserName  string   `json:"git_user_name,omitempty"`
	GitUserEmail string   `json:"git_user_email,omitempty"`
	TimestampUTC string   `json:"timestamp_utc,omitempty"`
}

// Request is the JSON input sent to plugin processes.
type Request struct {
	ProtocolVersion string                 `json:"protocol_version"`
	RequestID       string                 `json:"request_id"`
	PluginID        string                 `json:"plugin_id"`
	Hook            HookPhase              `json:"hook"`
	PluginConfig    map[string]interface{} `json:"plugin_config,omitempty"`
	Context         RequestContext         `json:"context"`
	Draft           CommitDraft            `json:"draft"`
	Answers         map[string]interface{} `json:"answers,omitempty"`
}

// Diagnostic is a plugin-emitted message.
type Diagnostic struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Path    string `json:"path,omitempty"`
}

// PromptOption is an option for select/multiselect prompts.
type PromptOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// PromptRequest asks core to collect additional user input.
type PromptRequest struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Options     []PromptOption `json:"options,omitempty"`
}

// UIOption is an option for UI select fields.
type UIOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// UIField is a declarative field definition rendered by core.
type UIField struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Required    bool       `json:"required,omitempty"`
	CharLimit   int        `json:"char_limit,omitempty"`
	Placeholder string     `json:"placeholder,omitempty"`
	Value       string     `json:"value,omitempty"`
	Options     []UIOption `json:"options,omitempty"`
	Editor      bool       `json:"editor,omitempty"`
}

// UIRequest is a declarative form request rendered by core with huh.
type UIRequest struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	SubmitLabel string    `json:"submit_label,omitempty"`
	Fields      []UIField `json:"fields"`
}

// Mutations contains draft mutations requested by a plugin.
type Mutations struct {
	SetTitle      string                 `json:"set_title,omitempty"`
	SetBody       string                 `json:"set_body,omitempty"`
	AppendBody    string                 `json:"append_body,omitempty"`
	PrependBody   string                 `json:"prepend_body,omitempty"`
	AddTrailers   []Trailer              `json:"add_trailers,omitempty"`
	MetadataPatch map[string]interface{} `json:"metadata_patch,omitempty"`
}

// Response is the JSON output returned by plugin processes.
type Response struct {
	RequestID      string          `json:"request_id"`
	OK             bool            `json:"ok"`
	Fatal          bool            `json:"fatal,omitempty"`
	BlockCommit    bool            `json:"block_commit,omitempty"`
	BlockReason    string          `json:"block_reason,omitempty"`
	Diagnostics    []Diagnostic    `json:"diagnostics"`
	Mutations      *Mutations      `json:"mutations,omitempty"`
	PromptRequests []PromptRequest `json:"prompt_requests,omitempty"`
	UIRequests     []UIRequest     `json:"ui_requests,omitempty"`
}

// EntryPoint describes how a plugin executable is launched.
type EntryPoint struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Permissions declares required plugin capabilities.
type Permissions struct {
	Network         bool     `json:"network"`
	GitRead         bool     `json:"git_read"`
	GitWrite        bool     `json:"git_write"`
	FilesystemRead  []string `json:"filesystem_read,omitempty"`
	FilesystemWrite []string `json:"filesystem_write,omitempty"`
	Secrets         []string `json:"secrets,omitempty"`
}

// AIAnswerSpec describes how an AI agent should fill one answer key.
type AIAnswerSpec struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Type        string `json:"type,omitempty"` // string, bool, []string
	Required    bool   `json:"required,omitempty"`
	Strategy    string `json:"strategy,omitempty"` // e.g. infer_from_diff, infer_from_context, static
}

// MetadataSpec describes metadata read/write semantics.
type MetadataSpec struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PluginContract describes stable plugin IO contract.
type PluginContract struct {
	Answers        []AIAnswerSpec `json:"answers,omitempty"`
	MetadataReads  []MetadataSpec `json:"metadata_reads,omitempty"`
	MetadataWrites []MetadataSpec `json:"metadata_writes,omitempty"`
	TrailerWrites  []MetadataSpec `json:"trailer_writes,omitempty"`
}

// AIHints documents how an AI agent should invoke and satisfy a plugin.
type AIHints struct {
	Purpose      string `json:"purpose"`
	Instructions string `json:"instructions,omitempty"`
}

// Manifest is the canonical plugin manifest.
type Manifest struct {
	APIVersion       string                 `json:"api_version"`
	Kind             string                 `json:"kind"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name,omitempty"`
	Version          string                 `json:"version"`
	Description      string                 `json:"description,omitempty"`
	Homepage         string                 `json:"homepage,omitempty"`
	Repository       string                 `json:"repository,omitempty"`
	License          string                 `json:"license,omitempty"`
	ProtocolVersions []string               `json:"protocol_versions,omitempty"`
	Entrypoint       EntryPoint             `json:"entrypoint"`
	Hooks            []HookPhase            `json:"hooks"`
	Permissions      Permissions            `json:"permissions,omitempty"`
	Contract         *PluginContract        `json:"contract,omitempty"`
	AIHints          *AIHints               `json:"ai_hints,omitempty"`
	ConfigSchema     map[string]interface{} `json:"config_schema,omitempty"`
}
