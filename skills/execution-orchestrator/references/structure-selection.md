# 结构选择

只在需要决定执行层级时读取本文件。每种结构解决不同问题。复杂度必须触发结构评估，但不能直接推导出一整套固定流程。

substantial execution 前，每个 Agent 都先判断工作是否原子、哪些子问题可拆分，以及上下文隔离、专业注意力、局部并行或独立验证是否有净收益。仍由当前 Agent 对结果负责的拆分使用 Subagent；需要持久 owner、跨轮交接、独立验收或可恢复拓扑的拆分使用 Work Item/WorkGraph。

## 独立判断信号

| 信号 | 合适结构 | 不代表什么 |
| --- | --- | --- |
| 局部步骤容易遗漏，进度值得在节点内展示 | Task/Todo | 不产生新 owner 或交接 |
| 独立上下文、专业注意力或局部并行有净收益 | Subagent | 不转移父 Agent 的责任 |
| 一个结果能单独描述、指派、验收或接管 | Work Item | 不要求必须由另一 Agent 承担 |
| 多个责任之间存在真实依赖、并行或恢复顺序 | Plan/WorkGraph | 不等于步骤清单越长越好 |
| 另一持久 Agent 需要跨轮拥有责任 | Room Assignment | 普通 `@` 不自动产生 Assignment |
| objective 必须跨当前执行边界持续存在 | Goal | 复杂度和参与人数不是充分证据 |

## 节点语义

### Agent

Agent 是对交付负责并与用户沟通的主要节点。Lead/创建者也必须把自己真正承担的规划、研究、整合、审核或交付体现为自己的执行节点，而不是只给别人分配工作。

### Task/Todo

Task 是当前 Agent 内部的局部清单。它应展开在对应 Agent 或 Work Item 内部，不建立独立责任、交接或验收，也不成为 shared WorkGraph 的第二真相源。

### Subagent

Subagent 帮助父 Agent 完成父 Agent 当前拥有的责任。只有上下文隔离、专业视角或局部并行的收益高于启动与合并成本时才使用；父 Agent 仍负责整合、验证和提交。

### Work Item 与 Room member

Work Item 表达“谁交付什么”。同一 Agent 可以拥有多个节点；不同 Agent 也可以协作一个上层责任，但必须明确最终 owner。需要持久身份、跨轮交接或共享验收时使用 Room Assignment；只需要讨论或一次性建议时使用普通消息。

## 并行先映射执行者

先判断是否需要独立责任与验收，再选择并行结构。没有依赖只说明工作可以同时 Ready，不会自动增加执行槽。

| 责任与执行需要 | 结构选择 | 运行语义 |
| --- | --- | --- |
| 两份产出需要独立 owner、交接或验收，并且希望同时执行 | 两个 Work Item，分别分配给不同 Room Agent | 两个持久 Agent slot 可以真实并行 |
| 同一 Agent 对一份整合交付负责，但内部子问题值得并行 | 一个 Work Item，由该 Agent 启动多个 Subagent | 子智能体是同一责任内部的并行 executor |
| 同一 Agent 获得多个并列 Work Item，没有启动 Subagent | 保留多个责任节点，但按该 Agent 的队列执行 | 这是串行责任队列，不能称为并行 |
| 没有第二个合适 Agent，Subagent 收益也不足 | 保持当前 owner 并顺序执行 | 明确降级为串行，不制造执行者 |

不要为了让图看起来并行而复制 Agent 身份。若独立交付确实需要并行但 Room 中没有第二个合适成员，由 coordinator 调整责任结构、选择一个总 Work Item 内的 Subagent 分解，或诚实地串行推进。

### Tool

Tool 是被观测的执行活动，不是模型手工维护的责任节点。底层为每次调用保留独立 Node Run；失败后成功不会覆盖或折叠第一次事实，只有存在 exact retry identity 时才增加 `retry` 边。画布按用户可观察性把 Tool 放在 nested/detail 层级，WebSearch、WebFetch、命令、写入、浏览器与外部 capability 等有助于理解动作的调用可以显示；visibility 只影响展示，不删除运行事实。

## 可以组合，但不要套模板

- 单 Agent 可以有 WorkGraph，也可以只用 Task 或 Subagent。
- Room 可以只有聊天，不必因为多人参与而创建 Plan。
- Goal 可以独立使用 Agent Loop、Task、Subagent 和 Runtime Graph，不需要 Execution/Plan。
- WorkGraph 可以不带 Goal；只有同时需要跨 boundary 持续 objective 时才绑定 exact Goal revision。
- 一个 Agent 节点内部可以同时有 Task、Tool 与多个 Subagent 分支。
- Lead 可以自己承担 Work Item 并自审，不必制造另一个 reviewer。
- 简单任务可能因外部等待而需要 Goal；复杂任务也可能在当前 round 直接完成而不需要 Goal。

## Goal 选择

只有 objective 需要跨 round、上下文切换、外部等待、中断恢复、预算边界或高恢复成本继续存在时，才考虑 Goal。若当前受管 Execution 开放 `promote_execution_to_goal`，Agent 可以基于这些事实自适应提升；不要从 Plan 长度、Room、Subagent 或“看起来重要”推断持久化。

一旦选择 Goal，加载 `goal-manager`，由它处理创建、纠正、完成和阻塞的生命周期细节。不要因为 Goal 存在就补建 WorkGraph；两种结构必须分别通过自己的选择信号。

## 用例只是校验

一次性小任务通常直接执行；单 Agent 多步骤通常先考虑 Task；局部研究并行通常考虑 Subagent；多个独立交付通常考虑 Work Item + Plan；跨 Agent 持久交接通常增加 Room Assignment；条件返工通常使用 Gate + 新 Node Run；跨边界持续通常考虑 Goal。

这些例子只用于检查语义理解，不能反向作为触发规则。让任务决定结构，让实际运行事件解释工作图。
