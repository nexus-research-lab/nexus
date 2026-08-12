// INPUT: Bridge provider-neutral lifecycle event 与 Nexus round identity。
// OUTPUT: 可幂等持久化并与托管 WorkGraph 合并的 Runtime NodeRun / EdgeRun，以及 exact ToolRun 的结构化 Artifact 引用。
// POS: runtime 观测事实的协议模型；不承载 Plan、Assignment 或 Goal 生命周期。
package protocol

import "time"

const (
	// ExecutionRuntimeGraphNodeProjectionLimit 是 WorkGraph visibility 判定后的
	// 主图 runtime 节点窗口。它独立于模型执行契约的 32 项限制；只有应进入
	// 主图的节点超限时，公共 read model 才通过 total / truncated 明示。
	ExecutionRuntimeGraphNodeProjectionLimit = 256
	// ExecutionRuntimeGraphEdgeProjectionLimit 是 WorkGraph visibility 判定后的
	// 主图运行边窗口，包括 Agent 已经选择并实际发生的 retry / loop_back。
	ExecutionRuntimeGraphEdgeProjectionLimit = 512
	// ExecutionRuntimeGraphArtifactProjectionLimit 限制单个 Tool NodeRun 展示的
	// exact Artifact 数量；durable 引用仍可继续积累并在后续产品能力中分页读取。
	ExecutionRuntimeGraphArtifactProjectionLimit = 16
)

const (
	// ExecutionRuntimeMetadataWorkGraphVisibility 是 provider-neutral 的展示提示；
	// 它只能提升既有运行事实的可见层级，不能触发、路由或重试任何工作。
	ExecutionRuntimeMetadataWorkGraphVisibility = "workgraph_visibility"
)

// ExecutionRuntimeNodeKind 是 runtime 自动观测层的节点类型。
type ExecutionRuntimeNodeKind string

const (
	ExecutionRuntimeNodeAgent    ExecutionRuntimeNodeKind = "agent"
	ExecutionRuntimeNodeSubagent ExecutionRuntimeNodeKind = "subagent"
	ExecutionRuntimeNodeTool     ExecutionRuntimeNodeKind = "tool"
	ExecutionRuntimeNodeGate     ExecutionRuntimeNodeKind = "gate"
)

// ExecutionRuntimeNodeStatus 是 NodeRun 的单调运行状态。
type ExecutionRuntimeNodeStatus string

const (
	ExecutionRuntimeNodeRunning     ExecutionRuntimeNodeStatus = "running"
	ExecutionRuntimeNodeSucceeded   ExecutionRuntimeNodeStatus = "succeeded"
	ExecutionRuntimeNodeFailed      ExecutionRuntimeNodeStatus = "failed"
	ExecutionRuntimeNodeCancelled   ExecutionRuntimeNodeStatus = "cancelled"
	ExecutionRuntimeNodeInterrupted ExecutionRuntimeNodeStatus = "interrupted"
)

// ExecutionRuntimeEdgeKind 表示 runtime 自动观测或语义 Gate 形成的控制边。
type ExecutionRuntimeEdgeKind string

const (
	ExecutionRuntimeEdgeInvoke   ExecutionRuntimeEdgeKind = "invoke"
	ExecutionRuntimeEdgeSpawn    ExecutionRuntimeEdgeKind = "spawn"
	ExecutionRuntimeEdgeGuard    ExecutionRuntimeEdgeKind = "guard"
	ExecutionRuntimeEdgeLoopBack ExecutionRuntimeEdgeKind = "loop_back"
	// ExecutionRuntimeEdgeRetry 只表示当前 NodeRun 有 exact previous
	// run identity；它记录 Agent 已作出的选择，不授权或自动发起重试。
	ExecutionRuntimeEdgeRetry ExecutionRuntimeEdgeKind = "retry"
)

// ExecutionRuntimeNodeRun 是一次可恢复、可去重的运行节点事实。
type ExecutionRuntimeNodeRun struct {
	ID               string                       `json:"id"`
	GraphID          string                       `json:"graph_id"`
	OwnerUserID      string                       `json:"owner_user_id"`
	SessionKey       string                       `json:"session_key"`
	ExecutionID      string                       `json:"execution_id,omitempty"`
	Kind             ExecutionRuntimeNodeKind     `json:"kind"`
	SubjectID        string                       `json:"subject_id"`
	ParentSubjectID  string                       `json:"parent_subject_id,omitempty"`
	RootRoundID      string                       `json:"root_round_id"`
	RuntimeRoundID   string                       `json:"runtime_round_id"`
	AgentRoundID     string                       `json:"agent_round_id"`
	AgentID          string                       `json:"agent_id,omitempty"`
	Name             string                       `json:"name,omitempty"`
	Description      string                       `json:"description,omitempty"`
	Status           ExecutionRuntimeNodeStatus   `json:"status"`
	Failed           bool                         `json:"failed,omitempty"`
	ResultSummary    string                       `json:"result_summary,omitempty"`
	ErrorCode        string                       `json:"error_code,omitempty"`
	ErrorSummary     string                       `json:"error_summary,omitempty"`
	SummaryTruncated bool                         `json:"summary_truncated,omitempty"`
	DurationMS       int64                        `json:"duration_ms,omitempty"`
	StartedAt        time.Time                    `json:"started_at"`
	UpdatedAt        time.Time                    `json:"updated_at"`
	FinishedAt       *time.Time                   `json:"finished_at,omitempty"`
	Artifacts        []WorkspaceFileArtifactBlock `json:"artifacts,omitempty"`
	Metadata         map[string]any               `json:"metadata,omitempty"`
}

// ExecutionRuntimeEdgeRun 是两个已持久化 NodeRun 之间的运行边。
type ExecutionRuntimeEdgeRun struct {
	ID           string                   `json:"id"`
	GraphID      string                   `json:"graph_id"`
	OwnerUserID  string                   `json:"owner_user_id"`
	SessionKey   string                   `json:"session_key"`
	SourceNodeID string                   `json:"source_node_id"`
	TargetNodeID string                   `json:"target_node_id"`
	Kind         ExecutionRuntimeEdgeKind `json:"kind"`
	CreatedAt    time.Time                `json:"created_at"`
}

// ExecutionRuntimeArtifactRef 是 durable message 中 exact ToolUse 对应的结构化
// Artifact 事实。它独立于 Tool NodeRun 到达顺序持久化，读取时再按
// agent_round_id + tool_use_id 回挂。
type ExecutionRuntimeArtifactRef struct {
	ID           string                     `json:"id"`
	GraphID      string                     `json:"graph_id"`
	OwnerUserID  string                     `json:"owner_user_id"`
	SessionKey   string                     `json:"session_key"`
	ExecutionID  string                     `json:"execution_id,omitempty"`
	RootRoundID  string                     `json:"root_round_id"`
	AgentRoundID string                     `json:"agent_round_id"`
	ToolUseID    string                     `json:"tool_use_id"`
	Artifact     WorkspaceFileArtifactBlock `json:"artifact"`
	CreatedAt    time.Time                  `json:"created_at"`
	UpdatedAt    time.Time                  `json:"updated_at"`
}

// ExecutionRuntimeGraph 是 session 最近运行图或当前 Execution 的运行层快照。
type ExecutionRuntimeGraph struct {
	GraphID        string                    `json:"graph_id,omitempty"`
	Nodes          []ExecutionRuntimeNodeRun `json:"nodes,omitempty"`
	Edges          []ExecutionRuntimeEdgeRun `json:"edges,omitempty"`
	NodeTotal      int                       `json:"node_total"`
	EdgeTotal      int                       `json:"edge_total"`
	NodesTruncated bool                      `json:"nodes_truncated"`
	EdgesTruncated bool                      `json:"edges_truncated"`
}
