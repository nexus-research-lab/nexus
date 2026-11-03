// INPUT: Explicit coordinator routing for the current immutable Plan.
// OUTPUT: Durable once-only dispatch policy and its observed admission state.
// POS: WorkGraph routing contract; a policy never proves delivery or acceptance.
package protocol

type ExecutionDispatchPolicy struct {
	ID              string                `json:"id"`
	ExecutionID     string                `json:"execution_id"`
	PlanID          string                `json:"plan_id"`
	WorkItemID      string                `json:"work_item_id"`
	SpecID          string                `json:"spec_id"`
	TargetAgentID   string                `json:"target_agent_id"`
	ReturnToAgentID string                `json:"return_to_agent_id"`
	Instruction     string                `json:"instruction,omitempty"`
	DispatchKind    ExecutionDispatchKind `json:"dispatch_kind,omitempty"`
	State           string                `json:"state"`
	Reason          string                `json:"reason,omitempty"`
	AssignmentID    string                `json:"assignment_id,omitempty"`
	AttemptID       string                `json:"attempt_id,omitempty"`
}
