// INPUT: Host-selected desktop execution policy.
// OUTPUT: Stable environment marker retained in the runtime process-policy fingerprint.
// POS: Host-only policy metadata; this marker is not a tool or sandbox escape grant.
package protocol

import "time"

// NexusDesktopSandboxPolicyEnvName identifies a host-managed desktop sandbox policy
// across approval-mode changes. Client option assembly discards caller-supplied values.
const NexusDesktopSandboxPolicyEnvName = "NEXUS_RUNTIME_POLICY_DESKTOP_SANDBOX"

// SandboxPolicyReceiptPhase is the durable lifecycle state of a desktop
// sandbox policy receipt.  The receipt is an audit fact; it is never an OS
// isolation attestation or a permission grant.
type SandboxPolicyReceiptPhase string

const (
	SandboxPolicyReceiptConfirmed  SandboxPolicyReceiptPhase = "confirmed"
	SandboxPolicyReceiptRetiring   SandboxPolicyReceiptPhase = "retiring"
	SandboxPolicyReceiptRetired    SandboxPolicyReceiptPhase = "retired"
	SandboxPolicyReceiptUnknown    SandboxPolicyReceiptPhase = "unknown"
	SandboxPolicyReceiptReconciled SandboxPolicyReceiptPhase = "reconciled"
)

// SandboxPolicyReceiptSnapshot is the storage-neutral representation shared
// by runtime and the host database repository. JSON fields intentionally stay
// opaque here so protocol does not depend on an SDK or a concrete storage
// package.
type SandboxPolicyReceiptSnapshot struct {
	Version                  int
	OwnerUserID              string
	SessionKey               string
	SessionID                string
	SessionIDProvisional     bool
	RuntimeKind              string
	Generation               uint64
	RoundID                  string
	PolicyDigest             string
	RequiredCapabilities     []string
	AcknowledgedCapabilities []string
	CapabilityEvidence       string
	IsolationEvidence        string
	ResourcePolicyJSON       string
	LeaseID                  string
	Phase                    SandboxPolicyReceiptPhase
	UnknownReason            string
	ConfirmedAt              time.Time
	UpdatedAt                time.Time
}
