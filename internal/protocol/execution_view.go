// INPUT: ExecutionSnapshot 中当前 active Plan、责任状态与 WorkAttempt 运行身份。
// OUTPUT: 面向 Web/桌面端的安全 WorkGraph、Agent/Subagent 分层运行图、NodeRun 历史与 Artifact 引用；不暴露 command、lease 或 runtime capability identity。
// POS: Execution Orchestration 状态机与 DM/Room Execution Graph UI 之间的跨边界展示协议。
package protocol

import "time"

// ExecutionWorkItemViewStatus 是 UI 对当前 Work Item 交付阶段的稳定枚举。
type ExecutionWorkItemViewStatus string

const (
	ExecutionWorkItemViewWaiting          ExecutionWorkItemViewStatus = "waiting"
	ExecutionWorkItemViewReady            ExecutionWorkItemViewStatus = "ready"
	ExecutionWorkItemViewAssigned         ExecutionWorkItemViewStatus = "assigned"
	ExecutionWorkItemViewRunning          ExecutionWorkItemViewStatus = "running"
	ExecutionWorkItemViewBlocked          ExecutionWorkItemViewStatus = "blocked"
	ExecutionWorkItemViewSubmitted        ExecutionWorkItemViewStatus = "submitted"
	ExecutionWorkItemViewChangesRequested ExecutionWorkItemViewStatus = "changes_requested"
	ExecutionWorkItemViewAccepted         ExecutionWorkItemViewStatus = "accepted"
	ExecutionWorkItemViewFailed           ExecutionWorkItemViewStatus = "failed"
	ExecutionWorkItemViewCancelled        ExecutionWorkItemViewStatus = "cancelled"
)

// ExecutionPlanView 是当前 immutable Plan revision 的用户可见身份。
type ExecutionPlanView struct {
	ID             string             `json:"id"`
	Revision       int64              `json:"revision"`
	Status         PlanRevisionStatus `json:"status"`
	RevisionReason string             `json:"revision_reason,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	ActivatedAt    *time.Time         `json:"activated_at,omitempty"`
}

// ExecutionProgressView 是 WorkGraph 顶部摘要使用的互斥状态计数。
type ExecutionProgressView struct {
	Total            int `json:"total"`
	Required         int `json:"required"`
	Accepted         int `json:"accepted"`
	Running          int `json:"running"`
	Blocked          int `json:"blocked"`
	Submitted        int `json:"submitted"`
	Ready            int `json:"ready"`
	Waiting          int `json:"waiting"`
	ChangesRequested int `json:"changes_requested"`
	Failed           int `json:"failed"`
	Cancelled        int `json:"cancelled"`
}

// ExecutionAttemptView 展示一次责任 Agent 或其子智能体的有界执行尝试。
type ExecutionAttemptView struct {
	ID              string              `json:"id"`
	AssignmentID    string              `json:"assignment_id"`
	ParentAttemptID string              `json:"parent_attempt_id,omitempty"`
	ExecutorKind    AttemptExecutorKind `json:"executor_kind"`
	ExecutorAgentID string              `json:"executor_agent_id,omitempty"`
	ParentAgentID   string              `json:"parent_agent_id,omitempty"`
	AgentRoundID    string              `json:"agent_round_id,omitempty"`
	ChildSessionID  string              `json:"child_session_id,omitempty"`
	TaskID          string              `json:"task_id,omitempty"`
	ToolUseID       string              `json:"tool_use_id,omitempty"`
	Status          WorkAttemptStatus   `json:"status"`
	FailureReason   string              `json:"failure_reason,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	StartedAt       *time.Time          `json:"started_at,omitempty"`
	FinishedAt      *time.Time          `json:"finished_at,omitempty"`
}

// ExecutionGraphNodeKind 是当前已由权威领域身份支撑的运行图节点类型。
// Gate 来自 Assignment review binding / durable review dispatch；Tool 来自 Bridge
// lifecycle identity，二者都不依赖模型用自然语言汇报状态。
type ExecutionGraphNodeKind string

const (
	ExecutionGraphNodeAgent    ExecutionGraphNodeKind = "agent"
	ExecutionGraphNodeSubagent ExecutionGraphNodeKind = "subagent"
	ExecutionGraphNodeTool     ExecutionGraphNodeKind = "tool"
	ExecutionGraphNodeGate     ExecutionGraphNodeKind = "gate"
)

// ExecutionGraphNodeVisibility 控制默认主图与节点内展开层级。
type ExecutionGraphNodeVisibility string

const (
	ExecutionGraphNodePrimary ExecutionGraphNodeVisibility = "primary"
	ExecutionGraphNodeNested  ExecutionGraphNodeVisibility = "nested"
	ExecutionGraphNodeDetail  ExecutionGraphNodeVisibility = "detail"
)

// ExecutionGraphEdgeKind 区分共享责任依赖、只读协调责任、真实 dispatch、
// Agent 内部 child spawn、可选语义 Gate、正式审核和控制返回事实。
type ExecutionGraphEdgeKind string

const (
	ExecutionGraphEdgeDependency ExecutionGraphEdgeKind = "dependency"
	ExecutionGraphEdgeDispatch   ExecutionGraphEdgeKind = "dispatch"
	// ExecutionGraphEdgeCoordination 只投影 coordinator 对已声明根工作项的责任，
	// 不表示已经创建 Assignment、启动 round 或选择了下一步。
	ExecutionGraphEdgeCoordination ExecutionGraphEdgeKind = "coordination"
	ExecutionGraphEdgeSpawn        ExecutionGraphEdgeKind = "spawn"
	ExecutionGraphEdgeInvoke       ExecutionGraphEdgeKind = "invoke"
	ExecutionGraphEdgeGuard        ExecutionGraphEdgeKind = "guard"
	ExecutionGraphEdgeReview       ExecutionGraphEdgeKind = "review"
	ExecutionGraphEdgeLoopBack     ExecutionGraphEdgeKind = "loop_back"
	ExecutionGraphEdgeRetry        ExecutionGraphEdgeKind = "retry"
)

// ExecutionGraphNodeRunView 是稳定 GraphNode 下的一次不可变运行事实。
// managed Attempt 与 runtime NodeRun 会按 exact round/subject identity 合并，
// 避免把同一次物理运行重复展示成两条历史。
type ExecutionGraphNodeRunView struct {
	ID               string                       `json:"id"`
	AttemptID        string                       `json:"attempt_id,omitempty"`
	RuntimeNodeID    string                       `json:"runtime_node_id,omitempty"`
	AgentRoundID     string                       `json:"agent_round_id,omitempty"`
	SubjectID        string                       `json:"subject_id,omitempty"`
	Status           string                       `json:"status,omitempty"`
	ResultSummary    string                       `json:"result_summary,omitempty"`
	ErrorCode        string                       `json:"error_code,omitempty"`
	ErrorSummary     string                       `json:"error_summary,omitempty"`
	SummaryTruncated bool                         `json:"summary_truncated,omitempty"`
	DurationMS       int64                        `json:"duration_ms,omitempty"`
	StartedAt        *time.Time                   `json:"started_at,omitempty"`
	FinishedAt       *time.Time                   `json:"finished_at,omitempty"`
	Artifacts        []WorkspaceFileArtifactBlock `json:"artifacts,omitempty"`
}

// ExecutionGraphNodeView 把稳定责任节点与其当前 Node Run 分开表达。
// Agent 节点 ID 沿用 Work Item ID；Subagent 节点 ID 沿用 child Attempt ID。
type ExecutionGraphNodeView struct {
	ID                   string                       `json:"id"`
	Kind                 ExecutionGraphNodeKind       `json:"kind"`
	Visibility           ExecutionGraphNodeVisibility `json:"visibility"`
	WorkItemID           string                       `json:"work_item_id"`
	AttemptID            string                       `json:"attempt_id,omitempty"`
	ParentNodeID         string                       `json:"parent_node_id,omitempty"`
	AgentID              string                       `json:"agent_id,omitempty"`
	AgentRoundID         string                       `json:"agent_round_id,omitempty"`
	SubjectID            string                       `json:"subject_id,omitempty"`
	Name                 string                       `json:"name,omitempty"`
	Description          string                       `json:"description,omitempty"`
	LifecycleStatus      string                       `json:"lifecycle_status,omitempty"`
	ReviewDispatchID     string                       `json:"review_dispatch_id,omitempty"`
	ReviewerKind         WorkReviewerKind             `json:"reviewer_kind,omitempty"`
	ResponsibilityStatus ExecutionWorkItemViewStatus  `json:"responsibility_status,omitempty"`
	RunStatus            WorkAttemptStatus            `json:"run_status,omitempty"`
	ResultSummary        string                       `json:"result_summary,omitempty"`
	ErrorCode            string                       `json:"error_code,omitempty"`
	ErrorSummary         string                       `json:"error_summary,omitempty"`
	SummaryTruncated     bool                         `json:"summary_truncated,omitempty"`
	DurationMS           int64                        `json:"duration_ms,omitempty"`
	StartedAt            *time.Time                   `json:"started_at,omitempty"`
	FinishedAt           *time.Time                   `json:"finished_at,omitempty"`
	Runs                 []ExecutionGraphNodeRunView  `json:"runs,omitempty"`
	Position             int                          `json:"position"`
}

// ExecutionGraphEdgeView 是前端只画方向、点击后再解释语义的 typed edge。
type ExecutionGraphEdgeView struct {
	ID              string                 `json:"id"`
	Kind            ExecutionGraphEdgeKind `json:"kind"`
	SourceNodeID    string                 `json:"source_node_id"`
	TargetNodeID    string                 `json:"target_node_id"`
	SourceNodeRunID string                 `json:"source_node_run_id,omitempty"`
	TargetNodeRunID string                 `json:"target_node_run_id,omitempty"`
	CreatedAt       *time.Time             `json:"created_at,omitempty"`
}

// ExecutionGraphView 是 WorkGraph responsibility 与 WorkAttempt runtime 的确定性分层读模型。
// runtime total / truncated 仅描述 visibility != detail 的主图运行投影；节点
// 检查器内的 detail 历史不占主图配额，也不触发 partial。
type ExecutionGraphView struct {
	Nodes                 []ExecutionGraphNodeView `json:"nodes,omitempty"`
	Edges                 []ExecutionGraphEdgeView `json:"edges,omitempty"`
	RuntimeNodeTotal      int                      `json:"runtime_node_total"`
	RuntimeEdgeTotal      int                      `json:"runtime_edge_total"`
	RuntimeNodesTruncated bool                     `json:"runtime_nodes_truncated"`
	RuntimeEdgesTruncated bool                     `json:"runtime_edges_truncated"`
}

// ExecutionSubmissionView 展示当前 spec 最近一次不可变交付声明。
type ExecutionSubmissionView struct {
	ID               string    `json:"id"`
	SubmitterAgentID string    `json:"submitter_agent_id"`
	ResultSummary    string    `json:"result_summary"`
	ResultRefs       []string  `json:"result_refs,omitempty"`
	Evidence         []string  `json:"evidence,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ExecutionAcceptanceView 展示最近一次交付的验收结论。
type ExecutionAcceptanceView struct {
	ID              string                          `json:"id"`
	Decision        WorkAcceptanceDecision          `json:"decision"`
	ReviewerKind    WorkReviewerKind                `json:"reviewer_kind"`
	ReviewerID      string                          `json:"reviewer_id,omitempty"`
	CriteriaResults []WorkAcceptanceCriterionResult `json:"criteria_results,omitempty"`
	Feedback        string                          `json:"feedback,omitempty"`
	CreatedAt       time.Time                       `json:"created_at"`
}

// ExecutionWorkItemView 是一个 Work Item 当前 spec 的完整用户可见交付契约。
type ExecutionWorkItemView struct {
	ID                 string                      `json:"id"`
	LogicalKey         string                      `json:"logical_key"`
	Kind               WorkItemKind                `json:"kind"`
	Subject            string                      `json:"subject"`
	Objective          string                      `json:"objective"`
	Deliverable        string                      `json:"deliverable"`
	AcceptanceCriteria []string                    `json:"acceptance_criteria,omitempty"`
	InputRefs          []string                    `json:"input_refs,omitempty"`
	OutputScopes       []WorkOutputScope           `json:"output_scopes,omitempty"`
	DependencyIDs      []string                    `json:"dependency_ids,omitempty"`
	ParentWorkItemID   string                      `json:"parent_work_item_id,omitempty"`
	Required           bool                        `json:"required"`
	Terminal           bool                        `json:"terminal,omitempty"`
	Position           int                         `json:"position"`
	Status             ExecutionWorkItemViewStatus `json:"status"`
	BlockReason        string                      `json:"block_reason,omitempty"`
	NeededInput        string                      `json:"needed_input,omitempty"`
	OwnerAgentID       string                      `json:"owner_agent_id,omitempty"`
	AssignmentID       string                      `json:"assignment_id,omitempty"`
	AssignmentStatus   WorkAssignmentStatus        `json:"assignment_status,omitempty"`
	AssignmentStrategy AssignmentStrategy          `json:"assignment_strategy,omitempty"`
	ReviewAgentID      string                      `json:"review_agent_id,omitempty"`
	ReviewDispatchID   string                      `json:"review_dispatch_id,omitempty"`
	ReviewStatus       string                      `json:"review_status,omitempty"`
	Attempts           []ExecutionAttemptView      `json:"attempts,omitempty"`
	Submission         *ExecutionSubmissionView    `json:"submission,omitempty"`
	Acceptance         *ExecutionAcceptanceView    `json:"acceptance,omitempty"`
	UpdatedAt          time.Time                   `json:"updated_at"`
}

// ExecutionView 是 DM/Room 共用的当前或最近一次 WorkGraph 展示快照。
type ExecutionView struct {
	ID                    string                  `json:"id"`
	SessionKey            string                  `json:"session_key"`
	ScopeKind             ExecutionScopeKind      `json:"scope_kind"`
	RoomID                string                  `json:"room_id,omitempty"`
	ConversationID        string                  `json:"conversation_id,omitempty"`
	CoordinatorAgentID    string                  `json:"coordinator_agent_id,omitempty"`
	Objective             string                  `json:"objective"`
	CompletionCriteria    []string                `json:"completion_criteria,omitempty"`
	GoalID                string                  `json:"goal_id,omitempty"`
	GoalObjectiveRevision int64                   `json:"goal_objective_revision,omitempty"`
	Status                ExecutionStatus         `json:"status"`
	Version               int64                   `json:"version"`
	Plan                  *ExecutionPlanView      `json:"plan,omitempty"`
	Progress              ExecutionProgressView   `json:"progress"`
	WorkItems             []ExecutionWorkItemView `json:"work_items,omitempty"`
	Graph                 ExecutionGraphView      `json:"graph"`
	CompletionBlockers    []string                `json:"completion_blockers,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
	CompletedAt           *time.Time              `json:"completed_at,omitempty"`
}
