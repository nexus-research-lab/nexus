// Package configuration 提供 Nexus 配置控制面：统一发现、读取、预检、变更、校验与审计。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - model.go / catalog.go：配置域、检查、变更计划与审计协议。
//   - service.go / snapshot.go：服务装配、按 owner 读取与配置健康检查。
//   - change.go / change_validate.go：乐观并发、幂等、确认门槛与严格预检。
//   - change_input.go / change_dispatch.go / preferences_mutation.go：补丁适配、领域分派与 Preferences 回滚。
//   - audit.go：配置变更审计仓储。
//   - sanitize.go：任意配置树的凭据与内部提示词脱敏。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package configuration
