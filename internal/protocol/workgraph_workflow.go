// INPUT: 当前/历史 managed Execution 经后台模型抽取的关键结构草图与后台保存请求。
// OUTPUT: 可预览或跨 Session 调用的命名 WorkGraph；不携带 Tool、Attempt、Submission 或 Acceptance 事实。
// POS: WorkGraph 草图预览、隐藏后台保存、Slash 目录和 runtime prompt 展开共用的跨边界协议。
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

// WorkGraphWorkflowPreview 是尚未持久化或进入 Slash 目录的抽象草图。
// 用户可在受限临时 DM 分支中要求模型修订元信息、节点与依赖；保存前仍须通过完整结构校验。
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

// PreviewWorkGraphWorkflowRequest 请求从一张 exact 完成图生成非持久化草图。
type PreviewWorkGraphWorkflowRequest struct {
	SourceSessionKey  string `json:"source_session_key"`
	SourceExecutionID string `json:"source_execution_id"`
	OutputLanguage    string `json:"output_language,omitempty"`
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

// WorkGraphWorkflowEditorSession 是从源 transcript fork、但不会进入普通会话目录的临时 DM 分支。
type WorkGraphWorkflowEditorSession struct {
	EditorID              string                   `json:"editor_id"`
	Revision              int64                    `json:"revision"`
	AgentID               string                   `json:"agent_id"`
	SessionKey            string                   `json:"session_key"`
	DisplayAfterUnixMilli int64                    `json:"display_after_unix_milli"`
	Preview               WorkGraphWorkflowPreview `json:"preview"`
	ExpiresAt             time.Time                `json:"expires_at"`
}

// StartWorkGraphWorkflowEditorRequest 创建 owner/source-session-scoped 临时编辑分支。
type StartWorkGraphWorkflowEditorRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	PreviewID        string `json:"preview_id"`
	OutputLanguage   string `json:"output_language,omitempty"`
	SlashName        string `json:"slash_name,omitempty"`
	Title            string `json:"title,omitempty"`
	Description      string `json:"description,omitempty"`
}

// GetWorkGraphWorkflowEditorRequest 读取精确临时分支的最新草图版本。
type GetWorkGraphWorkflowEditorRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	EditorID         string `json:"editor_id"`
}

// ApplyWorkGraphWorkflowEditorRequest 把临时分支的 exact revision 原子应用到原 preview。
type ApplyWorkGraphWorkflowEditorRequest struct {
	SourceSessionKey string `json:"source_session_key"`
	EditorID         string `json:"editor_id"`
	Revision         int64  `json:"revision"`
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
