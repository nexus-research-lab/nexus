// Package server 装配 HTTP/WebSocket 路由、实时通知与进程生命周期。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go / lifecycle.go：消费 app.AppServices，启动后台协调器；关闭时先排空 HTTP、等待后台退出，再释放共享资源；启动失败逆序回收。
//   - routes.go / routes_web.go / path_param_router.go / http_handlers.go / websocket.go：HTTP/Web/可选 Team 路由、Desktop 线上账号与 Team 同源代理、统一路径段解码（含 Provider 历史 model_id 兼容边界）、HTTP handlerSet 装配、WS 入口与 orchestration ExecutionInvalidationSink 装配。
//   - realtime_invalidation.go / configuration_notifier.go：Session、conversation 标题、定时任务、Agent 与 Room 配置变更到 websocket 实时投影的统一失效通知装配。
//   - channel_external_session.go：外部通道会话到 WebSocket 的通知适配。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package server
