// Package protocol 是 HTTP、WebSocket、前端与 runtime 共享的 wire truth。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 本包只定义跨边界模型、枚举、事件和代码生成输入。产品行为见
// docs/specs；服务 command 与仓储 DTO 留在 internal/service 和
// internal/storage，不能在本头部复制第二份业务规范。
//
// 成员地图：
//   - agent.go / agent_private.go / skill.go：Agent 运行时画像、独立业务标签、owner-scoped 创建对账结果、同 owner 联系人、可游标翻页的私域消息投影、受控执行工具策略与 Skill 协议。
//   - session*.go / message_annotation.go / input_queue.go：会话、消息、轮次、
//     Connector 继承/显式选择快照与待物化 runtime fork 边界、
//     公开 handoff 回复因果注解、外部 IM 身份、Goal 完成收据、记忆引用、
//     上下文占用和持久输入队列。
//   - room*.go：Room、可按好友对恢复的联系人内部通道、成员 participation gate、
//     唯一 conversation draft、directed message、public handoff 与 runtime slot。
//   - goal*.go / objective_alignment.go：Goal 生命周期、独立 host Goal command、
//     objective revision、非授权 Room collaboration attribution、Execution binding
//     五态 resolution、用量与对齐审计。
//   - execution*.go / execution_plan_proposal.go / workgraph_workflow.go / workgraph_artifact.go：Execution、Plan/Work Item、
//     Assignment/Attempt/Submission/Acceptance、dispatch/cancellation、Runtime Graph、
//     画布专用 append-only WorkGraph history、只读 WorkGraph view 与不含运行事实的
//     owner-scoped 命名工作图，以及普通对话中可直接渲染的完整草图快照。当前语义见 docs/specs/execution-orchestration-spec.md
//     和 docs/specs/execution-graph-spec.md。
//   - automation_run.go / im_permission_command.go：受控 Automation 运行上下文与
//     普通 runtime/Automation 共用的 IM 权限短命令。
//   - failure.go / event.go / conversation_failure.go / typescript_*.go / generate.go：统一 FailureCore、事件 envelope、Conversation 用户级失败分类、session-scoped
//     command catalog、interrupt ACK、聊天 source 活动快照、WorkGraph/Subagent 失效事件、上下文占用事件与前端 TS 生成。
//   - attachment.go / workspace_file_artifact.go / delivery_policy.go：附件、
//     文件产物与投递策略。
//   - identity.go / value.go / provider_failure.go / tool_result.go /
//     configuration_runtime.go / runtime_command.go：共享 ID、值解码、Provider
//     失败分类、mutation outcome，以及 nexuscfg / Agent-facing nexus broker wire 常量。
//
// 主要暴露接口：Goal、ExecutionSnapshot/ExecutionView、ExecutionWorkBinding/
// ExecutionReviewBinding、EventMessage 及 New*Event 构造器；精确字段以对应 Go
// 类型为准，前端事件类型由 typescript_event.go 生成。
//
// [PROTOCOL]: wire 变化时更新本头部，并检查 docs/specs、docs/README.md 与 AGENTS.md。
package protocol
