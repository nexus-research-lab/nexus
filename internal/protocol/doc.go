// Package protocol 是跨 HTTP/WebSocket/前端/运行时边界共享的协议真相源。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 只放跨边界共享的协议模型、枚举、事件构造和代码生成输入；服务内部输入、仓储 DTO、
// 持久化 codec 留在对应 internal/service/* 或 internal/storage/*。
//
// 成员清单（按域，本包整体即协议模型，故文件不再加 model_ 前缀）：
//   - agent.go / skill.go：Agent 模型、同 owner 联系人、平台/用户级外部 Skill 引用、显式停用名称与创建/更新协议。
//   - session*.go：Session / Message / SessionKey 统一会话模型、持久化上下文占用快照与 transcript 原生消息边界。
//   - room*.go：房间、联系人内部通道用途、成员持久 participation gate、每 Room 唯一未开始 conversation draft、directed message。
//   - conversation_turn.go / event.go / goal*.go / objective_alignment.go / execution*.go / execution_plan_proposal.go / input_queue.go：
//     对话投影、统一事件类型、Room member participation change、session-scoped command catalog、精确 interrupt 完成 ACK 与带 public handoff 关联的权威 runtime slot 快照、Goal 生命周期/objective revision、显式 Goal 的稳定 Execution 预留与旧记录确定性恢复、actual/budget token 双口径、最终 usage report/fence、child checkpoint/lifecycle evidence、Room parent terminal ledger 与 durable scope 回补、输入队列快照、持久接受 ACK 及互斥 work/review capability envelope 校验。
//     objective_alignment.go 定义 Goal completion 与 Execution loop guard 共用、但不拥有任一生命周期的逐 criterion evidence、gap 和 aligned/not_aligned/inconclusive 三态审计协议。
//     execution*.go 额外定义 Goal 可选绑定下的 Execution、typed predecessor successor linkage、immutable Plan revision、stable Work Item/spec、模型执行契约单一集合上限、固定 subagent reconciliation grace、typed canonical output scope 与跨平台保守比较键、Assignment、dispatch outbox、跨 Room queue/slot/runtime 的完整 WorkBinding、含 parent-exit reconciliation deadline 的 Attempt、exact-target cancellation outbox/Binding、immutable Submission、跨 Agent review-return outbox/ReviewBinding、append-only Acceptance、有序幂等事件协议，以及不暴露 capability identity 的 Web WorkGraph、review Gate 与带有界结果/错误摘要、NodeRun 历史、到达顺序无关 Artifact ref、显式 partial/total、可解释控制回边和 exact retry 关联的 provider-neutral Runtime NodeRun/EdgeRun 只读投影。execution_plan_proposal.go 定义独立于权威 Plan revision、已按 Goal/document/current Execution 补全 canonical root boundary 的 sealed proposal；digest 覆盖 trusted authority/round/target 与 Goal activation/reserved successor/predecessor，可变部分只保存 materialization receipt、CAS lease/retry 与 Goal confirmation 恢复状态。
//     event.go 同时承载 runtime 每轮结束后的 Agent session 上下文占用事件。
//   - chat_attachment.go / workspace_file_artifact.go / delivery_policy.go：
//     聊天附件、工作区文件产物、投递策略。
//   - identity.go / value.go / provider_failure.go / tool_result.go：ID 生成、跨边界值解码、稳定 Provider 失败分类，以及工具传输状态之外的显式 mutation 结果语义。
//   - generate.go / typescript_event.go：前端 TS 类型代码生成入口（go:generate）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package protocol
