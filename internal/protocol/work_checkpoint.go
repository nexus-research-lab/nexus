// INPUT: Explicit progress evidence from the current responsibility owner.
// OUTPUT: Immutable checkpoint observations for later reconciliation.
// POS: Resume context only; checkpoints never create Submission or Acceptance.
package protocol

import "time"

type WorkCheckpointEffect struct {
	Operation  string `json:"operation"`
	Outcome    string `json:"outcome"` // confirmed or unknown, as reported by the owner
	ReceiptRef string `json:"receipt_ref,omitempty"`
}
type WorkCheckpointData struct {
	Summary      string                 `json:"summary"`
	ArtifactRefs []string               `json:"artifact_refs,omitempty"`
	Evidence     []string               `json:"evidence,omitempty"`
	NextStep     string                 `json:"next_step"`
	Effects      []WorkCheckpointEffect `json:"effects,omitempty"`
}
type WorkCheckpoint struct {
	ID           string `json:"id"`
	ExecutionID  string `json:"execution_id"`
	PlanID       string `json:"plan_id"`
	WorkItemID   string `json:"work_item_id"`
	SpecID       string `json:"spec_id"`
	AssignmentID string `json:"assignment_id"`
	AttemptID    string `json:"attempt_id"`
	AgentID      string `json:"agent_id"`
	WorkCheckpointData
	CreatedAt time.Time `json:"created_at"`
}
