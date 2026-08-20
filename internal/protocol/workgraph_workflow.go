// INPUT: 历史 managed Execution 中经用户选择的责任节点与协作语义。
// OUTPUT: 可跨 Session 调用的命名 WorkGraph Workflow；不携带 Tool、Attempt、Submission 或 Acceptance 事实。
// POS: 历史 WorkGraph 提炼、Slash 目录和 runtime prompt 展开共用的跨边界协议。
package protocol

import "time"

// WorkGraphWorkflowNodeRole 表示模板节点在复用流程中的语义权重。
type WorkGraphWorkflowNodeRole string

const (
	WorkGraphWorkflowNodeKey           WorkGraphWorkflowNodeRole = "key"
	WorkGraphWorkflowNodeCollaboration WorkGraphWorkflowNodeRole = "collaboration"
)

// WorkGraphWorkflow 是从历史责任图提炼出的可复用工作流。
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
	SourceWorkItemID   string                    `json:"source_work_item_id"`
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

// WorkGraphWorkflowNodeSelection 是历史图提炼请求中的显式节点选择。
type WorkGraphWorkflowNodeSelection struct {
	WorkItemID string                    `json:"work_item_id"`
	Role       WorkGraphWorkflowNodeRole `json:"role"`
}

// CreateWorkGraphWorkflowRequest 创建一个命名 Workflow Slash command。
type CreateWorkGraphWorkflowRequest struct {
	CommandID         string                           `json:"-"`
	SourceSessionKey  string                           `json:"source_session_key"`
	SourceExecutionID string                           `json:"source_execution_id"`
	SlashName         string                           `json:"slash_name"`
	Title             string                           `json:"title"`
	Description       string                           `json:"description,omitempty"`
	Nodes             []WorkGraphWorkflowNodeSelection `json:"nodes"`
}
