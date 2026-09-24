// INPUT: 宿主确认的桌面沙箱策略、runtime 代次与生命周期阶段。
// OUTPUT: owner/session/generation 复合身份下的持久化策略回执。
// POS: runtime effective-policy receipt 的数据库审计边界；回执不授予权限，也不证明 OS 隔离。
package sandbox

// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：策略回执的幂等写入、阶段收口与 owner-scoped 读取。
//
// 暴露接口：Repository、NewRepository；Repository 提供 Save、UpdatePhase、
// Get 与 Latest 的 owner-scoped 读写。
//
// [PROTOCOL]: 变更时更新父级入口 internal/storage/doc.go。
