package codex

type Config struct {
	Model                 string               `toml:"model"`
	ModelProvider         string               `toml:"model_provider"`
	ApprovalPolicy        *ApprovalPolicy      `toml:"approval_policy"`
	ForcedLoginMethod     ForcedLoginKind      `toml:"forced_login_method"`
	ModelReasoningEffort  ReasoningEffort      `toml:"model_reasoning_effort"`
	WebSearch             WebSearchKind        `toml:"web_search"`
	ShowRawAgentReasoning bool                 `toml:"show_raw_agent_reasoning"`
	ModelCatalogJson      string               `toml:"model_catalog_json"`
	SandBoxMode           SandBoxModeKind      `toml:"sandbox_mode"`
	ModelProviders        map[string]Model     `toml:"model_providers"`
	McpServers            map[string]McpServer `toml:"mcp_servers"`
}

type ApprovalPolicy struct {
	Granular ApprovalGranular `toml:"granular"`
}

type ApprovalGranular struct {
	SandboxApproval    bool `toml:"sandbox_approval"`
	Rules              bool `toml:"rules"`
	McpElicitations    bool `toml:"mcp_elicitations"`
	RequestPermissions bool `toml:"request_permissions"`
	SkillApproval      bool `toml:"skill_approval"`
}

type Model struct {
	Name                    string       `toml:"name"`
	BaseUrl                 string       `toml:"base_url"`
	WireApi                 ModelApiKind `toml:"wire_api"`
	EnvKey                  string       `toml:"env_key"`
	ExperimentalBearerToken string       `toml:"experimental_bearer_token"`
}

type McpServer struct {
	Url               string            `toml:"url"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var"`
	HttpHeaders       map[string]string `toml:"http_headers"`
}

type ForcedLoginKind string

const (
	LoginChatGPT ForcedLoginKind = "chatgpt"
	LoginApi     ForcedLoginKind = "api"
)

type ReasoningEffort string

const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
	ReasoningEffortXHigh  ReasoningEffort = "xhigh"
	ReasoningEffortMax    ReasoningEffort = "max"
	ReasoningEffortUltra  ReasoningEffort = "ultra"
)

type WebSearchKind string

const (
	WebSearchDisabled WebSearchKind = "disabled"
	WebSearchCached   WebSearchKind = "cached"
	WebSearchIndexed  WebSearchKind = "indexed"
	WebSearchLive     WebSearchKind = "live"
)

type SandBoxModeKind string

const (
	SandBoxReadOnly       SandBoxModeKind = "read-only"
	SandBoxWorkSpaceWrite SandBoxModeKind = "workspace-write"
	SandBoxAll            SandBoxModeKind = "danger-full-access"
)

type ModelApiKind string

const (
	ModelApiResponses ModelApiKind = "responses"
)

type TurnType string

const (
	UserMessage         TurnType = "userMessage"
	HookPrompt          TurnType = "hookPrompt"
	AgentMessage        TurnType = "agentMessage"
	FunctionCallOutput  TurnType = "functionCallOutput"
	Plan                TurnType = "plan"
	Reasoning           TurnType = "reasoning"
	CommandExecution    TurnType = "commandExecution"
	FileChange          TurnType = "fileChange"
	McpToolCall         TurnType = "mcpToolCall"
	DynamicToolCall     TurnType = "dynamicToolCall"
	CollabAgentToolCall TurnType = "collabAgentToolCall"
	SubAgentActivity    TurnType = "subAgentActivity"
	WebSearch           TurnType = "webSearch"
	ImageView           TurnType = "imageView"
	Sleep               TurnType = "sleep"
	ImageGeneration     TurnType = "imageGeneration"
	EnteredReviewMode   TurnType = "enteredReviewMode"
	ExitedReviewMode    TurnType = "exitedReviewMode"
	ContextCompaction   TurnType = "contextCompaction"
)

type SummaryItem struct {
	ID   string   `json:"id"`
	Type TurnType `json:"type"`

	// userMessage
	Content []*TextUserInput `json:"content"`

	// agentMessage
	Text string `json:"text"`

	// optional
	CompletedAtMs int64 `json:"completedAtMs"`
}

type TextUserInput struct {
	Text string `json:"text"`
	Type string `json:"type"`
}
