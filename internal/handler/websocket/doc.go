// Package websocket 承载实时 WebSocket 连接、分发、订阅注册与广播。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handler.go / handler_connection.go / handler_dispatch.go / handler_control.go /
//     handler_error.go / handler_values.go：连接生命周期、消息分发、控制、错误、取值。
//   - handler_room_subscription.go / handler_session_binding.go / handler_broadcast.go：房间订阅、会话绑定、广播。
//   - handler_appserver_goal_rpc.go / goal_event_broadcaster.go：Codex app-server Goal RPC 与事件广播。
//   - registry_*.go：房间/工作区/事件订阅与 runtime 注册表。
//   - live_workspace.go：实时工作区推送。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package websocket
