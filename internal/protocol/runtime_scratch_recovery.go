// INPUT: 原启动 key、可信 scratch 身份与宿主清理阶段。
// OUTPUT: 跨崩溃可恢复的资源回收记录；不表示业务动作已完成。
// POS: 宿主内部恢复合同，任务不可提供目标路径或改写阶段。
package protocol

type SandboxScratchRecoveryPhase string

const (
	SandboxScratchRecoveryPrepared    SandboxScratchRecoveryPhase = "prepared"
	SandboxScratchRecoveryQuarantined SandboxScratchRecoveryPhase = "quarantined"
	SandboxScratchRecoveryDeleting    SandboxScratchRecoveryPhase = "deleting"
	SandboxScratchRecoveryComplete    SandboxScratchRecoveryPhase = "complete"
)

type SandboxScratchRecovery struct {
	ProcessKey SandboxProcessKey
	LeaseID    string
	Scratch    SandboxProcessScratch
	Phase      SandboxScratchRecoveryPhase
}
