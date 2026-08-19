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

只有需要读取或改变受管 WorkGraph 时才调用 CLI。使用宿主注入的 `NEXUS_COMMAND_PATH`；身份、Session、Room role、WorkBinding、ReviewBinding、Goal authority 和 physical round 都来自宿主。直接执行下面的受管命令，不要先用 `echo`、`printenv`、`env`、`set`、`test -n` 或同类命令探测注入变量。不要覆盖 `NEXUS_COMMAND_*`，不要使用 `nexusctl` 或其他编排管理入口。

1. 先读取当前 actor 的权威状态。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution inspect
   ```

   PowerShell runtime 使用 `& "${env:NEXUS_COMMAND_PATH}" ...`，不要混用变量语法。

   只有需要读取同一可信 scope 中的明确历史 Execution 时，才使用 contract 返回的显式历史模板：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution inspect --execution-id '<execution-id>'
   ```

   该 locator 只选择可读快照，不授予 coordinator、WorkBinding、ReviewBinding 或 Goal authority。

2. 从 `allowed_actions` 选择一个操作。每次新的 mutation 输入写入前，都必须紧邻写入重新读取该操作的精确 contract；只有状态不足以确定 operation 名称时才读取完整目录，不把完整目录作为每次调用的固定前置。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution contract --operation assign_work
   ```

3. 只使用刚刚返回的 contract 中的 `input_staging.path`。这是宿主为当前 physical round 预建且初始内容为 `{}` 的私有文件；不要复用记忆、旧输出或上一轮中的绝对路径。某个新返回路径首次写入前先用 Read 工具读一次，再用 Write 覆盖为一个完整 JSON 对象。即使同轮之前调用过同一 operation，新意图也先重读 exact contract，再覆盖当前路径。不要用 shell 重定向、heredoc、`cat` 或命令替换拼 JSON。

4. 用一条单行命令调用。`invoke` 只读取当前 physical round 的宿主管理输入槽，不接受 inline JSON 或调用方选择的文件。每个新意图使用一个 8–128 位稳定 `request_id`，重试同一意图时复用。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution invoke --operation assign_work --request-id 'execution-assign-UNIQUE'
   ```

5. 只把 `is_error=false` 且宿主 applied receipt 对应的结果当成状态变化。按返回的 `next_actions[].domain` / `operation` 行动；普通 snapshot revision 冲突或 `context_status=refresh_required` 可在同一 physical round 重新 `inspect`，不能把它解释成等待宿主换轮；权限拒绝不能通过改身份字段绕过。只有 `context_status=round_refresh_required` 表示 Goal/Execution authority 已被外部换代：立即结束本轮，不再 `inspect`、重写 Plan 或重试 invoke，等待宿主启动 successor round。

输入槽是 round 私有传输介质，不是状态源。CLI 的 operation contract 是当前参数、枚举与 Plan Document 约束的唯一真相；Skill 不复制完整 schema。

创建 Goal+WorkGraph 时必须串行：先由 `goal-manager` 完成 `create_goal`，确认 applied 后再 `prepare_plan_execution`，并在外层输入使用 `goal_binding=current`；两者不得并行。已有 transient WorkGraph 后才出现明确 Goal 意图时，不再 `create_goal`，而使用 `promote_execution_to_goal`。Composer 已创建 Goal 时由宿主启动携带 exact revision 的新 round，再按同一 `goal_binding=current` 路径建图。

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
- exact Room conversation 的成员可以用 `get_execution` 读取共享图的目标、拓扑和状态；`observation` lane 只表示可见性，不是 WorkBinding、ReviewBinding 或 coordination authority，不能据此提交、审核或修改图。
- 持久成员责任一旦成立，先建图并 materialize，再按刷新后的 context Assignment；裸 `@` 不创建责任。
- 无依赖只表示 Work Item 可同时 Ready；实际并行需要不同 Room Agent slot 或真实 Subagent。
- 不为展示制造 Tool、Subagent、Gate 或审核节点；Bridge 投影真实运行。
- 节点启动后推进到真实交付、具体外部阻塞或终态，不因 handoff 要求用户发送“继续”。
