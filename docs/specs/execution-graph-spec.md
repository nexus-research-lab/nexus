# Execution Runtime Graph 与 WorkGraph 只读投影协议

## 1. 文档范围

本文只定义 Nexus 当前已经实现的两类运行图数据，以及它们如何形成前端只读工作图：

1. **Runtime Graph**：从真实 Agent round、Tool 和 Subagent lifecycle 中观测并持久化的运行事实。
2. **ExecutionGraphView**：在正式 managed WorkGraph 内，把责任图与相关 Runtime Graph 合并后的安全只读投影。

责任状态机由 [Execution Orchestration 协议](./execution-orchestration-spec.md) 定义。本文不重复 Goal、Plan、Work Item、Assignment、Attempt、Submission 与 Acceptance 的写入规则，也不为模型或前端定义第二套编排入口。

协议字段和枚举的代码真相源是：

- [`internal/protocol/execution_runtime_graph.go`](../../internal/protocol/execution_runtime_graph.go)
- [`internal/protocol/execution_view.go`](../../internal/protocol/execution_view.go)

稳定边界是：

> Runtime Graph 记录已经发生的运行；managed WorkGraph 记录已经声明并持久化的责任；ExecutionGraphView 只把两类确切事实合并为可读视图，不授权、触发或改写执行。

## 2. 三个对象不能混用

| 对象 | 成立条件 | 负责什么 | 是否是公共“工作图” |
| --- | --- | --- | --- |
| Runtime Graph | Nexus 已观测到一个真实 round 或 lifecycle | Agent、Subagent、Tool、Gate 的 NodeRun 与运行边 | 否；它是内部运行事实 |
| managed WorkGraph | durable Execution 存在 active Plan，且 Plan 至少包含一个 Work Item | 责任、依赖、分派、尝试、提交与验收 | 是 |
| ExecutionGraphView | 读取一个 managed WorkGraph，并按 exact identity 合并相关 Runtime Graph | Web/桌面端安全只读图 | 是，且只能依附于 managed WorkGraph 返回 |

因此：

- 普通聊天、Goal-only continuation 或 planless Execution 可以产生 Runtime Graph，但不会因此产生公共 WorkGraph。
- Goal 与 managed WorkGraph 可以独立存在；只有持久化的 exact binding 才表示两者已关联。
- 新的 runtime-only round 不得覆盖同一 session 最近一次 managed WorkGraph。
- `@member`、普通 Room 消息和参与人数是通信事实，不能被推断成 Work Item、dependency 或 dispatch。

## 3. 真相源与合并原则

### 3.1 责任层真相

managed WorkGraph 的责任事实来自 Execution 当前 active Plan 及其 durable 领域对象：

- Work Item 与 `dependency_ids`；
- Assignment 与 Review Dispatch；
- parent/child Work Attempt；
- Submission 与 Acceptance；
- Execution、Plan 和 Work Item 的当前状态。

`parent_work_item_id` 只表达包含关系。只有 `dependency_ids` 影响 Work Item readiness，前端不得用父子关系制造依赖边。

### 3.2 运行层真相

Runtime Graph 来自 provider-neutral runtime lifecycle 和 Nexus round boundary：

- round 开始时建立根 Agent NodeRun；
- Tool、Subagent 与语义 Gate 的开始、进度和终态更新对应 NodeRun；
- round 结束时单调收口根 Agent NodeRun；
- exact Artifact 独立持久化，再按 `agent_round_id + tool_use_id` 回挂到 Tool NodeRun。

观测写入是 fail-open 的展示记录：写图失败不得让已经执行过的真实 Tool 或 Subagent 被自动重放。

### 3.3 只按结构化身份合并

责任节点与运行节点只能通过已有结构化身份关联，例如：

- Execution ID、Plan ID、Work Item ID、Assignment ID、Attempt ID；
- root/runtime/agent round ID；
- Tool use ID、child session/task identity；
- Runtime Graph 的 subject 与 parent subject identity。

`AgentRoundID` 只表示一个物理 Agent round 的外层容器，不是 Work Item ownership identity。同一个 DM coordinator round 串行承担多个自分配 Work Item 时，每次成功 `assign_work` 建立新的内部执行段；该段必须由服务端 mutation envelope 中的 exact Execution/Assignment/Attempt refs 回查权威 snapshot 后，持久化为 `execution_id + work_item_id + assignment_id + attempt_id`，后续 Tool 才能归入对应责任节点。Room 不使用这条 DM 自分配推断：Lead 的协调 round 保持 coordinator lane，成员工作与审核只认 durable WorkBinding/ReviewBinding。

禁止按工具名、自然语言、消息位置、DOM 顺序或时间邻近猜测归属、重试和责任。

历史兼容只允许一种 fail-closed 修复：同一 Agent round 内，一个成功 `assign_work` 的 exact start/finish 生命周期区间必须唯一包含一个 durable root Attempt 的创建事实，且该 Attempt 也只能被一个这样的区间命中。它是封闭因果区间校验，不是“找最近时间”；任一侧多解时不得恢复执行段。

## 4. Runtime Graph 数据模型

### 4.1 NodeRun kind

当前只存在四种 runtime node kind：

| kind | 语义 |
| --- | --- |
| `agent` | 一个真实 Agent round |
| `subagent` | 父 Agent 启动的独立 child 上下文 |
| `tool` | 一次具有独立 lifecycle identity 的 Tool 调用 |
| `gate` | 已被 runtime 或责任层身份支撑的语义检查/审核节点 |

Task、Goal、Plan、Work Item、Message 和 Artifact 都不是 Runtime Graph node kind。Task 是 Agent/Subagent 内的局部步骤；Work Item 是责任分组；Artifact 是 NodeRun 的结构化输出引用。

### 4.2 NodeRun status

当前只存在以下状态：

- `running`
- `succeeded`
- `failed`
- `cancelled`
- `interrupted`

同一个 exact NodeRun identity 可以从 running 单调更新到终态。新的物理调用必须产生新的 NodeRun，不能覆盖旧调用的失败或成功事实。

### 4.3 Runtime edge kind

Runtime Graph 当前只持久化以下边：

| kind | 语义 |
| --- | --- |
| `invoke` | Agent/Subagent 调用了 exact Tool |
| `spawn` | Agent/Subagent 启动了 exact child Subagent |
| `guard` | 已观测到节点与语义 Gate 的关系 |
| `loop_back` | 失败或审核返回已实际回到 exact owner/control anchor |
| `retry` | 新 NodeRun 携带 exact previous run identity |

`loop_back` 只记录控制返回事实，不代表服务端自动重新执行。`retry` 也只记录 Agent 已经发起的新 Run 与旧 Run 的精确关系；没有 exact retry identity 时不得自动创建该边。

### 4.4 独立运行事实

- 每个 durable Tool NodeRun 都是独立运行事实。
- 第一次失败、第二次成功必须保留两个 NodeRun；不得折叠成一个 `×2` 节点并改写最终状态。
- 无 exact retry identity 时仍可以同时显示失败与后续成功，但二者之间没有 `retry` 边。
- 只有 progress、没有独立 start/finish identity 的 provider facet 不成为独立节点。
- 结果和错误只持久化有界、脱敏摘要，不持久化凭证或完整原始 Tool 输入输出。

### 4.5 投影上限

WorkGraph 必须先完成产品可见性投影，再对主图窗口应用上限。当前上限为：

- 256 个 `visibility != detail` 的 runtime 主图节点；
- 512 条连接主图节点的 runtime 边；
- 每个 Tool NodeRun 16 个 Artifact。

普通成功的本地读取、查找等 `detail` 历史不占主图节点或边配额，也不能触发 partial。只有本应进入主图的节点或边超出上述窗口、实际未投影完整时，read model 才通过 total 与 truncated 字段明确表示部分投影；durable 事实仍然保留，不能把真实截断结果伪装成完整图。

节点检查器使用独立的最近 256 条 `detail` 历史窗口。该窗口不属于主图完整性承诺，因此不会改变 `runtime_*_total` 或触发主图 partial；完整 durable 事实仍保留在存储层。

## 5. ExecutionGraphView 只读模型

`ExecutionGraphView` 是 `ExecutionView.graph` 中的只读字段。它包含：

- `nodes`：稳定责任节点及其 `runs` 历史；
- `edges`：当前只读方向边；
- `runtime_node_total` / `runtime_edge_total`：visibility 判定后本应进入主图的 runtime 节点/边总数；
- `runtime_nodes_truncated` / `runtime_edges_truncated`：对应主图节点/边是否因窗口上限而未展示完整。

前端不得把此对象回写为 Plan、Assignment、Attempt 或 runtime command。

### 5.1 View node kind 与身份

View 使用与 Runtime Graph 相同的四种 kind：`agent`、`subagent`、`tool`、`gate`。

- Agent 节点以 Work Item 为稳定责任身份；同一 Agent 承担不同 Work Item 时是不同节点。
- 一个 Agent round 可以作为多个串行 Assignment/Attempt 执行段的外层容器；Tool 必须优先按 exact 执行段归属，不能因头像、Agent ID 或共享 AgentRoundID 被全部折叠到最后一个 Work Item。
- 每个 durable child Attempt 都拥有独立 Subagent 节点，不能按父 Agent 或头像合并。
- Tool 节点对应独立 Tool NodeRun。
- Gate 节点必须来自 durable review binding/dispatch 或已观测 runtime Gate，不能从文本猜测。
- managed Attempt 与 Runtime NodeRun 只按 exact round/subject identity 合并到 `runs`，避免同一次物理运行重复展示。

### 5.2 visibility

当前可见层级只有：

| visibility | 用途 |
| --- | --- |
| `primary` | 主责任图节点 |
| `nested` | Agent/Subagent ownership tree 中的可见运行节点 |
| `detail` | 保留在节点检查器中的支持性运行事实 |

visibility 只影响默认展示，不改变运行、权限或责任状态。

### 5.3 View edge kind

当前 View 只允许以下 edge kind：

| kind | 来源与语义 |
| --- | --- |
| `dependency` | Plan 中显式声明的 Work Item 依赖 |
| `dispatch` | 已发生的责任分派/运行启动 |
| `coordination` | coordinator 对已声明根工作项的责任；不等于 dispatch |
| `spawn` | exact parent 到 child Subagent |
| `invoke` | exact owner 到 Tool |
| `guard` | exact owner/control node 到 Gate |
| `review` | 成功 `submit_work` control anchor 到 reviewer Gate |
| `loop_back` | Tool/Gate 失败或 changes requested 后的已发生返回 |
| `retry` | 两个 exact NodeRun 之间的明确 retry 关系 |

边只携带 source/target node identity、可选 source/target run identity 和观测时间。Message、Artifact、state mapping 或任意条件表达式不属于当前边协议。

## 6. Tool 可见性规则

所有具有独立 lifecycle identity 的 Tool Run 都作为 durable 运行事实保留，并在投影窗口内进入 read model；画布只提升有助于用户理解“智能体实际做了什么”的动作。

默认进入 ownership tree 的用户可观察动作包括：

- WebSearch、WebFetch；
- Bash、KillShell；
- Write、Edit、MultiEdit、NotebookEdit；
- 外部 MCP capability 调用；
- 浏览器导航、点击、填写、下载、截图；
- 创建、更新、提交、发送、发布、部署、生成等明确动作；
- `submit_work`，因为它是 Work → Review 的真实 control anchor。

默认保留为 `detail` 的支持性动作包括：

- 成功的本地 Read、Grep、Glob 等读取/查找；
- filesystem/workspace MCP；
- Tool discovery 与 Skill 加载；
- 已被 Work Item、Gate 或 Goal 领域节点完整表达的管理工具调用。

以下结构事实可以把 Tool 从 `detail` 提升到 `nested`：

- running、failed、cancelled 或 interrupted；
- 参与 `loop_back` 或 `retry`；
- 携带 Artifact；
- 携带 `workgraph_visibility=primary|nested`；
- 同一 direct owner 下已出现同类失败，随后成功需要被用户看见。

Subagent 子树遵守同一规则，并可额外保留少量 direct supporting Tool 作为代表；这不允许把所有普通读取铺满画布。

## 7. 公共读取边界

### 7.1 `GET /executions/latest`

公共 WorkGraph 读取只返回 managed WorkGraph：

1. 优先返回同一 owner/session 最近的未终结 managed Execution；
2. 若不存在，返回最近一次 managed Execution，供 UI 回看 terminal 结果；
3. 若该 session 从未产生 managed WorkGraph，返回 `null`。

managed 的最低条件是 Execution 拥有 active Plan，且该 Plan 至少包含一个 Work Item。planless Execution、普通 runtime-only round 和 Goal-only continuation 都不能让该接口返回非空，也不能替换已保留的 managed WorkGraph。

接口只接受已认证 owner scope 内的 `session_key`，并返回 `ExecutionView` 的安全投影。它没有写入、重试、路由或状态推进能力。

### 7.2 前端资源策略

- Header 与移动端“工作图”入口固定常驻；没有 managed WorkGraph 时打开统一明确空态。
- Composer Agent Dock 只在当前 managed WorkGraph 非终态活动时显示。
- runtime-only graph 不能填充 Surface、替换已保留的 managed 图或触发 Composer Dock。
- 资源读取失败时可以保留最后一次成功快照，但必须显式标记 stale 与最后成功时间。
- 返回 truncated flags 时 Surface 必须显式标记 partial。

## 8. 图布局与交互

### 8.1 层级布局

- 主责任图自上而下排列。
- 同一 owner 的直接子节点从左到右排列。
- 每个 Subagent 的 Tool/Subagent 后代继续向下形成独立 ownership subtree。
- 只为一级 Agent/Gate 的完整 runtime tree 绘制一个归属框；Subagent 不再创建套娃子图框。
- `parent_work_item_id` 只影响包含分组，不参与 readiness 或伪造依赖层。

### 8.2 连线

- 普通流程和回连都使用 `M/L` 正交折线，不使用密集贝塞尔曲线。
- 回连先离开节点层，再沿所属归属框内部预留的底部/侧面走廊返回 control anchor。
- 同一目标与侧面的回连可以合流到共享 U 形总线；线与线可以重叠，线不得穿过节点。
- 回连总线与圆角边框必须保留稳定内缩留白。
- 普通流程使用可读中性灰；回连使用降饱和暖色和接近的线宽/透明度。
- review 与 changes-requested 返回应锚定成功的 `submit_work` 节点，而不是笼统连到 Agent 头像。

### 8.3 只读白板

用户可以：

- 平移、缩放、双击聚焦、适配视口和定位当前节点；
- 搜索、折叠/展开 ownership subtree；
- 点击节点或边打开检查器；
- 用空白点击或 Escape 关闭检查器；
- 从 Tool Run 的安全 workspace 相对 Artifact 跳转到既有 Workspace 打开链路。

这些操作只修改当前用户的本地视图状态，不得修改权威拓扑、运行状态、责任人或路线。UI 不提供 Graph 状态写入按钮。

## 9. 安全与一致性不变量

1. Runtime Graph 可以独立持久化，但不能独立成为公共 WorkGraph。
2. 公共 ExecutionGraphView 必须依附于 active Plan + non-empty Work Items。
3. Work Item dependency DAG 与 runtime `loop_back` / `retry` 是两类不同事实。
4. 每次 Tool/Subagent 物理运行保留独立身份和历史。
5. exact retry identity 缺失时不得推断 retry edge。
6. exact parent identity 缺失时不得按 Agent、文本或时间猜测 ownership。
7. coordinator responsibility 不等于已发生 dispatch。
8. Subagent completion 不自动完成 Work Item、Acceptance 或 Goal。
9. runtime observer 不发起 Tool、Subagent、重试或责任迁移。
10. read model 不暴露 command、lease、capability identity、凭证或完整原始 Tool I/O。
11. owner、session 与 Execution identity 必须精确匹配；跨 owner/session 数据不得进入视图。
12. 截断、旧快照和读取失败必须显式呈现，不能伪装成完整实时图。

## 10. 当前非目标

- 不提供通用图写入 API。
- 不允许前端直接改边、状态、责任人或运行路线。
- 不把 Runtime Graph 当作模型必须遵循的固定脚本。
- 不从普通聊天、Goal、Task、Tool 名称或 UI 布局反推 managed WorkGraph。
- 不把失败事实解释成服务端已经自动重试。
- 不把尚未实现的类型或行为写成当前契约。
