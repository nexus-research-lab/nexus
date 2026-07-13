// Package message 把 runtime/SDK 消息映射并投影为 Nexus 事件与 assistant 快照。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - processor.go / event_mapper.go：SDK 消息分发、状态持有与场景事件映射。
//   - system.go / task_event.go：可见系统事件与后台任务事件投影。
//   - assistant_error.go / result_message.go：assistant API 错误与终态结果消息。
//   - tool_result_message.go / workspace_artifact.go：工具结果消息与工作区产物投影。
//   - segment_assistant.go / projection_result.go / tool_result.go：assistant 分段、结果摘要挂载、工具结果观测。
//   - factory_guidance.go：运行中 round 的用户引导消息。
//   - codes_permission_error.go：结构化权限错误码推导。
//   - helpers.go：共享归一化与单路径 block 投影辅助。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package message
