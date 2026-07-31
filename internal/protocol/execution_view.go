// INPUT: ExecutionSnapshot 中当前 active Plan 的业务状态。
// OUTPUT: 面向 Web/桌面端的安全 WorkGraph 只读投影，不暴露 command、lease 或 runtime capability identity。
// POS: Execution Orchestration 状态机与 DM/Room 进程 UI 之间的跨边界展示协议。
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
	Status          WorkAttemptStatus   `json:"status"`
	FailureReason   string              `json:"failure_reason,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	StartedAt       *time.Time          `json:"started_at,omitempty"`
	FinishedAt      *time.Time          `json:"finished_at,omitempty"`
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
	CompletionBlockers    []string                `json:"completion_blockers,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
	CompletedAt           *time.Time              `json:"completed_at,omitempty"`
}
