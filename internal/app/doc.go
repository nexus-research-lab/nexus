// Package app 为 HTTP 和 CLI 显式装配共享业务服务与宿主适配。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - app_services.go / core_services.go：共享依赖图与数据库所有权；AppServices.Close 统一释放标题任务、授权任务、Browser 与自有数据库。
//   - agent_deletion_coordinator.go / dm_external_reply.go：Agent 删除与外部回复的跨域宿主适配。
//   - goal/：会话所有权、命令路由、引导、中断与续跑的 DM/Room 适配。
//   - execution/ / workgraph/：执行取消、命令上下文、历史投影与隐藏编辑会话适配。
//   - runtime/：round-scoped MCP、配置 broker、授权与内建工具装配。
//   - server/：HTTP/WebSocket 路由、实时通知及后台协调器的启停。
//
// Goal/Execution 跨域业务协调归 service/goalexecution，身份失效规则归 service/auth。
// app 不依赖 app/server；CLI 直接消费共享装配，不启动 HTTP 后台协调器。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package app
