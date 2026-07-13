// Package websocket 承载实时 WebSocket 连接、分发、订阅注册与广播。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handler.go / connection.go / dispatch.go / control.go / error.go / values.go：
//     连接生命周期、消息分发、控制动作表、错误与取值。
//   - room_subscription_handler.go / session_binding.go / broadcast.go：
//     房间订阅（含权威空 slot 快照清理）、会话绑定与广播。
//   - goal_rpc_handler.go / goal_rpc_registry.go / goal_event_broadcaster.go：
//     Codex app-server Goal RPC、pending call 注册与事件广播。
//   - app_event_subscription.go / room_subscription_registry.go / workspace_*.go：
//     房间、工作区、事件订阅的引用状态转换与 runtime 快照广播。
//   - live_workspace.go：实时工作区推送。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package websocket
