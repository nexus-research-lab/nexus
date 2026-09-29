# Nexus WorkGraph：代码解读与保护边界

> 本报告只解释当前仓库已经实现的 WorkGraph 行为。每个结论都对应具体的包、类型、方法、迁移或测试。设计设想和未验证的外部能力会单独标出。
>
> 配套实现文档：[WorkGraph 实现方案与使用说明](./workgraph-implementation-design.zh-CN.md)。
>
> 如果需要从“为什么要把 Task、Subagent、Goal 放进同一套长程机制”的角度阅读，请看[WorkGraph 设计理念与问题定义](./workgraph-design-principles.zh-CN.md)。

## 1. 阅读结论

代码里的 WorkGraph 由四类持久化对象组成，各自负责不同的事实：

1. **Execution Orchestration**：保存一次受管执行的责任事实。
2. **Runtime Graph**：保存 Agent、Subagent、Tool、Gate 的运行观察事实。
3. **Workflow**：保存可跨主题复用的命名工作图模板。
4. **Draft**：保存从完成 Execution 提炼出来、可继续编辑、可版本选择的草图。

Goal 保存长期目标。它可以和 Execution 建立双向确认的绑定，但不代替 WorkGraph 的 Work Item、Assignment、Attempt、Submission 和 Acceptance。

前端的 `ExecutionGraphView` 只是读取投影，不能直接修改责任图。Agent 通过每个 physical round 注入可信身份的 `nexus.command` 调用编排操作；owner、权限和 SQL ID 不能由普通文本提交。

## 2. 代码地图

| 层 | 代码入口 | 主要责任 |
| --- | --- | --- |
| 协议类型 | `internal/protocol/execution.go`、`execution_view.go`、`workgraph_workflow.go` | Execution、Plan、Work Item、运行投影、Workflow/Draft 的共享类型 |
| 责任编排 | `internal/service/orchestration/` | Plan proposal/materialization、分配、执行尝试、提交、评审、完成审计 |
| 运行图 | `internal/service/orchestration/runtime_graph.go` 与 `internal/storage/orchestration/runtime_graph.go` | NodeRun/EdgeRun 的生命周期观察 |
| Goal | `internal/service/goal/`、`internal/service/goalexecution/` | objective revision、continuation、用量、alignment、Goal/Execution binding |
| Workflow/Draft | `internal/service/workgraphworkflow/` | 完成图提炼、草图版本、命名图保存、Slash 展开 |
| MCP 控制面 | `internal/mcp/command/execution/operation/` | Agent 可调用的 execution 与 WorkGraph authoring operation |
| round 装配 | `internal/app/runtime/command.go` | 为每个 round 固定 owner、Agent、Session、authority 和 binding |
| HTTP 边界 | `internal/handler/execution/` | 只读 Execution、Draft 编辑、确认保存和目录接口 |
| 前端投影 | `web/src/features/conversation/shared/execution/` | 画布、检查器、运行历史、Draft/命名图 UI |
| 数据演进 | `db/migrations/sqlite/`、`db/migrations/postgres/` | SQLite/PostgreSQL 双方言表结构 |

建议从 `internal/protocol/execution.go` 开始，再读 `internal/service/orchestration/`，最后读 `workgraphworkflow`。这样不会把前端显示模型误认为后端权威。

## 3. 核心数据模型：从类型而不是名词理解

### 3.1 Execution 是一次受管执行容器

`protocol.Execution` 保存：

- owner、session、DM/Room scope；
- coordinator；
- objective 和 completion criteria；
- Goal ID、Goal objective revision、activation origin/reason；
- recovery/replacement 关系；
- root round、trigger message；
- status、version 和时间戳。

它没有复制 active Plan 的完整内容；active Plan 由 `ExecutionPlanRevision.Status` 表达。这样可以避免 Execution 与 Plan 各自维护一份“当前图”。

创建与状态变更位于 `internal/service/orchestration`，数据库主表为 `executions`，迁移起点是 `00061_execution_orchestration.sql`。

### 3.2 Plan revision 是不可变图版本

`ExecutionPlanRevision` 的状态只有 `proposed`、`active`、`superseded`、`cancelled`。Plan 内容创建后不更新；replan 生成新 revision，旧 revision 保留。

Plan 中的结构由以下类型组合：

- `ExecutionPlanItem`：把 Work Item 的某个 immutable spec 纳入该 revision，并记录 parent、required、terminal、position；
- `ExecutionPlanDependency`：同一 Plan 内的 hard/soft 依赖；
- `ExecutionPlanOutputClaim`：文件、目录或语义产出范围及 exclusive/shared 冲突模式；
- `WorkItemSpec`：subject、objective、deliverable、acceptance criteria、input refs 和 spec hash。

Plan 的模型输入先经过 `prepare_plan_execution`，成为非权威 proposal；只有 `plan_execution` 才 materialize Execution、Plan 和 Work Item。这一边界由 `plan_proposal.go`、`plan_materialization.go` 和 execution operation 实现。

### 3.3 Work Item 是稳定责任身份

`WorkItem` 只保存 stable ID、Execution ID、logical key 和 kind。kind 有 `produce`、`review`、`verify`、`integrate`。

可变生命周期由 `WorkItemState` 保存，只有 `open`、`waiting_input`、`cancelled`、`superseded`。`ready`、`assigned`、`running`、`submitted`、`accepted` 不被复制成第二套状态，而是根据当前 Plan、Assignment、Attempt、Submission 和 Acceptance 推导。

这解释了一个关键行为：

> “Agent 说完成”或 Attempt 成功，都不能直接使下游 ready；hard dependency 要等上游 Submission 被 Acceptance 标记为 accepted。

### 3.4 Assignment、Attempt 和 binding 分工

Assignment 保存当前/历史责任归属、分配策略、派发、接管、释放和完成原因。当前 Work Item 只能有一个 current Assignment。

Attempt 保存一次真实执行，包含 root/child 关系、runtime session、runtime round、Agent round、SDK task 等身份。失败或返工产生新 Attempt，旧 Attempt 仍是历史事实。

WorkBinding 是宿主签发的运行权限，把 exact `Execution → Plan → Work Item → Assignment → Attempt → Dispatch` 绑定在一起。Room membership、头像、`<assigned_work>` 文本和 coordinator 身份都不能替代它。Reviewer 使用独立的 ReviewBinding，不能因审阅 Submission 获得 worker 权限。

## 4. 责任闭环的代码路径

### 4.1 读取和准备

Agent 先调用 `get_execution` 读取当前可信执行上下文。读取结果包含当前 lane、allowed actions、责任绑定、blocker 和有限的下一步上下文；它不是模型自行维护的状态缓存。

然后调用 `prepare_plan_execution`。后端会解析完整 Plan Document，校验字段、依赖、输出 claim 和 Goal binding，并封存 proposal。proposal 尚未创建真实 Work Item，也不能启动 Agent。

### 4.2 Materialize

`plan_execution` 使用宿主持有的 proposal identity，事务性创建或替换 Execution/Plan/Work Item 责任结构。模型不能通过普通 input 选择别人的 proposal，也不能直接提交 owner、session 或 Agent identity。

### 4.3 分配和执行

`assign_work` 校验 hard dependency、输出 claim、当前 Plan、owner 和 coordinator authority，随后创建 Assignment、root Attempt，并在需要时建立 Dispatch outbox。

Worker round 中的 `submit_work` 再次校验：

- Assignment 仍 active 且 owner 匹配；
- Attempt 属于同一 Execution/Plan/Work Item/Assignment 链；
- Attempt 已 succeeded；
- 当前 Work Item 没有未审 Submission；
- request、assignment 和 execution version 没有过期。

### 4.4 评审和返工

Submission 是 append-only 交付声明。Review Dispatch 将它交给 Reviewer，ReviewBinding 锁定 immutable Submission。`review_work` 写入 Acceptance：`accepted`、`rejected` 或 `changes_requested`。

accepted 才能解除 hard dependency，并触发 completion audit；rejected/changes_requested 保留旧 Submission 和 Gate，返工用新的 Attempt 和新的 Submission。

### 4.5 完成审计

最后一个 required/terminal Work Item 被 accepted 后，系统先写 completion audit receipt，再重新读取当前 Plan、blocker、Assignment 和 Goal binding，并用版本/CAS 规则尝试完成 Execution。UI 看到“节点都完成”不是完成条件。

如果存在 confirmed Goal binding，Goal 侧还要做 objective alignment 和 Goal 收口；没有 confirmed binding 的 WorkGraph 不会因为同一个 Session 恰好存在 Goal 就自动绑定。

### 4.6 取消、替换、接管和过期

停止责任和停止物理运行分成两个阶段。

- **控制面收口**：`abandon_execution`、Goal retarget、Execution replacement 和 `take_over_work` 推进 successor/superseded，释放 Assignment 或封住旧 revision。
- **物理中断**：`internal/service/orchestration/cancellation_dispatch.go` 把 exact Attempt/runtime round 的取消写入 outbox，由 `background_coordinator.go` 领取和投递。
- **中断结果**：`internal/runtime/interrupt.go` 区分 provider interrupt、local cancellation、no-op 和 unsupported。无法证明旧 round 已停止时，authority fence 仍拒绝迟到输出；旧 Attempt、取消回执和迟到 terminal 会保留。

Plan replacement 需要单独的 `operation: replace`、完整 successor boundary 和 `replacement_reason`。`plan_document.go`、`plan_proposal.go` 和 `execution_transition.go` 禁止跨 Execution 搬运 Work Item、Acceptance 或旧授权。相同目标的结构变化走 replan；停止而没有 successor 走 abandon。

## 5. Goal、Plan 与 WorkGraph 的真实复用边界

### 5.1 三种运行模式

代码语义允许：

| 模式 | 真实持久对象 | 适用含义 |
| --- | --- | --- |
| Goal-only | Goal + continuation | 长期目标、跨轮推进、阻塞和完成，不要求责任图 |
| WorkGraph-only | Execution + active Plan + Work Items | 一次受管编排，不自动读取 ambient Goal |
| Goal + WorkGraph | Goal 与 Execution 的 confirmed bilateral binding | Goal 的目标修订、完成标准和 continuation 约束该图 |

binding 解析状态包括 `standalone`、`reserved`、`pending`、`confirmed`、`conflict`。只有 `confirmed` 才能启用双方完成审计和 continuation 联动；pending/conflict 必须 fail closed。

### 5.2 Goal 复用的代码能力

WorkGraph 复用了 Goal 的：

- owner/session/room 身份边界；
- objective revision 和 completion criteria；
- continuation 的 durable receipt、lease、claim、settle/recovery；
- `goal/runtimeusage` 的用量转换和最终结算；
- completion 前的 alignment 审计。

WorkGraph 没有复用 Goal 的：

- Work Item、Assignment、Attempt、Submission、ReviewBinding；
- 依赖解锁和 Acceptance Gate；
- 旧 Execution 的责任链搬迁；
- 从 transcript 或旧 Plan 重新猜 objective authority。

### 5.3 Plan 的三个层次

需要区分三个经常被混用的词：

1. **Plan Document**：模型提交的结构化输入。
2. **ExecutionPlanRevision**：服务端封存的不可变权威版本。
3. **ExecutionGraphView**：前端读取的责任图 + Runtime Graph 投影。

所以“Plan”不是只存在 prompt 里的文字，也不是前端画布本身。

## 6. Agent 如何调用 WorkGraph

### 6.1 round-scoped `nexus.command`

`internal/app/runtime/command.go` 在每个 physical round 创建 MCP server，并把 owner、Agent、Session、round、Goal/Execution authority、WorkBinding、ReviewBinding 和 Plan Mode 固定进 server context。

`internal/mcp/command/execution/operation/registry.go` 注册普通 execution surface：

- `get_execution`
- `prepare_plan_execution`
- `plan_execution`
- `abandon_execution`
- `assign_work`
- `submit_work`
- `review_work`
- `block_work`
- `resume_work`
- `take_over_work`
- `audit_execution_alignment`
- `promote_execution_to_goal`

命名 WorkGraph authoring 由 `workflow.go` 另行追加：

- `inspect_workgraph_library`
- `extract_workgraph_preview`
- `get_workgraph_preview`
- `revise_workgraph_preview`
- `select_workgraph_preview_revision`
- `save_workgraph_preview`

这些操作共用命令协议，但 service 和 authority 不混在一个无边界的 JSON 里。

### 6.2 closed input 和可信字段

operation adapter 使用 closed input 解码和 schema bounds 校验；模型不提供 owner、Agent、Session、round、Execution、Assignment 或 Attempt 作为授权来源。稳定 command ID 从当前 round/tool-use identity 派生，未知结果要先 inspect/对账，不能换一个 request 重放副作用。

### 6.3 不同 Agent lane 的能力

| lane | 能做什么 |
| --- | --- |
| coordinator | inspect、prepare/materialize、assign、takeover、协调和必要的 review dispatch |
| worker | 在 exact WorkBinding 下执行、启动受管 child、submit、block/resume |
| reviewer | 在 exact ReviewBinding 下评审 immutable Submission |
| observation member | 读取有限共享图，不能写责任状态 |
| runtime-only subagent | 协助当前运行，但不能满足交付 Gate |
| hidden editor Agent | 只编辑/选择 Draft，不执行草图业务任务 |

### 6.4 普通工具与控制命令的分层

Bash、Read、Grep、浏览器等是执行工具；WorkGraph operation 是控制面。运行图可观察二者，但控制调用通常保留为 owner detail，只有失败、取消、中断、Artifact 或明确用户可见动作才提升为主图节点。

## 7. 从完成图提炼可复用 WorkGraph

### 7.1 入口条件

`Service.PreviewFromExecution` 在 `internal/service/workgraphworkflow/service.go` 中执行以下检查：

- owner、source session、source execution 必须 exact；
- source Execution 必须 `completed`；
- Plan 和 Work Items 必须存在；
- 同一 source 已有 Draft 时直接复用；
- owner 命名图数量不能超过上限。

### 7.2 模型输入和禁止输入

`abstraction.go` 的 `AbstractionInput` 只提供 objective、completion criteria 和完整责任节点。节点包含 logical key、kind、subject/objective/deliverable、acceptance criteria、required/terminal、父节点、依赖、delegated、独立复核、Attempt count 等粗粒度信号。

它明确不包含 Tool 输入/输出、凭证、完整结果正文、Artifact 内容、Assignment identity、Attempt identity、Submission、Review 或 Acceptance 事实。这样模型只能抽象“职责结构”，不能复制旧运行。

### 7.3 模型抽象，宿主校验

`LLMAbstractor` 使用 owner 默认 Provider/Model，温度为 0，最多 16,384 tokens，超时 3 分钟，并要求返回单个 JSON 对象。

`applyAbstraction` 及后续校验保证：

- logical key 必须来自源节点的非空子集；
- 不得新增、合并、拆分或虚构节点；
- 必须保留 must-preserve 节点；
- 至少保留一条 key 主路径和一个 terminal 节点；
- 节点字段、角色和验收条件完整；
- Slash 名称匹配 `^[a-z][a-z0-9-]{0,63}$`，且不覆盖保留命令或 owner 已有名称。

模型的职责是去除一次性主题和路径；结构正确性由宿主负责，失败则不保存。

## 8. Draft、命名图和编辑 Session

### 8.1 Draft 是可恢复编辑聚合

Draft 保存 source owner/session/execution、`head_revision`、`selected_revision`、editor identity、save state、lease、origin workflow 等。每次完整编辑追加不可变 version；选择旧版本只改 selected，不删除更高版本；后续修改用 head 做 CAS。

数据库迁移：

- `00119_workgraph_workflow_drafts.sql`：Draft 和版本表；
- `00134_workgraph_draft_saved_origins.sql`：保存来源与 origin 隔离。

`workgraphworkflow.Service` 同时有进程内 cache，但配置 `DraftRepository` 后，数据库 Draft 是重启、跨窗口和恢复的真相源。

### 8.2 命名 Workflow 保存什么

`workgraph_workflows` 保存 owner、Slash、标题/描述、来源 provenance、objective、completion criteria 和 version；节点表保存抽象后的 logical key、role、kind、语义字段、required/terminal、parent/position；依赖表保存 hard/soft DAG。

命名图不保存 Agent、Assignment、Attempt、Submission、Review、Acceptance、Runtime Graph、Tool input/output 或旧状态。

### 8.3 隐藏编辑 DM

`internal/app/workgraph/adapter.go` 创建用途为 `workgraph_editor` 的隐藏 DM Session。它由 owner 的 Nexus 主 Agent 承载，不继承来源 DM/Room 的 transcript、Connector、workspace 或权限。

编辑器只暴露受限 authoring operation。关闭 UI 不等于删除 Session；只有显式 close 才删除。保存动作直接把 selected Draft 交给事务，不再启动隐藏模型 round。

## 9. 数据库存储分层

### 9.1 Orchestration aggregate

`00061_execution_orchestration.sql` 及其后续迁移构成核心责任事实：

- `executions`
- `execution_plan_revisions`
- `execution_work_items`
- `execution_work_item_specs`
- `execution_plan_items`
- `execution_plan_dependencies`
- `execution_plan_output_claims`
- `execution_work_item_states`
- `execution_work_assignments`
- `execution_dispatches`
- `execution_attempts`
- `execution_submissions`
- `execution_review_dispatches`
- `execution_acceptances`
- `execution_events`

repository 在 mutation 中重新读取 aggregate，执行 owner、binding、version/CAS 和 terminal fence，再在 SQL transaction 中提交状态及事件。

### 9.2 Runtime Graph

`00064_runtime_execution_graph.sql` 建立：

- `runtime_graph_node_runs`：Agent、Subagent、Tool、Gate；
- `runtime_graph_edge_runs`：invoke、spawn、guard、loop_back 等运行边。

Artifact 通过 exact `agent_round_id + tool_use_id` 回挂到 Tool NodeRun。Runtime Graph 是观察事实，不能单独授权或触发重试。

### 9.3 Workflow/Draft aggregate

`00116_workgraph_workflows.sql` 是命名图，`00119` 和 `00134` 是 Draft/保存来源。命名图与 Draft 不承担一次执行的状态机。

### 9.4 消息快照和前端资源

完成 authoring 后，`internal/message/workgraph_artifact.go` 将当时的完整草图/命名图快照投影成 `workgraph_artifact` assistant content block。消息快照不会随 Draft head 漂移；它也不是责任真相。

前端 API 类型和读取逻辑位于 `web/src/lib/api/conversation/execution-api.ts`、`web/src/types/conversation/execution.ts` 和 `workgraph-workflow.ts`。前端只读取和确认保存，不能直接编辑 Work Item 或依赖边。

## 10. 关键保护性不变量

1. proposal prepare 不产生真实责任图，materialize 才产生权威对象。
2. 一个 Execution 最多一个 active Plan revision。
3. hard dependency 只由 accepted Acceptance 解锁。
4. 一个 Work Item 只有一个 current Assignment，历史 Assignment 不覆盖。
5. Attempt、Submission、Acceptance、Plan revision 和事件保留历史。
6. Goal/WorkGraph 只有 exact confirmed bilateral binding 才联动。
7. Agent identity 和 scope 来自宿主 context，不从标题、消息、头像或自然语言推断。
8. Runtime-only Subagent、普通 `@member` 和 UI optimistic 状态不能形成 WorkGraph authority。
9. Draft 编辑用完整草图 + revision/CAS，不能靠客户端 diff 或旧缓存覆盖。
10. 命名图复用创建 fresh Execution/Plan/Work Item/Assignment/Attempt/Submission/Acceptance identity。
11. Graph view、message artifact 和 runtime context 都是投影，不能反向写责任事实。
12. 未知副作用结果先对账，不能凭超时或断线猜成功/失败并自动重放。
13. 取消控制面收口与 Provider/local interrupt 物理结果分开；迟到 terminal 不能越过 superseded/expired fence。
