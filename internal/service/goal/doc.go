// Package goal 负责 Goal 状态机、审计事件与后续运行时续跑决策。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / state_machine.go / transition.go：Service 与状态机、状态迁移。
//   - continuation.go / progress.go / resume.go / steering.go：续跑 reserve/claim/release、revision 安全的进展记录、恢复、DM/Room runtime 导向。
//   - context.go / runtime.go / runtime_*.go：运行时上下文、accounting、wall clock。
//   - objective.go / preview.go / appserver.go：目标改写、预览填充、Codex app-server 语义。
//   - room_collaboration.go / tool.go / retarget.go / event.go / cleanup.go / helpers.go：revision 安全的 Room 协作证据、模型工具状态更新与同 ID 目标替换、事件广播、清理、辅助。
//   - errors.go / repository.go：跨调用方统一的错误分类与持久化契约。
//
// Codex app-server 协议模型见子包 appserver/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
