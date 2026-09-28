// INPUT: 宿主签发的 exact owner/session/generation、原 scratch 目录身份与原生进程集合登记。
// OUTPUT: 执行前持久意图、一次性放行状态及 exact 集合回收事实。
// POS: 宿主内部启动合同，不是模型 API、权限凭据或业务结果回执。
package protocol

import "time"

type SandboxProcessPhase string

const (
	SandboxProcessPrepared   SandboxProcessPhase = "prepared"
	SandboxProcessRegistered SandboxProcessPhase = "registered"
	SandboxProcessReleased   SandboxProcessPhase = "released"
	SandboxProcessAborted    SandboxProcessPhase = "aborted"
	SandboxProcessReaped     SandboxProcessPhase = "reaped"
)

type SandboxProcessKey struct {
	OwnerUserID string `json:"owner_user_id"`
	SessionKey  string `json:"session_key"`
	Generation  uint64 `json:"generation"`
	LaunchID    string `json:"launch_id"`
}

// SandboxProcessScratch is host-captured filesystem identity, persisted before
// launch. Zero means no trusted scratch proof, including historical records.
// Marker contents are never a source for these filesystem identities.
type SandboxProcessScratch struct {
	BasePath     string `json:"base_path,omitempty"`
	LeafName     string `json:"leaf_name,omitempty"`
	BaseIdentity string `json:"base_identity,omitempty"`
	LeafIdentity string `json:"leaf_identity,omitempty"`
}

// SandboxProcessIntent 不含命令参数、环境、Provider 凭据或任务正文。
type SandboxProcessIntent struct {
	Key          SandboxProcessKey     `json:"key"`
	Version      int                   `json:"version"`
	RuntimeKind  string                `json:"runtime_kind"`
	Purpose      SandboxProcessPurpose `json:"purpose,omitempty"`
	Scratch      SandboxProcessScratch `json:"scratch,omitempty"`
	LeaseID      string                `json:"lease_id,omitempty"`
	BootID       string                `json:"boot_id"`
	OwnerUID     uint32                `json:"owner_uid"`
	JobLabel     string                `json:"job_label"`
	HelperSHA256 string                `json:"helper_sha256"`
}

// SandboxProcessRegistration 来自已认证 helper 的内核观察，必须在放行前存储。
type SandboxProcessRegistration struct {
	Version     int    `json:"version"`
	BootID      string `json:"boot_id"`
	CoalitionID uint64 `json:"coalition_id"`
	OwnerUID    uint32 `json:"owner_uid"`
}

// SandboxProcessEvidence 由可信原生观察者提交，数据库只核对 exact binding。
// 根进程退出、空枚举、job 消失或等待超时均不能构造本事实。
type SandboxProcessEvidence struct {
	Registration   SandboxProcessRegistration `json:"registration"`
	Reason         string                     `json:"reason"`
	ObservedBootID string                     `json:"observed_boot_id"`
}

type SandboxProcessSnapshot struct {
	Intent       SandboxProcessIntent
	Phase        SandboxProcessPhase
	Registration *SandboxProcessRegistration
	Evidence     *SandboxProcessEvidence
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SandboxProcessPurpose 在同一宿主启动代次内区分探测与正式 runtime。
// 空值只兼容已保存的单进程登记，语义固定为 runtime，不是未知或任意用途。
type SandboxProcessPurpose string

const (
	SandboxProcessRuntime               SandboxProcessPurpose = "runtime"
	SandboxProcessClaudeSandboxProbe    SandboxProcessPurpose = "claude_sandbox_probe"
	SandboxProcessClaudeRestrictedProbe SandboxProcessPurpose = "claude_restricted_probe"
	SandboxProcessVersionProbe          SandboxProcessPurpose = "version_probe"
)

// Order 固定启动顺序；可以跳过不适用的探测，但不能倒退或重放。
func (p SandboxProcessPurpose) Order() (int, bool) {
	switch p {
	case SandboxProcessClaudeSandboxProbe:
		return 1, true
	case SandboxProcessClaudeRestrictedProbe:
		return 2, true
	case SandboxProcessVersionProbe:
		return 3, true
	case "", SandboxProcessRuntime:
		return 4, true
	default:
		return 0, false
	}
}
