# 图控制

只在设计或调整 WorkGraph 拓扑、审核、Gate 或 Loop 时读取本文件。

## 两层图

- **责任图**记录 Work Item、owner、依赖、交付、审核与接管，是可恢复的持久主干。
- **运行图**记录 Agent、Subagent、Tool、Gate、Hook 等真实 Node Run，并嵌套在责任节点内部。

Task 属于 Agent 节点内部的局部步骤；Subagent 和 Tool 属于实际运行子图。不要把三者都提升成平级 Work Item，也不要用运行事件反向改写已经发生的责任历史。

## Plan Document 传输

需要建立或调整责任图时，把完整 YAML 作为单个 `plan_document` string 写入 `prepare_plan_execution` 的 command input，校验成功后只把返回的 `proposal_id` 与 `proposal_digest` 原样交给一次 `plan_execution`。`goal_binding` 是外层 command JSON 中与 `plan_document` 并列的字段，绝不是 Plan YAML root 字段。fresh Goal-free `create` 的外层输入明确使用 `goal_binding=none`；只有当前 round 已持有 exact Goal authority 且确实要绑定时才用 `goal_binding=current`；`replan`/`replace` 省略或使用 `inherit`，不能改变既有边界。如果还需新建 Goal 并把图绑定给它，先等待 `create_goal` 的 applied receipt，再准备绑定它的 Plan；不要并行执行二者，因为 Goal 身份与 objective 是 proposal 的权威 fence。只有 exact Goal-bound context 下的 fresh `create` 可以省略 root `objective`，服务端会继承 exact Goal objective；同一 session 中只是存在 ambient Goal 不等于绑定。每个 `create`/`replace` 都必须填写 `completion_criteria`，Goal-free `create`/`replace` 还必须填写 `objective`，`replan` 则继承当前 Execution 的 objective 与 completion criteria。改变已绑定 Goal 先执行 `retarget_goal`，不要在 Plan 中改写或概述成另一个权威目标。

Plan operation 只按本轮 `execution inspect` 返回的 current Execution 选择，不能从历史图或 Goal 的 predecessor 关系猜测：没有 current Execution 时使用 `create`；存在同一 objective boundary 的 current Execution、确实只增加 Plan revision 时才使用 `replan`；只有当前 transient、Goal-free Execution 需要整体替换时才考虑 `replace`。Goal reset/retarget 会把旧 Execution 变为 predecessor；如果 successor 尚未 materialize，此时虽然是在替换历史链路，successor 的第一份 Plan 仍是 `create`，并在 exact Goal authority 下使用外层 `goal_binding=current`。

Plan Document 的精确字段、枚举和条件必填项只有一个真相源：`execution contract --operation prepare_plan_execution` 返回的 input schema 与 parser-backed `document_contract`。Skill 不复制完整字段表；不要根据记忆猜别名，也不要根据单个报错逐字段删改。校验失败时读取返回的完整 contract，修正后一次重交整份 YAML。

`prepare_plan_execution` 的文档校验失败才允许在同一 physical round 修正文档。若返回 `context_status=round_refresh_required`，说明启动本轮时固定的 Goal/Execution authority 已被用户 retarget 或宿主 successor 替换；`inspect` 只能读新状态，不能给旧 round 换发 authority，因此本轮必须结束并等待宿主 continuation，不能把它当作 YAML 错误重试。

下面只是一个最小 create 示例，不是第二份 schema：

```yaml
nexus_plan: 1
operation: create
objective: "Deliver the requested outcome"
completion_criteria:
  - "The requested outcome is delivered and verified"
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
    output_scopes:
      - "semantic:requested-outcome"
```

`replan` / `replace` 的复用、依赖、输出 scope 和 active-work 条件以当轮 contract 为准。`plan_document` 必须是 input JSON 内的一个完整 string；不要发送 placeholder、fragment 或多份 YAML document，也不要自行猜测旧字段或枚举。

## 并行与依赖

- 区分三件事：无依赖表示“可同时 Ready”，不同执行上下文已经启动才表示“实际并行”，Attempt 时间重叠才是已经发生的并行事实。
- 输入稳定、输出责任不冲突且没有真实前置依赖时才并行。
- 如果 B 必须消费 A 的交付结果，就声明依赖并等待相应验收；不要为了提高并发率伪造独立分支。
- 多个 Agent 可以并行承担不同 Work Item；一个 Agent 节点内部也可以并行启动多个 Subagent。
- 希望多个独立 Work Item 真正并行时，把它们交给不同 Room Agent。若由同一 Agent 对整体交付负责，优先保留一个父 Work Item，并把局部分支交给不同 Subagent。
- 多个并列 Work Item 分配给同一个 Agent 时，它们进入该 Agent 的串行队列；除非真实 child Subagent 已启动，否则状态与回复都不得称其为并行。
- 没有合适的不同 Agent 或 Subagent 时允许串行执行；不要复制身份、伪造 Subagent 或用并列布局暗示不存在的并发。
- 输出范围重叠时先明确责任顺序：只有一方通过全 hard `depends_on` 路径等待另一方 Acceptance 时，才可把同一 exclusive scope 作为起草→定稿这类顺序移交；平行分支、兄弟节点、父子嵌套或 soft-only 关系仍需不同 scope。只有确实允许并发写入时才用 `shared_output_scopes`，不要用 semantic scope 掩盖实际会修改的文件。

## 动态扩展与 replan

执行中发现新范围时，优先保留已发生的 Node Run、提交、审核和证据，再按当前 `allowed_actions` 追加后继节点或建立新 Plan revision。不要删除或改写历史来伪装原计划一直如此。

追加与替换的具体字段、可变更边界和当前可用动作以 `nexus_execution_context` 和当轮 operation contract 为准；Skill 不复制瞬时版本或参数协议。

## Review 与 Gate

Gate 只表示会真实改变路线的检查，不代表每一步都需要用户确认。

- owner 与 reviewer 相同：自审折叠在同一 Agent 节点。
- reviewer 不同：显示独立 review Gate，并通过结构化 review handoff 交接。
- 高风险、争议大或需要独立证据时优先独立审核；否则不要机械增加 reviewer。
- `return_to_agent_id` 可以选择 Lead，也可以选择另一位适合且可用的 Room 成员；独立并行分支很多、Lead 会成为瓶颈或需要职责分离时，应把审核分散给不同 reviewer。Submission 之后等待审核是已提交责任，不是外部输入阻塞，worker 不得为催审再调用 `block_work`。
- Gate 返回结论与证据，不替 Agent 决定路由。

## Loop

Loop 是“执行 → 检查 → Agent 根据结果决定下一轮”，不是把静态 `depends_on` 写成环。

例如技术报告：Writer 生成草稿；Gate 检查来源、比较维度和读者可用性；若有缺口，Agent 选择启动新的 Writer Node Run、追加取证节点、等待输入或采用其他路线；满足目标后进入 Lead 整合。

每一轮重跑必须有独立 Node Run 身份，历史 Gate 结果保持可见。Objective Alignment 可以作为证据检查，但不会自动重试、结束或选择回边。Goal 生命周期不是使用 Loop 的前提。
