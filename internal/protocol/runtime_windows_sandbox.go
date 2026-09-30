// INPUT: Windows 产品监督的原始 owner/session、策略代次、lease 和独立启动意图。
// OUTPUT: 不含命令/凭据正文的持久 Windows 阶段与执行双摘要。
// POS: 独立于 Darwin UID/launchd 登记，不把 helper 退出当资源清理。
package protocol

type WindowsSandboxKey struct {
	OwnerUserID string `json:"owner_user_id"`
	SessionKey  string `json:"session_key"`
	Generation  uint64 `json:"generation"`
	LaunchID    string `json:"launch_id"`
}

type WindowsSandboxIntent struct {
	Key           WindowsSandboxKey `json:"key"`
	Version       int               `json:"version"`
	Purpose       string            `json:"purpose"`
	LeaseID       string            `json:"lease_id"`
	ScratchRoot   string            `json:"scratch_root"`
	PolicyDigest  [32]byte          `json:"policy_digest"`
	OptionsDigest [32]byte          `json:"options_digest"`
	CommandDigest [32]byte          `json:"command_digest"`
	HelperSHA256  string            `json:"helper_sha256"`
}

type WindowsSandboxPrepared struct {
	ExecutionID    string   `json:"execution_id"`
	PrepareDigest  [32]byte `json:"prepare_digest"`
	ManifestDigest [32]byte `json:"manifest_digest"`
}

type WindowsSandboxOutcome struct {
	Prepared WindowsSandboxPrepared `json:"prepared"`
	Cleaned  bool                   `json:"cleaned"`
	ExitCode uint32                 `json:"exit_code"`
	Reason   string                 `json:"reason"`
}

type WindowsSandboxSnapshot struct {
	Intent   WindowsSandboxIntent
	Phase    string
	Prepared *WindowsSandboxPrepared
	Outcome  *WindowsSandboxOutcome
}

type WindowsSandboxPolicyBinding struct {
	Key      WindowsSandboxKey      `json:"key"`
	Prepared WindowsSandboxPrepared `json:"prepared"`
}

// WindowsSandboxResources 的路径只用于诊断/绑定，不是冷恢复重新打开或删除的授权。
type WindowsSandboxResources struct {
	OwnerUserID string
	SessionKey  string
	LeaseID     string
	LaunchID    string
	ScratchRoot string
	Phase       string
}

type WindowsSandboxRecoveryDiagnostic struct {
	Process            WindowsSandboxSnapshot
	Resources          *WindowsSandboxResources
	Reason             string
	ReconciledPolicies int64
}

// WindowsSandboxPurposeOrder 在同一启动代次内保留 probe 与 runtime 的顺序，禁止重播早期用途。
func WindowsSandboxPurposeOrder(purpose string) (int, bool) {
	switch purpose {
	case "version_probe":
		return 1, true
	case "runtime":
		return 2, true
	}
	return 0, false
}
