// Package server 装配 HTTP 服务、路由、WebSocket、实时链路与各内建 MCP builder。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go / lifecycle.go / app_services.go / core_services.go：服务生命周期与依赖装配；accepted Review completion audit、Execution -> Goal durable confirmation、Goal 会话标题、Plan proposal 与 parent-exit subagent Attempt 各有跨进程恢复器，Assignment、review-return 与 cancellation outbox 由 Room dispatch 恢复器处理。
//   - routes.go / routes_web.go / handlers.go / websocket.go：HTTP/Web 路由、WS 入口与 orchestration ExecutionInvalidationSink 装配。
//   - *_mcp.go：automation / communication / connector / execution / goal / visualize / imagegen / room 内建 MCP server 装配及 DM runner / Room slot exact Goal/revision/Execution、trusted WorkBinding/ReviewBinding 状态绑定。
//   - execution_goal.go / explicit_goal_execution.go / execution_subagent_history.go：显式/自适应 promotion、单域原子的 goal_only create_goal、fresh Plan 只读继承 canonical Goal objective、历史 reservation 恢复、Plan materialization 或 promotion 驱动的 pending/confirmed Goal/Execution 双向幂等 binding、Goal objective revision rebase saga 与受限 Subagent ToolRun 历史适配。
//   - execution_cancellation.go：把 durable cancellation target 适配到 exact Room slot 或 runtime round，并保留 provider-interrupted、local-cancelled、already-ended 与 unsupported 的真实结果。
//   - goal_command.go / goal_session_ownership.go / goal_interrupt.go / goal_resume.go / goal_guidance.go / realtime_invalidation.go：host Goal command 的 DM/Room 路由、Goal create 的 owner-scoped Agent/Room session 证明、Room runtime 成员身份、中断、恢复、DM/Room steering 与实时失效。
//   - channel_external_session.go / dm_external_reply.go：外部通道会话与 DM 外部回复。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package server
