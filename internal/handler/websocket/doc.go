// Package websocket 承载实时 WebSocket 连接、分发、订阅注册与广播。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handler.go / connection.go / dispatch.go / control.go / control_dispatcher.go / error.go / values.go：
//     连接生命周期、消息分发、带请求身份 ACK（含独立 set_goal 与精确 interrupt 完成确认）的控制动作表；
//     set_goal 在同步授权后进入保序、有界、断连可排空的 detached 队列，错误与取值留在 transport 层。
//   - room_subscription_handler.go / session_binding.go / broadcast.go / automation_permission_events.go：
//     房间订阅（含按 Conversation/Session source 隔离的权威活动快照）、会话绑定、Session 元数据、热缓存上下文和 Automation 持久权限按 Agent 重放、广播。
//   - command_catalog.go：在 bind_session 时选择 Nexus 内置的 runtime 清单，合并
//     Nexus host 命令，并把安全权威目录投影到共享或私有 Composer。
//   - goal_rpc_handler.go / goal_rpc_registry.go / goal_event_broadcaster.go：
//     Codex app-server Goal RPC、并发/绑定冲突的稳定 server-error reason_code、授权成功后的 owner/thread 双重隔离订阅注册与事件广播。
//   - execution_invalidation.go：只把 orchestration 成功 mutation 投影为 owner/session 双重隔离的 WorkGraph 失效事件。
//   - app_event_subscription.go / room_subscription_registry.go / workspace_*.go：
//     房间、工作区、事件订阅的引用状态转换与 runtime 快照广播。
//   - live_workspace.go：实时工作区推送。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package websocket
