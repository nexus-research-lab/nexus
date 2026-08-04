// INPUT: Bridge provider-neutral lifecycle event 与 Nexus round identity。
// OUTPUT: 可幂等持久化并与托管 WorkGraph 合并的 Runtime NodeRun / EdgeRun。
// POS: runtime 观测事实的协议模型；不承载 Plan、Assignment 或 Goal 生命周期。
package protocol

import "time"

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
	ID               string                     `json:"id"`
	GraphID          string                     `json:"graph_id"`
	OwnerUserID      string                     `json:"owner_user_id"`
	SessionKey       string                     `json:"session_key"`
	ExecutionID      string                     `json:"execution_id,omitempty"`
	Kind             ExecutionRuntimeNodeKind   `json:"kind"`
	SubjectID        string                     `json:"subject_id"`
	ParentSubjectID  string                     `json:"parent_subject_id,omitempty"`
	RootRoundID      string                     `json:"root_round_id"`
	RuntimeRoundID   string                     `json:"runtime_round_id"`
	AgentRoundID     string                     `json:"agent_round_id"`
	AgentID          string                     `json:"agent_id,omitempty"`
	Name             string                     `json:"name,omitempty"`
	Description      string                     `json:"description,omitempty"`
	Status           ExecutionRuntimeNodeStatus `json:"status"`
	Failed           bool                       `json:"failed,omitempty"`
	ResultSummary    string                     `json:"result_summary,omitempty"`
	ErrorCode        string                     `json:"error_code,omitempty"`
	ErrorSummary     string                     `json:"error_summary,omitempty"`
	SummaryTruncated bool                       `json:"summary_truncated,omitempty"`
	DurationMS       int64                      `json:"duration_ms,omitempty"`
	StartedAt        time.Time                  `json:"started_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	FinishedAt       *time.Time                 `json:"finished_at,omitempty"`
	Metadata         map[string]any             `json:"metadata,omitempty"`
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

// ExecutionRuntimeGraph 是 session 最近运行图或当前 Execution 的运行层快照。
type ExecutionRuntimeGraph struct {
	GraphID string                    `json:"graph_id,omitempty"`
	Nodes   []ExecutionRuntimeNodeRun `json:"nodes,omitempty"`
	Edges   []ExecutionRuntimeEdgeRun `json:"edges,omitempty"`
}
