// Package server 装配 HTTP 服务、路由、WebSocket、实时链路与各内建 MCP builder。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go / lifecycle.go / app_services.go / core_services.go：服务生命周期与依赖装配；accepted Review completion audit、Execution -> Goal durable confirmation、Goal 会话标题、Plan proposal 各有跨进程恢复器，parent-exit subagent Attempt 按最近 durable deadline 和变更事件唤醒，Assignment、review-return 与 cancellation outbox 由 Room dispatch 恢复器处理。
//   - runtime_auth_transition.go / agent_deletion_coordinator.go：认证启用前阻断 admission、撤销 system owner runtime 的原子转场，与数据库提交点后立即撤销 Agent runtime 的跨域删除协调。
//   - routes.go / routes_web.go / http_handlers.go / websocket.go：HTTP/Web 路由、HTTP handlerSet 装配、WS 入口与 orchestration ExecutionInvalidationSink 装配。
//   - *_mcp.go / channel_authorization.go / connector_authorization.go / runtime_mcp_authority.go：authorization / communication / connector / visualize / imagegen / room 内建 MCP server 按 Session 拓扑稳定装配；Channel/Connector 真人授权 builder 与共享可信身份判定在 *_authorization.go 与 runtime_mcp_authority.go；Goal/Execution/Automation 不再挂载 MCP。
//   - configuration_runtime.go / runtime_command.go / runtime_command_input.go：nexuscfg loopback broker 与 Agent-facing nexus CLI 的 Goal/Execution/Automation command broker、command capability 环境、round 私有 0600 JSON 输入槽、调用时读取的动态 SDK Session identity 与 typed mutation receipt；不直接向 runtime 开放数据库。
//   - execution_command_context.go / goal_command_context.go：DM/Room runtime 每次 command 原子读取 Goal/Execution/Work/Review identity 的权威装配边界；私有 Goal authority 不泄漏为 Execution authority。
//   - execution_goal_promotion.go / explicit_goal_execution.go / execution_subagent_history.go：Execution 晋升 adaptive Goal、单域原子的 goal_only create_goal、fresh Plan 只读继承 canonical Goal objective、历史 reservation 恢复、Plan materialization 或 promotion 驱动的 pending/confirmed Goal/Execution 双向幂等 binding、Goal objective revision rebase saga 与受限 Subagent ToolRun 历史适配。
//   - execution_cancellation.go：把 durable cancellation target 适配到 exact Room slot 或 runtime round，并保留 provider-interrupted、local-cancelled、already-ended 与 unsupported 的真实结果。
//   - goal_command.go / goal_session_ownership.go / goal_interrupt.go / goal_resume.go / goal_guidance.go：host Goal command 的 DM/Room 路由、Goal create 的 owner-scoped Agent/Room session 证明、Room runtime 成员身份、中断、恢复与 DM/Room steering。
//   - realtime_invalidation.go / configuration_notifier.go：Session、conversation 标题、定时任务、Agent 与 Room 配置变更到 websocket 实时投影的统一失效通知装配。
//   - human_tool_approval.go：runtime permission 人工 allow 按工具域路由到 configuration 或 Connector durable approval recorder。
//   - channel_external_session.go / dm_external_reply.go：外部通道会话与 DM 外部回复。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package server
