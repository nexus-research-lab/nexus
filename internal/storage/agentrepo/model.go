// INPUT: Agent 服务规范化后的完整创建值、领域创建 claim、更新值与可选预期 runtime 版本。
// OUTPUT: 跨方言 SQL 写入所需的创建回执、CreateRecord 与 UpdateRecord。
// POS: Agent 领域模型到规范化仓储列值的持久化边界。
package agentrepo

// CreationRequestStatus 表示领域回执的持久状态。
type CreationRequestStatus string
type CreationRequestStage string

const (
	CreationRequestPending   CreationRequestStatus = "pending"
	CreationRequestCommitted CreationRequestStatus = "committed"
	CreationRequestDeleted   CreationRequestStatus = "deleted"
	CreationRequestFailed    CreationRequestStatus = "failed"
)

const (
	CreationRequestReserved          CreationRequestStage = "reserved"
	CreationRequestWorkspacePrepared CreationRequestStage = "workspace_prepared"
)

// CreationRequestRecord 只保存幂等与对账所需的非敏感领域事实。
type CreationRequestRecord struct {
	OwnerUserID       string
	CreationRequestID string
	IntentDigest      string
	AgentID           string
	WorkspacePath     string
	Status            CreationRequestStatus
	Stage             CreationRequestStage
	ClaimToken        string
	LeaseExpiresAtMS  int64
	FailureCode       string
}

// CreateRecord 表示落库前的完整创建记录。
type CreateRecord struct {
	AgentID              string
	OwnerUserID          string
	Slug                 string
	Name                 string
	WorkspacePath        string
	Status               string
	IsMain               bool
	Avatar               string
	Description          string
	BusinessTagsJSON     string
	VibeTagsJSON         string
	DisplayName          string
	Headline             string
	ProfileMarkdown      string
	RuntimeID            string
	ProfileID            string
	Provider             string
	Model                string
	PermissionMode       string
	AllowedToolsJSON     string
	DisallowedToolsJSON  string
	MCPServersJSON       string
	ConnectorIDsJSON     string
	SkillIDsJSON         string
	DisabledSkillIDsJSON string
	MaxTurns             *int
	MaxThinkingTokens    *int
	SettingSourcesJSON   string
	RuntimeVersion       int64
}

// UpdateRecord 表示落库前的 Agent 更新记录。
type UpdateRecord struct {
	AgentID                string
	OwnerUserID            string
	Name                   string
	WorkspacePath          string
	Avatar                 string
	Description            string
	BusinessTagsJSON       string
	VibeTagsJSON           string
	Provider               string
	Model                  string
	PermissionMode         string
	AllowedToolsJSON       string
	DisallowedToolsJSON    string
	MCPServersJSON         string
	ConnectorIDsJSON       string
	SkillIDsJSON           string
	DisabledSkillIDsJSON   string
	MaxTurns               *int
	MaxThinkingTokens      *int
	SettingSourcesJSON     string
	ExpectedRuntimeVersion *int64
}
