// INPUT: Agent 资料、runtime 选项以及带可选期望版本的创建/更新请求。
// OUTPUT: 跨 HTTP/服务/运行时共享的 Agent 模型、受控工具快照与 runtime_version CAS 协议。
// POS: Agent 配置资源及其乐观并发令牌的协议真相源。
package protocol

import "time"

// DefaultAgentPermissionMode 是新用户与新 Agent 共用的产品默认权限。
const DefaultAgentPermissionMode = "acceptEdits"

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
	// SkillIDs 保存全局技能库中当前 Agent 已启用的引用；路径不进入协议。
	//
	// 平台和 ~/.agents/skills 使用 canonical name，用户导入来源使用
	// external:<name>。运行时副本或 Agent workspace 文件不能充当绑定状态。
	SkillIDs []string `json:"skill_ids,omitempty"`
	// DisabledSkillIDs 保存 Agent 明确停用的工作区动态 Skill 名称。
	//
	// 全局 Skill 未出现在 SkillIDs 即为停用；工作区 Skill 默认动态可见，
	// 只有显式停用时才进入此列表。
	DisabledSkillIDs []string `json:"disabled_skill_ids,omitempty"`
	SettingSources   []string `json:"setting_sources,omitempty"`
}

// RuntimeToolPolicy 是一次受控执行覆盖 Agent 工具配置的完整快照。
// nil 指针表示继续读取 Agent 当前配置；非 nil 的空切片表示创建快照时未设置规则。
type RuntimeToolPolicy struct {
	AllowedTools    []string `json:"allowed_tools"`
	DisallowedTools []string `json:"disallowed_tools"`
}

// Agent 表示对外 Agent 模型。
type Agent struct {
	AgentID         string  `json:"agent_id"`
	Name            string  `json:"name"`
	WorkspacePath   string  `json:"workspace_path"`
	IsMain          bool    `json:"is_main,omitempty"`
	DisplayName     string  `json:"display_name,omitempty"`
	Headline        string  `json:"headline,omitempty"`
	ProfileMarkdown string  `json:"profile_markdown,omitempty"`
	Options         Options `json:"options"`
	// RuntimeVersion 是只读的运行时配置版本。
	RuntimeVersion int64     `json:"runtime_version"`
	CreatedAt      time.Time `json:"created_at"`
	Status         string    `json:"status"`
	Avatar         string    `json:"avatar,omitempty"`
	Description    string    `json:"description,omitempty"`
	VibeTags       []string  `json:"vibe_tags,omitempty"`
	SkillsCount    int       `json:"skills_count"`
	OwnerUserID    string    `json:"-"`
}

// CreateRequest 表示创建 Agent 请求。
type CreateRequest struct {
	Name            string   `json:"name"`
	Options         *Options `json:"options,omitempty"`
	Avatar          string   `json:"avatar,omitempty"`
	Description     string   `json:"description,omitempty"`
	ProfileTemplate string   `json:"profile_template,omitempty"`
	VibeTags        []string `json:"vibe_tags,omitempty"`
}

// UpdateRequest 表示更新 Agent 请求。
type UpdateRequest struct {
	Name        *string  `json:"name,omitempty"`
	Options     *Options `json:"options,omitempty"`
	Avatar      *string  `json:"avatar,omitempty"`
	Description *string  `json:"description,omitempty"`
	VibeTags    []string `json:"vibe_tags,omitempty"`
	// ExpectedRuntimeVersion 可选；设置后仅在当前版本匹配时更新 Agent。
	ExpectedRuntimeVersion *int64 `json:"expected_runtime_version,omitempty"`
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

// ProfileTemplateResponse 表示新建 Agent 时可编辑的默认行为模板。
type ProfileTemplateResponse struct {
	Content string `json:"content"`
}
