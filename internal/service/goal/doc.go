// Package goal 负责 Goal 状态机、审计事件与后续运行时续跑决策。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / transition.go / session_ownership.go / owner_claim.go：Service、owner-scoped durable 读取、日志装配、外部 metadata 信任边界、状态机与状态迁移、Goal create 的 session 证明及 ownerless legacy Goal 一次性 provenance 认领。
//   - continuation.go / progress.go / resume.go / steering.go：普通续跑与 awaiting_plan 专用 successor-planning continuation 的 durable reserve/leased-claim/leased-start/terminal-settle/retry/release、root receipt 与 Agent audit 双身份终态、revision 安全的进展与 Room collaboration handback 记录、由 mutation/runtime wake + exact retry deadline + 低频审计驱动的跨进程恢复、DM/Room runtime 导向与完成后的 result-first 最终交付提示。
//   - runtime.go / runtime_*.go / source_usage.go / usage_finalization.go：运行时上下文与 permission-mode 策略、scope-aware Goal create preflight/activation 回滚、actual/budget token 增量 accounting、最终 usage 查询/fence、child checkpoint/lifecycle evidence、Room parent terminal ledger、durable scope round-start/from-now 归属与 wall clock。
//   - objective.go / alignment.go / preview.go / appserver.go：目标改写、共享 Objective Alignment 报告的 Goal 生命周期适配、首个 Goal 用户意图的会话标题投影与重启恢复、Codex app-server 语义。
//   - objective_transition.go / execution_binding.go / room.go / tool.go / retarget.go / event.go / cleanup.go / helpers.go：durable objective revision / Execution rebase saga、反向 binding、中央 binding 读面、WorkGraph completion 与 clear gate、Room ownership/authority/collaboration/completion、模型工具状态更新与同 ID 目标替换、事件广播、关联 Goal 探测与清理。
//   - errors.go / repository.go：跨调用方统一错误分类与持久化契约。
//
// Codex app-server 协议模型见子包 appserver/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
