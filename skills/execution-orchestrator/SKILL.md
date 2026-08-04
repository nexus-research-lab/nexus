---
name: execution-orchestrator
description: 当任务可能需要在直接执行、Task/Todo、Subagent、Plan/WorkGraph、Room Assignment、Gate/Loop 或 Goal 之间做选择，或当前 round 已包含 nexus_execution_context 时使用。负责选择最小充分结构并按需加载图控制或通信参考；不把复杂度、参与人数或一次 @ 自动升级为托管流程。
---

# Execution Orchestrator

把本 Skill 当作编排导航，不当作固定流水线。先完成任务，再让工作图忠实反映真正发生的过程。

稳定语义只有一句：

> Goal 决定持续追求什么；Plan 决定工作怎样展开；Work Item 决定谁交付什么；子智能体帮助一个 Agent 完成自己的工作项；Room 负责让多个 Agent 的工作项可见地交接和协同。

## 使用步骤

1. 先读取当前任务事实和 `<nexus_execution_context>`。存在 context 时，以其中的 lane、binding、snapshot revision、依赖和 `allowed_actions` 为准。
2. 从直接执行开始，逐项判断是否真的需要局部记忆、上下文隔离、独立责任、持久拓扑、条件检查或跨执行边界持续性。
3. 只加入价值高于协调成本的结构；这些能力可以组合，也可以一个都不用。
4. 根据当前决策只读取下面最相关的参考文件，并完整读完该文件再行动。
5. 执行任务并持续消费真实结果。状态工具记录事实，图负责展示状态，最终回复优先交付内容而不是复述流程。

## 最小选择表

| 真实需要 | 首选表达 |
| --- | --- |
| 连贯且可在当前上下文直接完成 | 直接 Agent Loop |
| 当前 Agent 容易遗漏局部步骤 | Task/Todo |
| 隔离上下文、专业视角或局部并行有净收益 | Subagent |
| 出现独立 owner、交付、验收或接管 | Work Item + Assignment |
| 真实依赖、并行分支或恢复点值得持久化 | Plan/WorkGraph |
| 持久 Agent 之间需要跨轮责任与可见交接 | Room Assignment |
| 检查结果会改变下一步路线 | Gate；需要再运行时形成 Loop |
| objective 应跨 round、等待、中断或上下文边界继续存在 | Goal；随后加载 `goal-manager` |

## 按需参考

- 决定 **Task、Subagent、Work Item、Room 或 Goal 的层级与组合**时，读取 [references/structure-selection.md](references/structure-selection.md)。
- 设计或修改 **依赖、并行、审核、Gate、Loop、追加节点或 replan** 时，读取 [references/graph-control.md](references/graph-control.md)。
- 处理 **Room `@`、父子 Agent 消息、MCP 状态标记、Bridge 观测、handoff 后连续执行**时，读取 [references/communication-and-continuity.md](references/communication-and-continuity.md)。

## 稳定边界

- 参与人数、任务复杂度、Plan 长度或 Subagent 数量本身都不是触发器。
- 普通聊天、brainstorm、投票和一次性帮助可以只走消息，不必建图。
- Lead/创建者也是执行者，可以研究、整合、自审、接管和最终交付。
- 不为展示效果制造 Tool、Subagent、Gate 或审核节点；Bridge 和后端会投影真实运行。
- 节点启动后继续推进到真实交付、具体外部阻塞或终态，不因一次 handoff 要求用户发送“继续”。
