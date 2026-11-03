// INPUT: Authenticated user recovery intent for one exact WorkGraph root Attempt.
// OUTPUT: Exact continuation request/receipt and scoped queue admission identity.
// POS: Shared HTTP and durable queue recovery identities; runtime effects require reconciliation.
package protocol

// ExecutionRecoveryTarget identifies an exact root; HTTP targets the stopped root and bound queues target its replacement.
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
