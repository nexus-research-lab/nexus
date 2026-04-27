package agentrepo

// CreateRecord 表示落库前的完整创建记录。
type CreateRecord struct {
	AgentID             string
	OwnerUserID         string
	Slug                string
	Name                string
	WorkspacePath       string
	Status              string
	IsMain              bool
	Avatar              string
	Description         string
	VibeTagsJSON        string
	DisplayName         string
	Headline            string
	ProfileMarkdown     string
	RuntimeID           string
	ProfileID           string
	Provider            string
	PermissionMode      string
	AllowedToolsJSON    string
	DisallowedToolsJSON string
	MCPServersJSON      string
	MaxTurns            *int
	MaxThinkingTokens   *int
	SettingSourcesJSON  string
	RuntimeVersion      int
}

// UpdateRecord 表示落库前的 Agent 更新记录。
type UpdateRecord struct {
	AgentID             string
	OwnerUserID         string
	Slug                string
	Name                string
	WorkspacePath       string
	Avatar              string
	Description         string
	VibeTagsJSON        string
	Provider            string
	PermissionMode      string
	AllowedToolsJSON    string
	DisallowedToolsJSON string
	MCPServersJSON      string
	MaxTurns            *int
	MaxThinkingTokens   *int
	SettingSourcesJSON  string
}
