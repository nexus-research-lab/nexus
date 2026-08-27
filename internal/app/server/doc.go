// Package server 是 HTTP 进程的组合根，装配路由、生命周期、服务与内建 MCP。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go / lifecycle.go / app_services.go / core_services.go：HTTP 进程生命周期与总依赖装配；跨进程恢复器和后台 worker 仍由组合根启动。
//   - runtime_auth_transition.go / agent_deletion_coordinator.go：认证启用前阻断 admission、撤销 system owner runtime 的原子转场，与数据库提交点后立即撤销 Agent runtime 的跨域删除协调。
//   - routes.go / routes_web.go / http_handlers.go / websocket.go：HTTP/Web 路由、HTTP handlerSet 装配、WS 入口与 orchestration ExecutionInvalidationSink 装配。
//   - realtime_invalidation.go / configuration_notifier.go：Session、conversation 标题、定时任务、Agent 与 Room 配置变更到 websocket 实时投影的统一失效通知装配。
//   - channel_external_session.go / dm_external_reply.go：外部通道会话与 DM 外部回复。
//   - goal/：Goal 命令、会话所有权、续跑与 Goal/Execution 绑定。
//   - execution/：Execution command context、精确取消与 Subagent 历史投影。
//   - workgraph/：WorkGraph 隐藏编辑 Session 与隔离保存 round。
//   - runtime/：round-scoped nexus MCP、配置 broker、Connector/Channel 授权与内建 runtime 工具装配。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package server
