// Package goal 负责 Goal 状态机、审计事件与后续运行时续跑决策。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / state_machine.go / transition.go / session_ownership.go / owner_claim.go：Service、user-created metadata 的统一信任边界、owner-scoped transport 读取、状态机、状态迁移、Goal create 的 session 证明与 ownerless legacy Goal 一次性 owner provenance 认领边界。
//   - continuation.go / progress.go / resume.go / steering.go：普通续跑与 awaiting_plan 专用 successor-planning continuation 的 reserve/claim/release、revision 安全的进展与 Room collaboration handback 记录、恢复、DM/Room runtime 导向与完成后的 result-first 最终交付提示。
//   - context.go / runtime.go / runtime_*.go / source_usage.go / usage_finalization.go：运行时上下文、scope-aware Goal create preflight/activation 回滚、actual/budget token 增量 accounting、最终 usage 查询/fence、child checkpoint/lifecycle evidence、Room parent terminal ledger、durable scope round-start/from-now 归属与 wall clock。
//   - objective.go / alignment.go / preview.go / appserver.go：目标改写、共享 Objective Alignment 报告的 Goal 生命周期适配、首个 Goal 用户意图的会话标题投影与重启恢复、Codex app-server 语义。
//   - objective_transition.go / execution_binding.go / execution_binding_read.go / execution_completion.go / clear.go / room_metadata.go / model_mutation_authority.go / room_command_update.go / room_collaboration.go / room_completion.go / tool.go / retarget.go / event.go / cleanup.go / helpers.go：durable Goal objective revision / Execution rebase saga、新 Goal-only mode 与 proposal-owned managed binding、含旧显式 Goal reservation fence 与 server-owned reserved/pending/confirmed phase 的反向 Execution binding、严格 owner-scoped 且不持久化的中央 binding 读面、WorkGraph completion 与 clear lifecycle gate、Room creator/lead 权限、负责人新 round 的 exact Goal-only mutation 快照、host Goal replace 的 server-only Room ownership 合并、与 complete 解耦且在同一 Goal 生命周期内单调累计的 revision-safe 协作审计事实、模型工具状态更新与同 ID 目标替换、事件广播、关联 Goal 探测与清理、辅助。
//   - errors.go / repository.go：跨调用方统一的错误分类与持久化契约。
//
// Codex app-server 协议模型见子包 appserver/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
