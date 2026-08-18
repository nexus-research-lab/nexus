---
name: execution-orchestrator
description: 当 substantial task 需要在直接执行、Task/Todo、Subagent、Plan/WorkGraph、Room Assignment、Gate/Loop 或 Goal 之间做选择，当前 round 已包含 nexus_execution_context，或需要通过 nexus execution CLI 读取和更新 WorkGraph 时使用。负责选择最小充分结构并在 exact round authority 下记录责任、交付、审核与恢复事实；复杂度只触发评估，不机械映射到托管流程。
---

# Execution Orchestrator

把本 Skill 当作编排导航，不当作固定流水线。先完成任务，再让工作图忠实反映真正发生的过程。

> Goal 决定持续追求什么；Plan 决定工作怎样展开；Work Item 决定谁交付什么；Subagent 帮助一个 Agent 完成自己的责任；Room 让多个持久 Agent 可见地交接和协同。Goal 与 WorkGraph 独立选择，只在确实同时需要持续性与责任拓扑时绑定。

## 结构选择

1. 先读取任务事实和 `<nexus_execution_context>`。以其中的 lane、binding、snapshot revision、依赖和 `allowed_actions` 为准。
2. substantial execution 前评估任务是否原子、哪些子问题可拆分，以及上下文隔离、专业视角、局部并行或独立验证是否有净收益。
3. 同一 Agent 对整体交付负责时，用一个责任承载整体并在内部启动 Subagent；需要持久 owner、交付、验收、接管或恢复拓扑时才建立 Work Item/WorkGraph。
4. 只加入价值高于协调成本的结构；这些能力可以组合，也可以一个都不用。
5. 根据当前决策完整读取一个最相关参考：
   - 层级与组合：[references/structure-selection.md](references/structure-selection.md)
   - 依赖、Plan、Review、Gate、Loop、replan：[references/graph-control.md](references/graph-control.md)
   - Room/父子 Agent 内容传递与连续执行：[references/communication-and-continuity.md](references/communication-and-continuity.md)

## Execution 命令工作流

只有需要读取或改变受管 WorkGraph 时才调用 CLI。使用宿主注入的 `NEXUS_COMMAND_PATH`；身份、Session、Room role、WorkBinding、ReviewBinding、Goal authority 和 physical round 都来自宿主。不要覆盖 `NEXUS_COMMAND_*`，不要使用 `nexusctl` 或其他编排管理入口。

1. 读取当前目录和状态。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution contract
   "${NEXUS_COMMAND_PATH}" --json execution inspect
   ```

   PowerShell runtime 使用 `& "${env:NEXUS_COMMAND_PATH}" ...`，不要混用变量语法。

2. 从 `allowed_actions` 选择一个操作，再按需读取精确 contract。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution contract --operation assign_work
   ```

3. 从 contract 输出读取 `input_staging.path`。这是宿主预建且初始内容为 `{}` 的文件：每个 physical round 第一次写入前，先用 Read 工具读该路径一次，再用 Write 覆盖为一个完整 JSON 对象；同轮后续新意图直接覆盖旧内容。不要用 shell 重定向、heredoc、`cat` 或命令替换拼 JSON。

4. 用一条单行命令调用。每个新意图使用一个 8–128 位稳定 `request_id`，重试同一意图时复用。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution invoke --operation assign_work --input-file "${NEXUS_COMMAND_INPUT_PATH}" --request-id 'execution-assign-UNIQUE'
   ```

5. 只把 `is_error=false` 且宿主 applied receipt 对应的结果当成状态变化。按返回的 `next_actions[].domain` / `operation` 行动；revision 冲突、权限拒绝或 lane 改变时重新 `inspect`，不修改身份字段绕过。

输入槽是 round 私有传输介质，不是状态源。CLI 的 operation contract 是当前参数、枚举与 Plan Document 约束的唯一真相；Skill 不复制完整 schema。

## 最小选择表

| 真实需要 | 首选表达 |
| --- | --- |
| 当前上下文可连贯完成 | 直接 Agent Loop |
| 当前 Agent 容易遗漏局部步骤 | Task/Todo |
| 隔离上下文、专业视角或局部并行有净收益 | Subagent |
| 独立 owner、交付、验收或接管 | Work Item + Assignment |
| 真实依赖、并行分支或恢复点值得持久化 | Plan/WorkGraph |
| 持久 Agent 之间跨轮交接 | Room Assignment |
| 检查结果会改变路线 | Gate；需要再运行时形成 Loop |
| objective 应跨 round、等待或中断继续存在 | Goal；随后加载 `goal-manager` |

## 稳定边界

- 复杂度是评估信号，不是固定路由。
- Goal 与 WorkGraph 分别选择；只信 exact Goal context 和明确绑定意图。
- 普通聊天、brainstorm、投票和一次性帮助只走消息，不必建图。
- 持久成员责任一旦成立，先建图并 materialize，再按刷新后的 context Assignment；裸 `@` 不创建责任。
- 无依赖只表示 Work Item 可同时 Ready；实际并行需要不同 Room Agent slot 或真实 Subagent。
- 不为展示制造 Tool、Subagent、Gate 或审核节点；Bridge 投影真实运行。
- 节点启动后推进到真实交付、具体外部阻塞或终态，不因 handoff 要求用户发送“继续”。
