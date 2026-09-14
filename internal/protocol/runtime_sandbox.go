// INPUT: Host-selected desktop execution policy.
// OUTPUT: Stable environment marker retained in the runtime process-policy fingerprint.
// POS: Host-only policy metadata; this marker is not a tool or sandbox escape grant.
package protocol

// NexusDesktopSandboxPolicyEnvName identifies a host-managed desktop sandbox policy
// across approval-mode changes. Client option assembly discards caller-supplied values.
const NexusDesktopSandboxPolicyEnvName = "NEXUS_RUNTIME_POLICY_DESKTOP_SANDBOX"
