// Package provider 是 Provider 配置与模型卡的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储入口与共享事务边界。
//   - provider.go / model.go / usage.go：Provider、模型与用量读写。
//   - dialect.go / scan.go：方言适配与行扫描。
//   - model_provider.go：Entity 等持久化模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package provider
