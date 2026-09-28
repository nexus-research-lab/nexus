// INPUT: 宿主桌面沙箱策略、启动意图、原生登记与 runtime 生命周期阶段。
// OUTPUT: owner/session/generation 复合身份下的策略回执与一次性启动事实。
// POS: runtime effective-policy receipt 的数据库审计边界；回执不授予权限，也不证明 OS 隔离。
package sandbox

// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - lifecycle_recovery.go：已终止进程的资源完成阶段和独立 pending 策略扫描，稳定 launch ID 分页。
//   - scratch_recovery.go / policy_recovery.go：资源清理意图、单调阶段及 pending 栅栏；原进程和 scratch 完成后只 reconcile exact 关联策略。
//   - receipt_process.go：策略代次绑定原 runtime exact key，按原进程代次读取，拒绝猜测旧回执或改绑。
//   - repository.go：策略回执的幂等写入、阶段收口与 owner-scoped 读取。
//   - process_recovery_scan.go：按唯一 launch ID 有界读取跨 owner 未收口 key；只供持锁宿主恢复，不暴露用户端点。
//   - process_repository.go / process_validation.go：宿主启动意图、原生集合登记、一次性放行领取
//     与 exact 回收事实、可选可信 scratch parent/leaf 身份；同代次探测到 runtime 按用途顺序保存独立 launch，仍保持唯一活跃范围，旧单进程登记视为 runtime；独立于连接后策略回执，但共享 owner/session/generation。
//
// 暴露接口：Repository、NewRepository；Repository 提供 Save、UpdatePhase、
// Get 与 Latest 的 owner-scoped 读写；进程事实提供 PrepareProcess、RegisterProcess、
// ClaimProcessRelease、AbortPreparedProcess、ReapProcess、Process、LatestProcess、RuntimeProcess 与 PendingProcessKeys；资源/策略恢复由 PrepareScratchRecovery、ScratchRecovery、AdvanceScratchRecovery、PendingScratchRecovery、ReconcileProcessPolicies、CompleteProcessResources、PendingLifecycleKeys 提供。
//
// [PROTOCOL]: 变更时更新父级入口 internal/storage/doc.go。
