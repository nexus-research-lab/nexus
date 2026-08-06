# 图控制

只在设计或调整 WorkGraph 拓扑、审核、Gate 或 Loop 时读取本文件。

## 两层图

- **责任图**记录 Work Item、owner、依赖、交付、审核与接管，是可恢复的持久主干。
- **运行图**记录 Agent、Subagent、Tool、Gate、Hook 等真实 Node Run，并嵌套在责任节点内部。

Task 属于 Agent 节点内部的局部步骤；Subagent 和 Tool 属于实际运行子图。不要把三者都提升成平级 Work Item，也不要用运行事件反向改写已经发生的责任历史。

## Plan Document 传输

需要建立或调整责任图时，把完整 YAML 作为单个 `plan_document` string 交给 `prepare_plan_execution`，校验成功后只把返回的 `proposal_id` 与 `proposal_digest` 原样交给一次 `plan_execution`。如果还需新建 Goal，先等待 `create_goal` 成功，再准备绑定它的 Plan；不要并行调用二者，因为 Goal 身份与 objective 是 proposal 的权威 fence。active Goal 下的 fresh `create` 只能省略 root `objective`，服务端会继承 exact Goal objective；每个 `create`/`replace` 都必须填写 `completion_criteria`，Goal-free `create`/`replace` 还必须填写 `objective`，`replan` 则继承当前 Execution 的 objective 与 completion criteria。改变 Goal 先调用 `retarget_goal`，不要在 Plan 中改写或概述成另一个权威目标。

每个 Work Item 必填且只能使用精确字段名 `logical_key`、`kind`、`subject`、`objective`、`deliverable`；可选字段为 `existing_work_item_id`、`acceptance_criteria`、`required`、`terminal`、`parent_logical_key`、`depends_on`、`soft_depends_on`、`input_refs`、`output_scopes`、`shared_output_scopes`。不要写 `dependencies`、`description`、`acceptance` 或 `scopes`：它们不是别名，会被 strict parser 拒绝。`kind` 只能是 `produce`、`review`、`verify` 或 `integrate`；依赖字段是 logical key 的 string sequence；输出范围必须使用 `file:<path>`、`dir:<path>` 或 `semantic:<key>`。

先按下面的完整形状一次写完，再提交；不要根据单个报错逐字段删改：

```yaml
nexus_plan: 1
operation: create
objective: "Deliver the requested outcome"
completion_criteria:
  - "The requested outcome is delivered and verified"
revision_reason: ""
supersede_active_work: false
replacement_reason: ""
items:
  - logical_key: produce
    kind: produce
    subject: "Produce the requested outcome"
    objective: "Create the requested deliverable"
    deliverable: "The completed requested outcome"
    acceptance_criteria:
      - "The deliverable satisfies the requested scope"
    required: true
    terminal: true
    parent_logical_key: ""
    depends_on: []
    soft_depends_on: []
    input_refs: []
    output_scopes:
      - "semantic:requested-outcome"
    shared_output_scopes: []
```

`existing_work_item_id` 只在 replan 复用既有 Work Item identity 时填写。不要把对象或数组直接作为工具参数，不要发送 placeholder、fragment 或多份 YAML document，也不要自行猜测旧字段或枚举。

## 并行与依赖

- 区分三件事：无依赖表示“可同时 Ready”，不同执行上下文已经启动才表示“实际并行”，Attempt 时间重叠才是已经发生的并行事实。
- 输入稳定、输出责任不冲突且没有真实前置依赖时才并行。
- 如果 B 必须消费 A 的交付结果，就声明依赖并等待相应验收；不要为了提高并发率伪造独立分支。
- 多个 Agent 可以并行承担不同 Work Item；一个 Agent 节点内部也可以并行启动多个 Subagent。
- 希望多个独立 Work Item 真正并行时，把它们交给不同 Room Agent。若由同一 Agent 对整体交付负责，优先保留一个父 Work Item，并把局部分支交给不同 Subagent。
- 多个并列 Work Item 分配给同一个 Agent 时，它们进入该 Agent 的串行队列；除非真实 child Subagent 已启动，否则状态与回复都不得称其为并行。
- 没有合适的不同 Agent 或 Subagent 时允许串行执行；不要复制身份、伪造 Subagent 或用并列布局暗示不存在的并发。
- 输出范围重叠时先明确唯一 owner 或共享规则，避免重复生产。

## 动态扩展与 replan

执行中发现新范围时，优先保留已发生的 Node Run、提交、审核和证据，再按当前 `allowed_actions` 追加后继节点或建立新 Plan revision。不要删除或改写历史来伪装原计划一直如此。

追加与替换的具体字段、可变更边界和当前可用动作以 `nexus_execution_context` 和工具 schema 为准；Skill 不复制瞬时版本或参数协议。

## Review 与 Gate

Gate 只表示会真实改变路线的检查，不代表每一步都需要用户确认。

- owner 与 reviewer 相同：自审折叠在同一 Agent 节点。
- reviewer 不同：显示独立 review Gate，并通过结构化 review handoff 交接。
- 高风险、争议大或需要独立证据时优先独立审核；否则不要机械增加 reviewer。
- Gate 返回结论与证据，不替 Agent 决定路由。

## Loop

Loop 是“执行 → 检查 → Agent 根据结果决定下一轮”，不是把静态 `depends_on` 写成环。

例如技术报告：Writer 生成草稿；Gate 检查来源、比较维度和读者可用性；若有缺口，Agent 选择启动新的 Writer Node Run、追加取证节点、等待输入或采用其他路线；满足目标后进入 Lead 整合。

每一轮重跑必须有独立 Node Run 身份，历史 Gate 结果保持可见。Objective Alignment 可以作为证据检查，但不会自动重试、结束或选择回边。Goal 生命周期不是使用 Loop 的前提。
