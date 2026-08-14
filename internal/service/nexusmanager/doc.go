// Package nexusmanager 提供受控的 Nexus 资源只读查询。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / access.go：窄依赖装配，以及每次调用的 owner、Agent、DM/Room、active round 与业务上下文/lease 不可转移重校验。
//   - query.go：脱敏资源查询与 workspace 只读。
//   - model.go / projection.go：MCP 稳定输出模型、host-only 动态协作归因入口及秘密/内部标识裁剪。
//
// 本包不承载创建、配置写入、删除、认证、用户、Provider、Channel、Connector credential
// 或 Automation 能力；这些边界必须继续由各自控制面服务持有。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package nexusmanager
