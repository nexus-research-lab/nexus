// INPUT: Authenticated user recovery intent for one exact WorkGraph root Attempt.
// OUTPUT: Recovery request/receipt and a queue target that grants no work capability.
// POS: Shared HTTP and durable queue recovery identities; runtime effects require reconciliation.
package protocol

// ExecutionRecoveryTarget identifies the failed root to inspect, never a command to replay.
type ExecutionRecoveryTarget struct {
	ExecutionID string `json:"execution_id"`
	WorkItemID  string `json:"work_item_id"`
	AttemptID   string `json:"attempt_id"`
}

type RequestExecutionRecovery struct {
	SessionKey string `json:"session_key"`
	RequestID  string `json:"request_id"`
	WorkItemID string `json:"work_item_id"`
	AttemptID  string `json:"attempt_id"`
}

// ExecutionRecoveryReceipt acknowledges durable intake, not successful recovery.
type ExecutionRecoveryReceipt struct {
	ExecutionRecoveryTarget
	RequestID string `json:"request_id"`
	Queued    bool   `json:"queued"`
}
