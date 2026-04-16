// =====================================================
// @File   ：model.go
// @Date   ：2026/04/10 22:06:00
// @Author ：leemysw
// 2026/04/10 22:06:00   Create
// =====================================================

package agentdomain

import "time"

// Options 表示 Agent 运行时配置。
type Options struct {
	Provider          string         `json:"provider,omitempty"`
	Model             string         `json:"model,omitempty"`
	PermissionMode    string         `json:"permission_mode,omitempty"`
	AllowedTools      []string       `json:"allowed_tools,omitempty"`
	DisallowedTools   []string       `json:"disallowed_tools,omitempty"`
	MaxTurns          *int           `json:"max_turns,omitempty"`
	MaxThinkingTokens *int           `json:"max_thinking_tokens,omitempty"`
	MCPServers        map[string]any `json:"mcp_servers,omitempty"`
	SettingSources    []string       `json:"setting_sources,omitempty"`
}

// Agent 表示对外 Agent 模型。
type Agent struct {
	AgentID       string    `json:"agent_id"`
	Name          string    `json:"name"`
	WorkspacePath string    `json:"workspace_path"`
	Options       Options   `json:"options"`
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status"`
	Avatar        string    `json:"avatar,omitempty"`
	Description   string    `json:"description,omitempty"`
	VibeTags      []string  `json:"vibe_tags,omitempty"`
	SkillsCount   int       `json:"skills_count"`
}

// CreateRequest 表示创建 Agent 请求。
type CreateRequest struct {
	Name        string   `json:"name"`
	Options     *Options `json:"options,omitempty"`
	Avatar      string   `json:"avatar,omitempty"`
	Description string   `json:"description,omitempty"`
	VibeTags    []string `json:"vibe_tags,omitempty"`
}

// UpdateRequest 表示更新 Agent 请求。
type UpdateRequest struct {
	Name        *string  `json:"name,omitempty"`
	Options     *Options `json:"options,omitempty"`
	Avatar      *string  `json:"avatar,omitempty"`
	Description *string  `json:"description,omitempty"`
	VibeTags    []string `json:"vibe_tags,omitempty"`
}

// ValidateNameResponse 对齐当前校验协议。
type ValidateNameResponse struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
	IsValid        bool   `json:"is_valid"`
	IsAvailable    bool   `json:"is_available"`
	WorkspacePath  string `json:"workspace_path,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// CreateRecord 表示落库前的完整创建记录。
type CreateRecord struct {
	AgentID             string
	Slug                string
	Name                string
	WorkspacePath       string
	Status              string
	Avatar              string
	Description         string
	VibeTagsJSON        string
	DisplayName         string
	Headline            string
	ProfileMarkdown     string
	RuntimeID           string
	ProfileID           string
	Provider            string
	Model               string
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
	Slug                string
	Name                string
	WorkspacePath       string
	Avatar              string
	Description         string
	VibeTagsJSON        string
	Provider            string
	Model               string
	PermissionMode      string
	AllowedToolsJSON    string
	DisallowedToolsJSON string
	MCPServersJSON      string
	MaxTurns            *int
	MaxThinkingTokens   *int
	SettingSourcesJSON  string
}
