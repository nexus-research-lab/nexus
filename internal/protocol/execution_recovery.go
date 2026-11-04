// INPUT: Authenticated user recovery intent for one exact WorkGraph root Attempt.
// OUTPUT: Exact continuation/retry requests, revision-bound impact previews and durable receipts.
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

// WorkRetryPreview is the exact revision and dependency closure the user confirms.
type WorkRetryPreview struct {
	ExecutionRecoveryTarget
	SnapshotRevision int64             `json:"snapshot_revision"`
	Submitted        bool              `json:"submitted"`
	Affected         []WorkRetryImpact `json:"affected"`
}
type WorkRetryImpact struct {
	WorkItemID string `json:"work_item_id"`
	Subject    string `json:"subject"`
	AgentID    string `json:"agent_id,omitempty"`
	Running    bool   `json:"running"`
	Submitted  bool   `json:"submitted"`
}
type RequestWorkRetry struct {
	RequestExecutionRecovery
	SnapshotRevision int64 `json:"snapshot_revision"`
}
