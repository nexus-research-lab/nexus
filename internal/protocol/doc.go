// Package protocol 是跨 HTTP/WebSocket/前端/运行时边界共享的协议真相源。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 只放跨边界共享的协议模型、枚举、事件构造和代码生成输入；服务内部输入、仓储 DTO、
// 持久化 codec 留在对应 internal/service/* 或 internal/storage/*。
//
// 成员清单（按域，本包整体即协议模型，故文件不再加 model_ 前缀）：
//   - agent.go / skill.go：Agent 模型、平台/用户级外部 Skill 引用、显式停用名称与创建/更新协议。
//   - session*.go：Session / Message / SessionKey 统一会话模型与 transcript 原生消息边界。
//   - room*.go：房间、成员、每 Room 唯一未开始 conversation draft、directed message。
//   - conversation_turn.go / event.go / goal*.go / execution*.go / input_queue.go：
//     对话投影、统一事件类型、session-scoped command catalog 与带 public handoff 关联的权威 runtime slot 快照、Goal 生命周期/objective revision、actual/budget token 双口径、最终 usage report/fence、child checkpoint/lifecycle evidence、Room parent terminal ledger 与 durable scope 回补、输入队列快照、持久接受 ACK 及互斥 work/review capability envelope 校验。
//     execution*.go 额外定义 Goal 可选绑定下的 Execution、typed predecessor successor linkage、immutable Plan revision、stable Work Item/spec、模型执行契约单一集合上限、固定 subagent reconciliation grace、typed canonical output scope 与跨平台保守比较键、Assignment、dispatch outbox、跨 Room queue/slot/runtime 的完整 WorkBinding、含 parent-exit reconciliation deadline 的 Attempt、exact-target cancellation outbox/Binding、immutable Submission、独立 review-return outbox/ReviewBinding、append-only Acceptance 与有序幂等事件协议。
//   - chat_attachment.go / workspace_file_artifact.go / delivery_policy.go：
//     聊天附件、工作区文件产物、投递策略。
//   - identity.go / value.go / provider_failure.go：ID 生成、跨边界值解码与稳定 Provider 失败分类。
//   - generate.go / typescript_event.go：前端 TS 类型代码生成入口（go:generate）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package protocol
