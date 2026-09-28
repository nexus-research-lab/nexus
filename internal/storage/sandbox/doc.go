// INPUT: 宿主桌面沙箱策略、启动意图、原生登记与 runtime 生命周期阶段。
// OUTPUT: owner/session/generation 复合身份下的策略回执与一次性启动事实。
// POS: runtime effective-policy receipt 的数据库审计边界；回执不授予权限，也不证明 OS 隔离。
package sandbox

// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：策略回执的幂等写入、阶段收口与 owner-scoped 读取。
//   - process_repository.go / process_validation.go：宿主启动意图、原生集合登记、一次性放行领取
//     与 exact 回收事实；独立于连接后策略回执，但共享 owner/session/generation。
//
// 暴露接口：Repository、NewRepository；Repository 提供 Save、UpdatePhase、
// Get 与 Latest 的 owner-scoped 读写；进程事实提供 PrepareProcess、RegisterProcess、
// ClaimProcessRelease、AbortPreparedProcess、ReapProcess、Process 与 LatestProcess。
//
// [PROTOCOL]: 变更时更新父级入口 internal/storage/doc.go。
