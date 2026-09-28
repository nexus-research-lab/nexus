// INPUT: 宿主签发的 exact owner/session/generation 与原生进程集合登记。
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

// SandboxProcessIntent 不含命令参数、环境、Provider 凭据或任务正文。
type SandboxProcessIntent struct {
	Key          SandboxProcessKey `json:"key"`
	Version      int               `json:"version"`
	RuntimeKind  string            `json:"runtime_kind"`
	LeaseID      string            `json:"lease_id,omitempty"`
	BootID       string            `json:"boot_id"`
	OwnerUID     uint32            `json:"owner_uid"`
	JobLabel     string            `json:"job_label"`
	HelperSHA256 string            `json:"helper_sha256"`
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
