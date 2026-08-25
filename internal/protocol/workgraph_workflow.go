// INPUT: 系统内置结构模板、当前/历史 managed Execution 经模型抽取的结构草图、不可变编辑版本与隔离后台保存请求。
// OUTPUT: 可恢复 Draft、可选择的版本、Slash 名称可用性、只读内置或 owner 保存的命名 WorkGraph；保存 capability 不携带运行事实。
// POS: WorkGraph 模板、提取、查询、对话编辑、版本选择、独立内部 Session 保存、Slash 目录和 runtime prompt 展开共用的跨边界协议。
package protocol

import "time"

// WorkGraphWorkflowNodeRole 表示模板节点在复用流程中的语义权重。
type WorkGraphWorkflowNodeRole string

const (
	WorkGraphWorkflowNodeKey           WorkGraphWorkflowNodeRole = "key"
	WorkGraphWorkflowNodeCollaboration WorkGraphWorkflowNodeRole = "collaboration"
)

// WorkGraphWorkflow 是从实际完成图抽象并由用户确认保存的可复用命名工作图。
// Source* 只保留 provenance；运行身份和结果事实永不进入模板。
type WorkGraphWorkflow struct {
	ID                 string                        `json:"id"`
	OwnerUserID        string                        `json:"-"`
	BuiltIn            bool                          `json:"built_in,omitempty"`
	SlashName          string                        `json:"slash_name"`
	Title              string                        `json:"title"`
	Description        string                        `json:"description,omitempty"`
	SourceExecutionID  string                        `json:"source_execution_id"`
	SourceSessionKey   string                        `json:"source_session_key"`
	Objective          string                        `json:"objective"`
	CompletionCriteria []string                      `json:"completion_criteria,omitempty"`
	Nodes              []WorkGraphWorkflowNode       `json:"nodes"`
	Dependencies       []WorkGraphWorkflowDependency `json:"dependencies,omitempty"`
	Version            int64                         `json:"version"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

// WorkGraphWorkflowNode 只保存重新创建 Work Item 所需的语义契约。
type WorkGraphWorkflowNode struct {
	WorkflowID         string                    `json:"-"`
	LogicalKey         string                    `json:"logical_key"`
	SourceWorkItemID   string                    `json:"source_work_item_id,omitempty"`
	Role               WorkGraphWorkflowNodeRole `json:"role"`
	Kind               WorkItemKind              `json:"kind"`
	Subject            string                    `json:"subject"`
	Objective          string                    `json:"objective"`
	Deliverable        string                    `json:"deliverable"`
	AcceptanceCriteria []string                  `json:"acceptance_criteria,omitempty"`
	Required           bool                      `json:"required"`
	Terminal           bool                      `json:"terminal,omitempty"`
	ParentLogicalKey   string                    `json:"parent_logical_key,omitempty"`
	Position           int                       `json:"position"`
}

// WorkGraphWorkflowDependency 保存模板节点之间的显式 DAG 边。
type WorkGraphWorkflowDependency struct {
	WorkflowID          string             `json:"-"`
	LogicalKey          string             `json:"logical_key"`
	DependsOnLogicalKey string             `json:"depends_on_logical_key"`
	Kind                WorkDependencyKind `json:"kind"`
}

// WorkGraphWorkflowPreview 是已进入 durable Draft、但尚未进入 Slash 目录的一条完整抽象草图版本。
// 用户可在普通对话或隐藏专用 DM 中修订元信息、节点与依赖；保存前仍须通过完整结构校验。
type WorkGraphWorkflowPreview struct {
	PreviewID          string                        `json:"preview_id"`
	SlashName          string                        `json:"slash_name"`
	Title              string                        `json:"title"`
	Description        string                        `json:"description,omitempty"`
	SourceExecutionID  string                        `json:"source_execution_id"`
	SourceSessionKey   string                        `json:"source_session_key"`
	Objective          string                        `json:"objective"`
	CompletionCriteria []string                      `json:"completion_criteria,omitempty"`
	Nodes              []WorkGraphWorkflowNode       `json:"nodes"`
	Dependencies       []WorkGraphWorkflowDependency `json:"dependencies,omitempty"`
	ExpiresAt          time.Time                     `json:"expires_at"`
}

// WorkGraphWorkflowPreviewVersion 是一条不可变草图版本；Preview 始终是该版本的完整结构快照。
type WorkGraphWorkflowPreviewVersion struct {
	Revision  int64                    `json:"revision"`
	Preview   WorkGraphWorkflowPreview `json:"preview"`
	CreatedAt time.Time                `json:"created_at"`
}

// WorkGraphWorkflowPreviewVersionSummary 供编辑器展示版本选择，不重复传输每一版完整图。
type WorkGraphWorkflowPreviewVersionSummary struct {
	Revision        int64     `json:"revision"`
	SlashName       string    `json:"slash_name"`
	Title           string    `json:"title"`
	NodeCount       int       `json:"node_count"`
	DependencyCount int       `json:"dependency_count"`
	Selected        bool      `json:"selected"`
	CreatedAt       time.Time `json:"created_at"`
}

// WorkGraphWorkflowDraft 是按 source Execution 唯一复用、可恢复且拥有不可变版本历史的草图聚合。
// Editor Session identity 由宿主持有，不进入普通 Agent DM 目录。
type WorkGraphWorkflowDraft struct {
	PreviewID            string                            `json:"preview_id"`
	OwnerUserID          string                            `json:"-"`
	SourceExecutionID    string                            `json:"source_execution_id"`
	SourceSessionKey     string                            `json:"source_session_key"`
	SourceAgentID        string                            `json:"-"`
	SourceConversationID string                            `json:"-"`
	OutputLanguage       string                            `json:"output_language"`
	HeadRevision         int64                             `json:"head_revision"`
	SelectedRevision     int64                             `json:"selected_revision"`
	EditorID             string                            `json:"editor_id,omitempty"`
	EditorAgentID        string                            `json:"editor_agent_id,omitempty"`
	EditorSessionKey     string                            `json:"editor_session_key,omitempty"`
	EditorDisplayAfter   int64                             `json:"editor_display_after_unix_milli,omitempty"`
	Preview              WorkGraphWorkflowPreview          `json:"preview"`
	Versions             []WorkGraphWorkflowPreviewVersion `json:"-"`
	SaveScheduled        bool                              `json:"save_scheduled"`
	SavedWorkflowID      string                            `json:"saved_workflow_id,omitempty"`
	SavedRevision        int64                             `json:"saved_revision,omitempty"`
	ExpiresAt            time.Time                         `json:"expires_at"`
	CreatedAt            time.Time                         `json:"created_at"`
	UpdatedAt            time.Time                         `json:"updated_at"`
}

// WorkGraphWorkflowDraftSummary 是模型与 UI 查询一个会话中多张草图时使用的紧凑目录项。
type WorkGraphWorkflowDraftSummary struct {
	PreviewID         string    `json:"preview_id"`
	SourceExecutionID string    `json:"source_execution_id"`
	SlashName         string    `json:"slash_name"`
	Title             string    `json:"title"`
	HeadRevision      int64     `json:"head_revision"`
	SelectedRevision  int64     `json:"selected_revision"`
	VersionCount      int       `json:"version_count"`
	NodeCount         int       `json:"node_count"`
	SaveScheduled     bool      `json:"save_scheduled"`
	SavedWorkflowID   string    `json:"saved_workflow_id,omitempty"`
	SavedRevision     int64     `json:"saved_revision,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// WorkGraphWorkflowSourceSummary 是当前 Session 可提取的 completed WorkGraph 候选。
type WorkGraphWorkflowSourceSummary struct {
	ExecutionID string          `json:"execution_id"`
	Status      ExecutionStatus `json:"status"`
	Objective   string          `json:"objective"`
	NodeCount   int             `json:"node_count"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// WorkGraphWorkflowLibrary 把一个 Session 的来源图、Draft、只读内置模板与 owner 命名图统一投影给 Skill。
type WorkGraphWorkflowLibrary struct {
	Sources   []WorkGraphWorkflowSourceSummary `json:"sources"`
	Drafts    []WorkGraphWorkflowDraftSummary  `json:"drafts"`
	Workflows []WorkGraphWorkflow              `json:"workflows"`
}

// PreviewWorkGraphWorkflowRequest 请求从一张 exact 完成图生成或复用 durable Draft。
type PreviewWorkGraphWorkflowRequest struct {
	SourceSessionKey  string `json:"source_session_key"`
	SourceExecutionID string `json:"source_execution_id"`
	OutputLanguage    string `json:"output_language,omitempty"`
}

// PreviewSavedWorkGraphWorkflowRequest 请求从命名图恢复其关联 Draft 以继续编辑。
type PreviewSavedWorkGraphWorkflowRequest struct {
	OutputLanguage string `json:"output_language,omitempty"`
}

// SaveWorkGraphWorkflowRequest 通过受管 CLI 保存用户确认过的 exact 草图。
type SaveWorkGraphWorkflowRequest struct {
	CommandID        string `json:"-"`
	SourceSessionKey string `json:"source_session_key"`
	PreviewID        string `json:"preview_id"`
}

// ScheduleWorkGraphWorkflowSaveRequest 请求宿主启动不进入聊天时间线的内部 Agent round。
type ScheduleWorkGraphWorkflowSaveRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	PreviewID        string `json:"preview_id"`
	SlashName        string `json:"slash_name,omitempty"`
	Title            string `json:"title,omitempty"`
	Description      string `json:"description,omitempty"`
}

// WorkGraphWorkflowSaveReceipt 表示 exact preview 已交给后台 Agent；它不表示 CLI 已经落库。
type WorkGraphWorkflowSaveReceipt struct {
	PreviewID string `json:"preview_id"`
	Status    string `json:"status"`
}

// WorkGraphWorkflowSlashNameAvailability 表示 owner scope 中一个 canonical Slash 名称能否用于 exact Draft。
type WorkGraphWorkflowSlashNameAvailability struct {
	SlashName string `json:"slash_name"`
	Available bool   `json:"available"`
}

// WorkGraphWorkflowEditorSession 是由 Nexus 主智能体承载、不会进入普通会话目录的隐藏专用 DM。
type WorkGraphWorkflowEditorSession struct {
	EditorID              string                                   `json:"editor_id"`
	Revision              int64                                    `json:"revision"`
	SelectedRevision      int64                                    `json:"selected_revision"`
	AgentID               string                                   `json:"agent_id"`
	SessionKey            string                                   `json:"session_key"`
	DisplayAfterUnixMilli int64                                    `json:"display_after_unix_milli"`
	Preview               WorkGraphWorkflowPreview                 `json:"preview"`
	Versions              []WorkGraphWorkflowPreviewVersionSummary `json:"versions"`
	ExpiresAt             time.Time                                `json:"expires_at"`
}

// StartWorkGraphWorkflowEditorRequest 创建或恢复 owner/source-session-scoped 隐藏编辑会话。
type StartWorkGraphWorkflowEditorRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	PreviewID        string `json:"preview_id"`
	OutputLanguage   string `json:"output_language,omitempty"`
	SlashName        string `json:"slash_name,omitempty"`
	Title            string `json:"title,omitempty"`
	Description      string `json:"description,omitempty"`
}

// GetWorkGraphWorkflowEditorRequest 读取精确隐藏编辑会话的当前草图与版本目录。
type GetWorkGraphWorkflowEditorRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	EditorID         string `json:"editor_id"`
}

// ApplyWorkGraphWorkflowEditorRequest 把隐藏会话的 exact 选中版本投影到当前 preview。
type ApplyWorkGraphWorkflowEditorRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	EditorID         string `json:"editor_id"`
	Revision         int64  `json:"revision"`
}

// SelectWorkGraphWorkflowEditorVersionRequest 选择一条已有版本作为当前编辑基线；不改写历史。
type SelectWorkGraphWorkflowEditorVersionRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	EditorID         string `json:"editor_id"`
	Revision         int64  `json:"revision"`
	SelectedRevision int64  `json:"selected_revision"`
}

// ReviseWorkGraphWorkflowDraftRequest 允许普通对话中的 Skill 按 exact preview 修改草图。
type ReviseWorkGraphWorkflowDraftRequest struct {
	PreviewID string `json:"preview_id"`
	ReviseWorkGraphWorkflowPreviewRequest
}

// ReviseWorkGraphWorkflowPreviewRequest 是编辑 Agent 通过受限 Execution CLI 一次性提交的完整草图版本。
type ReviseWorkGraphWorkflowPreviewRequest struct {
	Revision           int64                         `json:"revision"`
	SlashName          string                        `json:"slash_name"`
	Title              string                        `json:"title"`
	Description        string                        `json:"description"`
	Objective          string                        `json:"objective"`
	CompletionCriteria []string                      `json:"completion_criteria,omitempty"`
	Nodes              []WorkGraphWorkflowNode       `json:"nodes"`
	Dependencies       []WorkGraphWorkflowDependency `json:"dependencies,omitempty"`
}
