// Package protocol 是跨 HTTP/WebSocket/前端/运行时边界共享的协议真相源。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 只放跨边界共享的协议模型、枚举、事件构造和代码生成输入；服务内部输入、仓储 DTO、
// 持久化 codec 留在对应 internal/service/* 或 internal/storage/*。
//
// 成员清单（按域，本包整体即协议模型，故文件不再加 model_ 前缀）：
//   - session*.go：Session / Message / SessionKey 统一会话模型。
//   - room*.go：房间、成员、directed message、创建请求。
//   - conversation_turn.go / event.go / goal.go / input_queue.go：
//     对话投影、统一事件类型与瞬时 runtime 状态、Goal 生命周期、输入队列面。
//   - chat_attachment.go / workspace_file_artifact.go / delivery_policy.go：
//     聊天附件、工作区文件产物、投递策略。
//   - identity.go / value.go：ID 生成与跨边界值解码。
//   - generate.go / typescript_event.go：前端 TS 类型代码生成入口（go:generate）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package protocol
