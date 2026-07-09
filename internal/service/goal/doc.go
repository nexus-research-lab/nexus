// Package goal 负责 Goal 状态机、审计事件与后续运行时续跑决策。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / state_machine.go / service_transition.go：Service 与状态机、状态迁移。
//   - service_continuation.go / service_progress.go / service_resume.go / service_steering.go：续跑、进展记录、恢复、导向。
//   - service_context.go / service_runtime.go / runtime_*.go：运行时上下文、accounting、wall clock。
//   - service_objective.go / service_preview.go / service_appserver.go：目标改写、预览填充、Codex app-server 语义。
//   - service_room_collaboration.go / service_tool.go / service_event.go / service_cleanup.go / service_helpers.go：Room 协作证据、模型工具完成、事件广播、清理、辅助。
//   - errors.go / repository.go：错误与持久化。
//
// Codex app-server 协议模型见子包 appserver/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
