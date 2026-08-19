// Package message 把 runtime/SDK 消息映射并投影为 Nexus 事件与 assistant 快照。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - processor.go / event_mapper.go：SDK 消息分发、状态持有、内部中断归一化、统一事件封装与场景装饰。
//   - system.go / task_event.go / memory_attachment.go：可见系统事件、后台任务事件、记忆引用、同消息多 child token 快照与 shell 易失进度投影。
//   - result_message.go / provider_content_filter.go：assistant API 错误、终态结果消息与 Provider 内容安全拦截归一化。
//   - tool_result.go / workspace_artifact.go：工具结果消息、typed applied mutation 观察、fail-closed Goal 进展与工作区产物投影。
//   - segment_assistant.go / projection_result.go：assistant 分段、show_widget 字符串参数的增量投影、普通流式工具输入、保留执行身份与 failure phase 的结果摘要，以及只公开已知耗时/actual token 的 Goal 完成收据挂载。
//   - guided_input.go：运行中 round 的用户引导历史投影。
//   - helpers.go：共享归一化与单路径 block 投影辅助。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package message
