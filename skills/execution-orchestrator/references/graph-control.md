# 图控制

只在设计或调整 WorkGraph 拓扑、审核、Gate 或 Loop 时读取本文件。

## 两层图

- **责任图**记录 Work Item、owner、依赖、交付、审核与接管，是可恢复的持久主干。
- **运行图**记录 Agent、Subagent、Tool、Gate、Hook 等真实 Node Run，并嵌套在责任节点内部。

Task 属于 Agent 节点内部的局部步骤；Subagent 和 Tool 属于实际运行子图。不要把三者都提升成平级 Work Item，也不要用运行事件反向改写已经发生的责任历史。

## 并行与依赖

- 输入稳定、输出责任不冲突且没有真实前置依赖时才并行。
- 如果 B 必须消费 A 的交付结果，就声明依赖并等待相应验收；不要为了提高并发率伪造独立分支。
- 多个 Agent 可以并行承担不同 Work Item；一个 Agent 节点内部也可以并行启动多个 Subagent。
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
