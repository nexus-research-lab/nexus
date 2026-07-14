// Package server 装配 HTTP 服务、路由、WebSocket、实时链路与各内建 MCP builder。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go / lifecycle.go / app_services.go / core_services.go：服务生命周期与依赖装配。
//   - routes.go / routes_web.go / handlers.go / websocket.go：HTTP/Web 路由与 WS 入口。
//   - *_mcp.go：automation / connector / goal / imagegen / room 内建 MCP server 装配及 DM runner / Room slot revision 状态绑定。
//   - goal_interrupt.go / goal_resume.go / goal_guidance.go / realtime_invalidation.go：Goal 中断、恢复、DM/Room steering 与实时失效。
//   - channel_external_session.go / dm_external_reply.go：外部通道会话与 DM 外部回复。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package server
