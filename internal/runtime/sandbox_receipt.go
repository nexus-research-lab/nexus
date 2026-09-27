// INPUT: 已初始化的 Bridge session、宿主沙箱选项与可选 scratch lease。
// OUTPUT: 只记录本次 runtime 实际确认的沙箱能力、策略摘要和资源身份回执。
// POS: Connect 后的 fail-closed 生效边界；不把选项或能力协商当成 OS 隔离证明。
package runtime

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const sandboxEffectivePolicyReceiptVersion = 1

const maxSandboxReceiptUnknownReason = 512

// SandboxEffectivePolicyReceipt is an in-memory receipt for one connected
// runtime generation. It is diagnostic/runtime state, not authorization and
// not proof that every SDK IO path or OS descendant is confined.
type SandboxEffectivePolicyReceipt struct {
	Version       int                                `json:"version"`
	RuntimeKind   bridge.RuntimeKind                 `json:"runtime_kind"`
	Generation    uint64                             `json:"generation,omitempty"`
	Phase         protocol.SandboxPolicyReceiptPhase `json:"phase,omitempty"`
	UnknownReason string                             `json:"unknown_reason,omitempty"`
	SessionID     string                             `json:"session_id,omitempty"`
	// SessionIDProvisional is true for a fresh Claude connection whose init
	// event does not publish an identity until the first user turn. The host
	// must not treat an empty identity as a durable session binding.
	SessionIDProvisional     bool                `json:"session_id_provisional,omitempty"`
	PolicyDigest             string              `json:"policy_digest"`
	RequiredCapabilities     []bridge.Capability `json:"required_capabilities,omitempty"`
	AcknowledgedCapabilities []bridge.Capability `json:"acknowledged_capabilities,omitempty"`
	// CapabilityEvidence identifies the source of the acknowledgement. The
	// current Bridge contract is negotiation/configuration evidence only.
	CapabilityEvidence string `json:"capability_evidence"`
	// IsolationEvidence is deliberately explicit so consumers cannot render
	// this receipt as proof that all SDK IO or OS descendants are confined.
	IsolationEvidence string                        `json:"isolation_evidence"`
	ResourcePolicy    *bridge.SandboxResourcePolicy `json:"resource_policy,omitempty"`
	LeaseID           string                        `json:"lease_id,omitempty"`
	RoundID           string                        `json:"round_id,omitempty"`
	ConfirmedAt       time.Time                     `json:"confirmed_at"`
}

// boundedSandboxReceiptReason keeps durable lifecycle diagnostics useful while
// preventing a transport/SDK error (which may contain paths or unbounded
// subprocess output) from becoming a large persisted or API-visible field.
func boundedSandboxReceiptReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= maxSandboxReceiptUnknownReason {
		return reason
	}
	cut := maxSandboxReceiptUnknownReason
	for cut > 0 && (reason[cut]&0xc0) == 0x80 {
		cut--
	}
	return reason[:cut]
}

func sandboxReceiptReason(err error) string {
	if err == nil {
		return ""
	}
	return boundedSandboxReceiptReason(err.Error())
}

func receiptRuntimeKind(kind bridge.RuntimeKind) bridge.RuntimeKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case "", "nxs", "go", "go-native", "gonative":
		return bridge.RuntimeNXS
	case "claude", "claude-code", "claudecode", "cc":
		return bridge.RuntimeClaude
	default:
		return bridge.RuntimeKind(strings.ToLower(strings.TrimSpace(string(kind))))
	}
}

func requiredSandboxCapabilities(options bridge.Options) []bridge.Capability {
	if options.Sandbox == nil {
		return nil
	}
	result := make([]bridge.Capability, 0, 12)
	if receiptRuntimeKind(options.Runtime.Kind) == bridge.RuntimeNXS {
		if options.Sandbox.RequireSandbox {
			result = append(result, bridge.CapabilityRequiredSandbox)
		}
		if options.Sandbox.RequireFileTools {
			result = append(result, bridge.CapabilitySandboxFileTools)
		}
		if options.Sandbox.RequireSearchTools {
			result = append(result, bridge.CapabilitySandboxSearchTools)
		}
		if options.Sandbox.RequireMediaFiles {
			result = append(result, bridge.CapabilitySandboxMediaFiles)
		}
		if options.Sandbox.RequireMCPNetwork {
			result = append(result, bridge.CapabilitySandboxMCPNetwork)
		}
		if options.Sandbox.RequireMediaNetwork {
			result = append(result, bridge.CapabilitySandboxMediaNetwork)
		}
		if options.Sandbox.RequireNotebookFiles {
			result = append(result, bridge.CapabilitySandboxNotebookFiles)
		}
		if options.Sandbox.RequireSkillFiles {
			result = append(result, bridge.CapabilitySandboxSkillFiles)
		}
		if options.Sandbox.RequireContextFiles {
			result = append(result, bridge.CapabilitySandboxContextFiles)
		}
		if options.Sandbox.RequireProjectFiles {
			result = append(result, bridge.CapabilitySandboxProjectFiles)
		}
		if options.Sandbox.RequireManagedPolicy {
			result = append(result, bridge.CapabilitySandboxManagedPolicy)
		}
		if options.Sandbox.RequireSettingsFiles {
			result = append(result, bridge.CapabilitySandboxSettingsFiles)
		}
		if options.Sandbox.RequireSettingsWrites {
			result = append(result, bridge.CapabilitySandboxSettingsWrites)
		}
		if options.Sandbox.Resources != nil {
			result = append(result, bridge.CapabilitySandboxResources)
		}
	} else if receiptRuntimeKind(options.Runtime.Kind) == bridge.RuntimeClaude && options.Sandbox.RequireClaudeNativeSandbox {
		result = append(result, bridge.CapabilityClaudeNativeSandbox)
	} else if receiptRuntimeKind(options.Runtime.Kind) == bridge.RuntimeClaude && options.Sandbox.RequireClaudeRestricted {
		result = append(result, bridge.CapabilityClaudeRestricted)
	}
	return result
}

type sandboxRequirementDigest struct {
	RequireClaudeRestricted bool `json:"require_claude_restricted,omitempty"`
	RequireClaudeNative     bool `json:"require_claude_native,omitempty"`
	RequireSandbox          bool `json:"require_sandbox,omitempty"`
	RequireFileTools        bool `json:"require_file_tools,omitempty"`
	RequireSearchTools      bool `json:"require_search_tools,omitempty"`
	RequireMediaFiles       bool `json:"require_media_files,omitempty"`
	RequireMCPNetwork       bool `json:"require_mcp_network,omitempty"`
	RequireMediaNetwork     bool `json:"require_media_network,omitempty"`
	RequireNotebookFiles    bool `json:"require_notebook_files,omitempty"`
	RequireSkillFiles       bool `json:"require_skill_files,omitempty"`
	RequireContextFiles     bool `json:"require_context_files,omitempty"`
	RequireProjectFiles     bool `json:"require_project_files,omitempty"`
	RequireManagedPolicy    bool `json:"require_managed_policy,omitempty"`
	RequireSettingsFiles    bool `json:"require_settings_files,omitempty"`
	RequireSettingsWrites   bool `json:"require_settings_writes,omitempty"`
}

func sandboxRequirementsForDigest(settings *bridge.SandboxSettings) *sandboxRequirementDigest {
	if settings == nil {
		return nil
	}
	return &sandboxRequirementDigest{
		RequireClaudeRestricted: settings.RequireClaudeRestricted,
		RequireClaudeNative:     settings.RequireClaudeNativeSandbox,
		RequireSandbox:          settings.RequireSandbox,
		RequireFileTools:        settings.RequireFileTools,
		RequireSearchTools:      settings.RequireSearchTools,
		RequireMediaFiles:       settings.RequireMediaFiles,
		RequireMCPNetwork:       settings.RequireMCPNetwork,
		RequireMediaNetwork:     settings.RequireMediaNetwork,
		RequireNotebookFiles:    settings.RequireNotebookFiles,
		RequireSkillFiles:       settings.RequireSkillFiles,
		RequireContextFiles:     settings.RequireContextFiles,
		RequireProjectFiles:     settings.RequireProjectFiles,
		RequireManagedPolicy:    settings.RequireManagedPolicy,
		RequireSettingsFiles:    settings.RequireSettingsFiles,
		RequireSettingsWrites:   settings.RequireSettingsWrites,
	}
}

func sandboxPolicyDigest(options bridge.Options) (string, error) {
	type policyShape struct {
		RuntimeKind  bridge.RuntimeKind            `json:"runtime_kind"`
		Sandbox      *bridge.SandboxSettings       `json:"sandbox,omitempty"`
		Requirements *sandboxRequirementDigest     `json:"requirements,omitempty"`
		Resources    *bridge.SandboxResourcePolicy `json:"resources,omitempty"`
		Desktop      bool                          `json:"desktop"`
	}
	shape := policyShape{
		RuntimeKind:  receiptRuntimeKind(options.Runtime.Kind),
		Sandbox:      options.Sandbox,
		Requirements: sandboxRequirementsForDigest(options.Sandbox),
		Desktop:      options.Env[protocol.NexusDesktopSandboxPolicyEnvName] == "1",
	}
	if options.Sandbox != nil {
		shape.Resources = options.Sandbox.Resources
	}
	payload, err := json.Marshal(shape)
	if err != nil {
		return "", fmt.Errorf("marshal desktop sandbox policy digest: %w", err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(payload)), nil
}

func buildSandboxEffectivePolicyReceipt(session *bridge.Session, options bridge.Options, lease *SandboxResourceLease) (*SandboxEffectivePolicyReceipt, error) {
	if session == nil {
		return nil, errors.New("sandbox effective policy requires a connected session")
	}
	if options.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "1" {
		return nil, nil
	}
	if err := validateDesktopSandboxReceiptOptions(options); err != nil {
		return nil, err
	}
	if receiptRuntimeKind(options.Runtime.Kind) == bridge.RuntimeNXS {
		if options.Sandbox == nil || !options.Sandbox.RequireSandbox || !options.Sandbox.RequireFileTools {
			return nil, errors.New("desktop nxs runtime requires command and file sandbox requirements")
		}
	}
	required := requiredSandboxCapabilities(options)
	acknowledged := make([]bridge.Capability, 0, len(required))
	for _, capability := range required {
		if !session.Supports(capability) {
			return nil, fmt.Errorf("desktop sandbox capability was not acknowledged: %s", capability)
		}
		acknowledged = append(acknowledged, capability)
	}
	var resourcePolicy *bridge.SandboxResourcePolicy
	var leaseID, roundID string
	if options.Sandbox != nil && options.Sandbox.Resources != nil {
		copyPolicy := *options.Sandbox.Resources
		resourcePolicy = &copyPolicy
		if lease == nil {
			return nil, errors.New("desktop sandbox resource policy has no host lease")
		}
		if !lease.active() {
			return nil, errors.New("desktop sandbox resource lease is no longer active")
		}
		leasePolicy := lease.Resources()
		if leasePolicy == nil || *leasePolicy != *resourcePolicy {
			return nil, errors.New("desktop sandbox resource policy does not match host lease")
		}
		marker := lease.Marker()
		if marker == nil || strings.TrimSpace(marker.LeaseID) == "" {
			return nil, errors.New("desktop sandbox lease identity is unavailable")
		}
		if marker.CleanupState == cleanupStateUnknown {
			return nil, errors.New("desktop sandbox lease cleanup is unresolved")
		}
		leaseID = marker.LeaseID
		// The durable marker can be shared by several rounds of one
		// owner/session. Use the exact handle identity when available rather
		// than reporting the first round that created the shared directory.
		roundID = lease.RoundID()
		if roundID == "" {
			roundID = marker.RoundID
		}
	}
	sessionID := strings.TrimSpace(session.ID())
	policyDigest, err := sandboxPolicyDigest(options)
	if err != nil {
		return nil, err
	}
	return &SandboxEffectivePolicyReceipt{
		Version:                  sandboxEffectivePolicyReceiptVersion,
		RuntimeKind:              receiptRuntimeKind(options.Runtime.Kind),
		Phase:                    protocol.SandboxPolicyReceiptConfirmed,
		SessionID:                sessionID,
		SessionIDProvisional:     sessionID == "" && receiptRuntimeKind(options.Runtime.Kind) == bridge.RuntimeClaude,
		PolicyDigest:             policyDigest,
		RequiredCapabilities:     slices.Clone(required),
		AcknowledgedCapabilities: slices.Clone(acknowledged),
		CapabilityEvidence:       "bridge_negotiated",
		IsolationEvidence:        "not_attested",
		ResourcePolicy:           resourcePolicy,
		LeaseID:                  leaseID,
		RoundID:                  roundID,
		ConfirmedAt:              time.Now().UTC(),
	}, nil
}

// validateDesktopSandboxReceiptOptions prevents a hand-built desktop nxs
// configuration from receiving an apparently successful receipt while
// omitting the host's required sandbox capability contract. Production option
// assembly sets every field below; this check is the fail-closed boundary for
// alternate callers and tests.
func validateDesktopSandboxReceiptOptions(options bridge.Options) error {
	runtimeKind := receiptRuntimeKind(options.Runtime.Kind)
	if runtimeKind != bridge.RuntimeNXS && runtimeKind != bridge.RuntimeClaude {
		return fmt.Errorf("desktop sandbox runtime kind is unsupported: %q", strings.TrimSpace(string(options.Runtime.Kind)))
	}
	if runtimeKind != bridge.RuntimeNXS {
		return nil
	}
	settings := options.Sandbox
	if settings == nil {
		return errors.New("desktop nxs sandbox settings are missing")
	}
	missing := make([]string, 0, 12)
	if !settings.RequireSandbox {
		missing = append(missing, "sandbox")
	}
	if !settings.RequireFileTools {
		missing = append(missing, "file_tools")
	}
	if !settings.RequireSearchTools {
		missing = append(missing, "search_tools")
	}
	if !settings.RequireMediaFiles {
		missing = append(missing, "media_files")
	}
	if runtime.GOOS == "darwin" && (!settings.RequireMCPNetwork || !options.MCP.StrictConfig) {
		missing = append(missing, "mcp_network")
	}
	if !settings.RequireMediaNetwork {
		missing = append(missing, "media_network")
	}
	if !settings.RequireNotebookFiles {
		missing = append(missing, "notebook_files")
	}
	if !settings.RequireSkillFiles {
		missing = append(missing, "skill_files")
	}
	if !settings.RequireContextFiles {
		missing = append(missing, "context_files")
	}
	if !settings.RequireProjectFiles {
		missing = append(missing, "project_files")
	}
	if !settings.RequireManagedPolicy {
		missing = append(missing, "managed_policy")
	}
	if !settings.RequireSettingsFiles {
		missing = append(missing, "settings_files")
	}
	if !settings.RequireSettingsWrites {
		missing = append(missing, "settings_writes")
	}
	if settings.Enabled == nil || !*settings.Enabled {
		missing = append(missing, "enabled")
	}
	if settings.FailIfUnavailable == nil || !*settings.FailIfUnavailable {
		missing = append(missing, "fail_if_unavailable")
	}
	if len(missing) != 0 {
		return fmt.Errorf("desktop nxs sandbox contract is incomplete: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func cloneSandboxEffectivePolicyReceipt(input *SandboxEffectivePolicyReceipt) *SandboxEffectivePolicyReceipt {
	if input == nil {
		return nil
	}
	output := *input
	output.RequiredCapabilities = slices.Clone(input.RequiredCapabilities)
	output.AcknowledgedCapabilities = slices.Clone(input.AcknowledgedCapabilities)
	if input.ResourcePolicy != nil {
		policy := *input.ResourcePolicy
		output.ResourcePolicy = &policy
	}
	return &output
}

// sandboxPolicyReceiptSnapshot converts the SDK-facing receipt into the
// storage-neutral protocol shape. Resource policy remains opaque JSON so the
// storage layer cannot accidentally become an SDK authorization surface.
func sandboxPolicyReceiptSnapshot(
	ownerUserID string,
	sessionKey string,
	generation uint64,
	receipt *SandboxEffectivePolicyReceipt,
	phase protocol.SandboxPolicyReceiptPhase,
	unknownReason string,
) (protocol.SandboxPolicyReceiptSnapshot, error) {
	if receipt == nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, errors.New("desktop sandbox policy receipt is missing")
	}
	resourceJSON := ""
	if receipt.ResourcePolicy != nil {
		encoded, err := json.Marshal(receipt.ResourcePolicy)
		if err != nil {
			return protocol.SandboxPolicyReceiptSnapshot{}, fmt.Errorf("marshal desktop sandbox resource policy: %w", err)
		}
		resourceJSON = string(encoded)
	}
	required := make([]string, 0, len(receipt.RequiredCapabilities))
	for _, capability := range receipt.RequiredCapabilities {
		required = append(required, string(capability))
	}
	acknowledged := make([]string, 0, len(receipt.AcknowledgedCapabilities))
	for _, capability := range receipt.AcknowledgedCapabilities {
		acknowledged = append(acknowledged, string(capability))
	}
	return protocol.SandboxPolicyReceiptSnapshot{
		Version:                  receipt.Version,
		OwnerUserID:              strings.TrimSpace(ownerUserID),
		SessionKey:               strings.TrimSpace(sessionKey),
		SessionID:                receipt.SessionID,
		SessionIDProvisional:     receipt.SessionIDProvisional,
		RuntimeKind:              string(receipt.RuntimeKind),
		Generation:               generation,
		RoundID:                  receipt.RoundID,
		PolicyDigest:             receipt.PolicyDigest,
		RequiredCapabilities:     required,
		AcknowledgedCapabilities: acknowledged,
		CapabilityEvidence:       receipt.CapabilityEvidence,
		IsolationEvidence:        receipt.IsolationEvidence,
		ResourcePolicyJSON:       resourceJSON,
		LeaseID:                  receipt.LeaseID,
		Phase:                    phase,
		UnknownReason:            boundedSandboxReceiptReason(unknownReason),
		ConfirmedAt:              receipt.ConfirmedAt,
		UpdatedAt:                time.Now().UTC(),
	}, nil
}

// sandboxPolicyReceiptFromSnapshot reconstructs the diagnostic response from
// durable fields. It accepts lifecycle phases because a retired/unknown row is
// still useful audit evidence, while malformed opaque policy JSON fails closed.
func sandboxPolicyReceiptFromSnapshot(snapshot protocol.SandboxPolicyReceiptSnapshot) (*SandboxEffectivePolicyReceipt, error) {
	if snapshot.Version <= 0 || strings.TrimSpace(snapshot.PolicyDigest) == "" || snapshot.Generation == 0 {
		return nil, errors.New("persisted desktop sandbox policy receipt is incomplete")
	}
	switch snapshot.Phase {
	case protocol.SandboxPolicyReceiptConfirmed,
		protocol.SandboxPolicyReceiptRetiring,
		protocol.SandboxPolicyReceiptRetired,
		protocol.SandboxPolicyReceiptUnknown,
		protocol.SandboxPolicyReceiptReconciled:
	default:
		return nil, fmt.Errorf("persisted desktop sandbox policy receipt has unsupported phase %q", snapshot.Phase)
	}
	if snapshot.Phase == protocol.SandboxPolicyReceiptUnknown && strings.TrimSpace(snapshot.UnknownReason) == "" {
		return nil, errors.New("persisted desktop sandbox policy receipt has unknown phase without reason")
	}
	required := make([]bridge.Capability, 0, len(snapshot.RequiredCapabilities))
	for _, capability := range snapshot.RequiredCapabilities {
		if strings.TrimSpace(capability) == "" {
			return nil, errors.New("persisted desktop sandbox policy receipt has an empty required capability")
		}
		required = append(required, bridge.Capability(capability))
	}
	acknowledged := make([]bridge.Capability, 0, len(snapshot.AcknowledgedCapabilities))
	for _, capability := range snapshot.AcknowledgedCapabilities {
		if strings.TrimSpace(capability) == "" {
			return nil, errors.New("persisted desktop sandbox policy receipt has an empty acknowledged capability")
		}
		acknowledged = append(acknowledged, bridge.Capability(capability))
	}
	var resourcePolicy *bridge.SandboxResourcePolicy
	if strings.TrimSpace(snapshot.ResourcePolicyJSON) != "" {
		resourcePolicy = new(bridge.SandboxResourcePolicy)
		if err := json.Unmarshal([]byte(snapshot.ResourcePolicyJSON), resourcePolicy); err != nil {
			return nil, fmt.Errorf("decode persisted desktop sandbox resource policy: %w", err)
		}
		if err := resourcePolicy.Validate(); err != nil {
			return nil, fmt.Errorf("validate persisted desktop sandbox resource policy: %w", err)
		}
	}
	return &SandboxEffectivePolicyReceipt{
		Version:                  snapshot.Version,
		RuntimeKind:              receiptRuntimeKind(bridge.RuntimeKind(snapshot.RuntimeKind)),
		Generation:               snapshot.Generation,
		Phase:                    snapshot.Phase,
		UnknownReason:            snapshot.UnknownReason,
		SessionID:                snapshot.SessionID,
		SessionIDProvisional:     snapshot.SessionIDProvisional,
		PolicyDigest:             snapshot.PolicyDigest,
		RequiredCapabilities:     required,
		AcknowledgedCapabilities: acknowledged,
		CapabilityEvidence:       snapshot.CapabilityEvidence,
		IsolationEvidence:        snapshot.IsolationEvidence,
		ResourcePolicy:           resourcePolicy,
		LeaseID:                  snapshot.LeaseID,
		RoundID:                  snapshot.RoundID,
		ConfirmedAt:              snapshot.ConfirmedAt,
	}, nil
}
