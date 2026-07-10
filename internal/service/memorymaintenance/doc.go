// Package memorymaintenance 负责由 Nexus 唤醒 nxs 的后台记忆维护检查。
//
// 本包只拥有时钟、Agent 生命周期和失败重试；AutoDream 是否到期以及如何整理
// 记忆仍由 nxs 决定，避免产品后端复制 runtime 领域规则。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - coordinator.go：启动、扫描、并发去重和下一次检查时间。
//   - runner.go：解析 Agent/provider/background model 并同步调用 nxs。
//   - settings.go：读取 Agent workspace 中的 AutoDream 开关。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package memorymaintenance
