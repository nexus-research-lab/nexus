// Package provider 是 Provider 配置与模型卡的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储入口、方言短方法与稳定的 CAS/不存在/已回滚错误协议。
//   - mutation.go：Provider/model/default/test/delete/reassign 的共享 configuration_version 事务与 rollback/commit 证据。
//   - provider.go / model.go / usage.go：可见/owner-private/公共 Provider、模型与只读用量入口；现有聚合写入统一委托 Mutation。
//   - scan.go：行扫描。
//   - entity.go：包含 configuration_version 的持久化模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package provider
