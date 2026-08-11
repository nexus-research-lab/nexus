// Package protocol 是 HTTP、WebSocket、前端与 runtime 共享的 wire truth。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 本包只定义跨边界模型、枚举、事件和代码生成输入。产品行为见
// docs/specs；服务 command 与仓储 DTO 留在 internal/service 和
// internal/storage，不能在本头部复制第二份业务规范。
//
// 成员地图：
//   - agent.go / skill.go：Agent 与 Skill 协议。
//   - session*.go / conversation_turn.go / input_queue.go：会话、消息、轮次、
//     上下文占用和输入队列。
//   - room*.go：Room、成员、conversation draft、directed message 与 runtime slot。
//   - goal*.go / objective_alignment.go：Goal 生命周期、objective revision、
//     Execution binding 五态 resolution、用量与对齐审计。
//   - execution*.go / execution_plan_proposal.go：Execution、Plan/Work Item、
//     Assignment/Attempt/Submission/Acceptance、dispatch/cancellation、Runtime Graph
//     与只读 WorkGraph view。当前语义见 docs/specs/execution-orchestration-spec.md
//     和 docs/specs/execution-graph-spec.md。
//   - event.go / typescript_event.go / generate.go：事件 envelope、
//     owner/session WorkGraph 失效事件与前端 TS 生成。
//   - attachment.go / workspace_file_artifact.go / delivery_policy.go：附件、
//     文件产物与投递策略。
//   - identity.go / value.go / provider_failure.go / tool_result.go：共享 ID、值解码、
//     Provider 失败分类与 mutation outcome。
//
// 主要暴露接口：Goal、ExecutionSnapshot/ExecutionView、ExecutionWorkBinding/
// ExecutionReviewBinding、EventMessage 及 New*Event 构造器；精确字段以对应 Go
// 类型为准，前端事件类型由 typescript_event.go 生成。
//
// [PROTOCOL]: wire 变化时更新本头部，并检查 docs/specs、API_REFERENCE 与 AGENTS.md。
package protocol
