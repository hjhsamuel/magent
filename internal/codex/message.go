package codex

import (
	"encoding/json"
	"fmt"
)

type Method string

const (
	MethodInitialize  Method = "initialize"
	MethodInitialized Method = "initialized"

	MethodThreadStart         Method = "thread/start"
	MethodThreadStarted       Method = "thread/started"
	MethodThreadResume        Method = "thread/resume"
	MethodThreadRead          Method = "thread/read"
	MethodThreadDelete        Method = "thread/delete"
	MethodThreadTurnsList     Method = "thread/turns/list"
	MethodThreadStatusChanged Method = "thread/status/changed"

	MethodTurnStart     Method = "turn/start"
	MethodTurnStarted   Method = "turn/started"
	MethodTurnInterrupt Method = "turn/interrupt"
	MethodTurnCompleted Method = "turn/completed"

	MethodAgentMessageDelta     Method = "item/agentMessage/delta"
	MethodReasoningDelta        Method = "item/reasoning/textDelta"
	MethodReasoningSummaryDelta Method = "item/reasoning/summaryTextDelta"

	MethodItemStarted   Method = "item/started"
	MethodItemCompleted Method = "item/completed"

	MethodCommandExecutionApproval Method = "item/commandExecution/requestApproval"
	MethodFileChangeApproval       Method = "item/fileChange/requestApproval"
	MethodPermissionsApproval      Method = "item/permissions/requestApproval"
	MethodMcpServerElicitation     Method = "mcpServer/elicitation/request"
	MethodServerRequestResolved    Method = "serverRequest/resolved"

	MethodError Method = "error"

	MethodListSkills       Method = "skills/list"
	MethodReloadMCPServers Method = "config/mcpServer/reload"
)

type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method Method          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type InitializeParams struct {
	ClientInfo   ClientInfo              `json:"clientInfo"`
	Capabilities *InitializeCapabilities `json:"capabilities,omitempty"`
}

type InitializeResponse struct {
	UserAgent      string `json:"userAgent"`
	PlatformFamily string `json:"platformFamily"`
	PlatformOS     string `json:"platformOs"`
}

type InitializedParams struct{}

type ListSkillsParams struct {
	ForceReload bool `json:"forceReload"`
}

type ThreadStartParams struct{}

type ThreadResumeParams struct {
	ThreadID     string `json:"threadId"`
	ExcludeTurns bool   `json:"excludeTurns,omitempty"`
}

type ThreadResponse struct {
	Thread Thread `json:"thread"`
}

type Thread struct {
	ID            string       `json:"id"`
	SessionID     string       `json:"sessionId,omitempty"`
	Name          *string      `json:"name,omitempty"`
	Preview       string       `json:"preview,omitempty"`
	Cwd           string       `json:"cwd,omitempty"`
	ModelProvider string       `json:"modelProvider,omitempty"`
	CreatedAt     int64        `json:"createdAt,omitempty"`
	UpdatedAt     int64        `json:"updatedAt,omitempty"`
	Ephemeral     bool         `json:"ephemeral"`
	Status        ThreadStatus `json:"status"`
	Turns         []Turn       `json:"turns,omitempty"`
}

// ThreadStatusType is the app-server thread lifecycle state.
// Unknown future values remain decodable for protocol compatibility.
type ThreadStatusType string

const (
	ThreadStatusNotLoaded   ThreadStatusType = "notLoaded"
	ThreadStatusIdle        ThreadStatusType = "idle"
	ThreadStatusSystemError ThreadStatusType = "systemError"
	ThreadStatusActive      ThreadStatusType = "active"
)

// ThreadActiveFlag identifies why an active thread is waiting.
// Unknown future values remain decodable for protocol compatibility.
type ThreadActiveFlag string

const (
	ThreadActiveFlagWaitingOnApproval  ThreadActiveFlag = "waitingOnApproval"
	ThreadActiveFlagWaitingOnUserInput ThreadActiveFlag = "waitingOnUserInput"
)

type ThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type ThreadStartedNotification struct {
	Thread Thread `json:"thread"`
}

type ThreadStatusChangedNotification struct {
	ThreadID string       `json:"threadId"`
	Status   ThreadStatus `json:"status"`
}

type TextInput struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type TurnStartParams struct {
	ThreadID string      `json:"threadId"`
	Input    []TextInput `json:"input"`
}

type TurnStartResponse struct {
	Turn Turn `json:"turn"`
}

type Turn struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Items  []json.RawMessage `json:"items"`
	Error  *TurnError        `json:"error"`
}

type TurnError struct {
	Message           string          `json:"message"`
	CodexErrorInfo    json.RawMessage `json:"codexErrorInfo"`
	AdditionalDetails *string         `json:"additionalDetails"`
}

type TurnInterruptParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type TurnInterruptResponse struct{}

type TurnNotification struct {
	ThreadID string `json:"threadId"`
	Turn     Turn   `json:"turn"`
}

type AgentMessageDeltaNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	ItemID   string `json:"itemId"`
	Delta    string `json:"delta"`
}

type ApprovalDecision string

const (
	ApprovalAccept           ApprovalDecision = "accept"
	ApprovalAcceptForSession ApprovalDecision = "acceptForSession"
	ApprovalDecline          ApprovalDecision = "decline"
	ApprovalCancel           ApprovalDecision = "cancel"
)

type ApprovalResponse struct {
	Decision ApprovalDecision `json:"decision"`
}

type CommandExecutionApprovalParams struct {
	ThreadID                    string                  `json:"threadId"`
	TurnID                      string                  `json:"turnId"`
	ItemID                      string                  `json:"itemId"`
	Reason                      *string                 `json:"reason,omitempty"`
	Command                     *string                 `json:"command,omitempty"`
	Cwd                         *string                 `json:"cwd,omitempty"`
	CommandActions              []json.RawMessage       `json:"commandActions,omitempty"`
	ProposedExecpolicyAmendment []string                `json:"proposedExecpolicyAmendment,omitempty"`
	AvailableDecisions          []json.RawMessage       `json:"availableDecisions,omitempty"`
	NetworkApprovalContext      *NetworkApprovalContext `json:"networkApprovalContext,omitempty"`
}

type NetworkApprovalContext struct {
	Host     string `json:"host"`
	Protocol string `json:"protocol"`
}

type FileChangeApprovalParams struct {
	ThreadID  string  `json:"threadId"`
	TurnID    string  `json:"turnId"`
	ItemID    string  `json:"itemId"`
	Reason    *string `json:"reason,omitempty"`
	GrantRoot *string `json:"grantRoot,omitempty"`
}

// InitializeCapabilities controls which auxiliary notifications the server emits.
type InitializeCapabilities struct {
	OptOutNotificationMethods []Method `json:"optOutNotificationMethods,omitempty"`
}

// Exact matches only: never suppress whole namespaces containing actionable events.
// Return a fresh slice so initialization cannot mutate the filtering policy.
func ignoredNotificationMethods() []Method {
	return []Method{
		MethodThreadStatusChanged,
		"thread/tokenUsage/updated",
		"account/rateLimits/updated",
		"account/rateLimits/updates",    // Compatibility with the alternate spelling.
		"model/safetyBuffering/updated", // Transient UI hint, not turn progress.
		"skills/changed",                // Catalog cache invalidation; Runtime has no skills cache.
		"thread/compacted",              // Deprecated; compaction is represented by item events.
		"item/fileChange/outputDelta",   // Deprecated; item completion carries changes.
	}
}

func ignoreNotification(m Message) bool {
	// Requests must be answered; a method name alone must never discard one.
	if len(m.ID) != 0 || m.Error != nil {
		return false
	}
	for _, method := range ignoredNotificationMethods() {
		if m.Method == method {
			return true
		}
	}
	return false
}

// Only explicitly identified text deltas may be lost under backpressure.
// Requests, errors, lifecycle events and unknown future methods must survive.
func droppableEvent(m Message) bool {
	if len(m.ID) != 0 || m.Error != nil {
		return false
	}
	switch m.Method {
	case MethodAgentMessageDelta, MethodReasoningDelta:
		return true
	default:
		return false
	}
}

type ServerRequestResolvedParams struct {
	ThreadID  string          `json:"threadId"`
	RequestID json.RawMessage `json:"requestId"`
}

type ThreadTurnsListParams struct {
	ThreadID      string `json:"threadId"`
	SortDirection string `json:"sortDirection"`
	ItemsView     string `json:"itemsView"`
	Limit         int    `json:"limit"`
}

type ThreadTurnsListResponse struct {
	Data []Turn `json:"data"`
}

type ThreadReadParams struct {
	ThreadID     string `json:"threadId"`
	IncludeTurns bool   `json:"includeTurns"`
}

type ThreadDeleteParams struct {
	ThreadID string `json:"threadId"`
}

type PermissionGrantScope string

const (
	PermissionScopeTurn    PermissionGrantScope = "turn"
	PermissionScopeSession PermissionGrantScope = "session"
)

type PermissionsRequestApprovalParams struct {
	ThreadID      string                   `json:"threadId"`
	TurnID        string                   `json:"turnId"`
	ItemID        string                   `json:"itemId"`
	EnvironmentID *string                  `json:"environmentId"`
	StartedAtMs   int64                    `json:"startedAtMs"`
	Cwd           string                   `json:"cwd"`
	Reason        *string                  `json:"reason"`
	Permissions   RequestPermissionProfile `json:"permissions"`
}

type RequestPermissionProfile struct {
	Network    *AdditionalNetworkPermissions    `json:"network"`
	FileSystem *AdditionalFileSystemPermissions `json:"fileSystem"`
}

type GrantedPermissionProfile struct {
	Network    *AdditionalNetworkPermissions    `json:"network,omitempty"`
	FileSystem *AdditionalFileSystemPermissions `json:"fileSystem,omitempty"`
}

type AdditionalNetworkPermissions struct {
	Enabled *bool `json:"enabled"`
}

type AdditionalFileSystemPermissions struct {
	Read             []string                 `json:"read"`
	Write            []string                 `json:"write"`
	GlobScanMaxDepth *uint32                  `json:"globScanMaxDepth,omitempty"`
	Entries          []FileSystemSandboxEntry `json:"entries,omitempty"`
}

type FileSystemAccessMode string

const (
	FileSystemAccessRead  FileSystemAccessMode = "read"
	FileSystemAccessWrite FileSystemAccessMode = "write"
	FileSystemAccessDeny  FileSystemAccessMode = "deny"
)

type FileSystemSandboxEntry struct {
	Path   FileSystemPath       `json:"path"`
	Access FileSystemAccessMode `json:"access"`
}

type FileSystemPath struct {
	Type    string          `json:"type"`
	Path    string          `json:"path,omitempty"`
	Pattern string          `json:"pattern,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
}

type PermissionsRequestApprovalResponse struct {
	Permissions      GrantedPermissionProfile `json:"permissions"`
	Scope            PermissionGrantScope     `json:"scope"`
	StrictAutoReview bool                     `json:"strictAutoReview,omitempty"`
}

type McpElicitationMode string

const (
	McpElicitationForm             McpElicitationMode = "form"
	McpElicitationOpenAIForm       McpElicitationMode = "openai/form"
	McpElicitationOpenAIFormLegacy McpElicitationMode = "openaiForm"
	McpElicitationURL              McpElicitationMode = "url"
)

type McpServerElicitationRequestParams struct {
	ThreadID        string             `json:"threadId"`
	TurnID          *string            `json:"turnId"`
	ServerName      string             `json:"serverName"`
	Mode            McpElicitationMode `json:"mode"`
	Meta            json.RawMessage    `json:"_meta"`
	Message         string             `json:"message"`
	RequestedSchema json.RawMessage    `json:"requestedSchema,omitempty"`
	URL             string             `json:"url,omitempty"`
	ElicitationID   string             `json:"elicitationId,omitempty"`
}
type McpElicitationAction string

const (
	McpElicitationAccept  McpElicitationAction = "accept"
	McpElicitationDecline McpElicitationAction = "decline"
	McpElicitationCancel  McpElicitationAction = "cancel"
)

type McpServerElicitationRequestResponse struct {
	Action  McpElicitationAction `json:"action"`
	Content json.RawMessage      `json:"content"`
	Meta    json.RawMessage      `json:"_meta"`
}
