# Nexus WorkGraph：代码解读与保护边界

> 本报告把已实现的 WorkGraph 行为对应到包、类型、方法和迁移；以代码与测试为准。
>
> 概念、执行流程和 Agent 用法见[《实现方案与使用说明》](./workgraph-implementation-design.zh-CN.md)；设计动机见[《设计理念与问题定义》](./workgraph-design-principles.zh-CN.md)；合同见 [execution-orchestration-spec.md](../specs/execution-orchestration-spec.md) 与 [execution-graph-spec.md](../specs/execution-graph-spec.md)。

## 1. 代码地图

持久化对象分四类：**Execution Orchestration**（责任事实）、**Runtime Graph**（运行观察事实）、**Workflow**（命名工作图模板）、**Draft**（从完成 Execution 提炼、可编辑可选版本的草图）。Goal 可与 Execution 双向确认绑定，但不代替责任对象。

| 层 | 代码入口 | 主要责任 |
| --- | --- | --- |
| 协议类型 | [execution.go](../../internal/protocol/execution.go)、[execution_view.go](../../internal/protocol/execution_view.go)、[workgraph_workflow.go](../../internal/protocol/workgraph_workflow.go) | Execution、Plan、Work Item、运行投影、Workflow/Draft 共享类型 |
| 责任编排 | [internal/service/orchestration/](../../internal/service/orchestration) | Plan proposal/materialization、分配、尝试、提交、评审、完成审计 |
| 运行图 | [service/.../runtime_graph.go](../../internal/service/orchestration/runtime_graph.go)、[storage/.../runtime_graph.go](../../internal/storage/orchestration/runtime_graph.go) | NodeRun/EdgeRun 生命周期观察 |
| Goal | [internal/service/goal/](../../internal/service/goal)、[internal/service/goalexecution/](../../internal/service/goalexecution) | objective revision、continuation、用量、alignment、Goal/Execution binding |
| Workflow/Draft | [internal/service/workgraphworkflow/](../../internal/service/workgraphworkflow)、[internal/storage/workgraphworkflow/](../../internal/storage/workgraphworkflow) | 完成图提炼、草图版本、命名图保存、Slash 展开 |
| MCP 控制面 | [internal/mcp/command/execution/operation/](../../internal/mcp/command/execution/operation) | execution 与 WorkGraph authoring operation |
| round 装配 | [internal/app/runtime/command.go](../../internal/app/runtime/command.go) | 为每个 round 固定 owner、Agent、Session、authority 和 binding |
| HTTP | [internal/app/server/routes.go](../../internal/app/server/routes.go)、[internal/handler/execution/](../../internal/handler/execution) | 只读 Execution、Draft 编辑、确认保存、目录接口；owner 取自认证 context |
| 前端 | [web/src/features/conversation/shared/execution/](../../web/src/features/conversation/shared/execution)、[execution-api.ts](../../web/src/lib/api/conversation/execution-api.ts)、[execution.ts](../../web/src/types/conversation/execution.ts)、[workgraph-workflow.ts](../../web/src/types/conversation/workgraph-workflow.ts) | 画布、检查器、运行历史、Draft/命名图 UI；只读取和确认保存，不直接编辑 Work Item 或依赖边 |
| 迁移 | [db/migrations/sqlite/](../../db/migrations/sqlite)、[db/migrations/postgres/](../../db/migrations/postgres) | 双方言表结构 |

建议阅读顺序：`internal/protocol/execution.go` → `internal/service/orchestration/` → `workgraphworkflow`，避免把前端显示模型误认为后端权威。

类型层要点（字段语义见 orchestration spec §3）：

- `protocol.Execution` 保存 owner、session、DM/Room scope、coordinator、objective、completion criteria、Goal ID 与 objective revision、activation origin/reason、recovery/replacement 关系、root round、trigger message、status、version 和时间戳。
- `protocol.Execution` 不复制 active Plan 内容；当前 Plan 由 `ExecutionPlanRevision.Status`（`proposed`/`active`/`superseded`/`cancelled`）表达，避免两份"当前图"。
- Plan 结构由 `ExecutionPlanItem`（成员、parent、required、terminal、position）、`ExecutionPlanDependency`、`ExecutionPlanOutputClaim` 和 `WorkItemSpec`（含 input refs 与 spec hash）组合。
- `WorkItem` 只存 stable ID、Execution ID、logical key 和 kind；可变生命周期在 `WorkItemState`。
- Plan Document 的解析与 replace 约束在 [plan_document.go](../../internal/service/orchestration/plan_document.go)、[plan_proposal.go](../../internal/service/orchestration/plan_proposal.go)、[execution_transition.go](../../internal/service/orchestration/execution_transition.go)。

## 2. `nexus.command` 装配与 operation adapter

- **装配**：[command.go](../../internal/app/runtime/command.go) 的 `NewServerBuilder` 在每个 physical round 构建 MCP server，Goal、Execution、Automation、Subagent 和 WorkGraph authoring 共用。命令面固定，可信字段每轮注入 context。
- **可信字段投影**：[contract/contract.go](../../internal/mcp/command/execution/contract/contract.go) 的 `Context` 和 `Actor()` 把 owner、Agent、Session、round、Execution/Assignment/Attempt identity 投影到 `service/orchestration`；模型输入不能提供这些授权来源。
- **adapter**（[operation/adapter.go](../../internal/mcp/command/execution/operation/adapter.go)）：
  1. 用 `DisallowUnknownFields` 解码 closed input；
  2. 校验类型、枚举、长度和 collection bounds；
  3. 从 SDK tool-use identity 或 canonical round identity 派生 stable command ID；
  4. 读取最新 Snapshot，不依赖模型上次看到的版本；
  5. 调用 `service/orchestration` 的 typed mutation；
  6. 返回 `applied`、`no_op`、`rejected`、`superseded`、`refresh_required` 等结构化结果。
- 模型没有任意 SQL ID 选择器，也没有临时 `input.json`、shell shim、loopback broker 或额外 capability token；input 直接经 SDK stream-json MCP call 进入当前 round 的 server。
- **operation 注册**：[registry.go](../../internal/mcp/command/execution/operation/registry.go) 注册 orchestration spec §6 的 12 个 execution operation。[workflow.go](../../internal/mcp/command/execution/operation/workflow.go) 另行追加 authoring operation：`inspect_workgraph_library`、`extract_workgraph_preview`、`get_workgraph_preview`、`revise_workgraph_preview`、`select_workgraph_preview_revision`、`save_workgraph_preview`。二者共用命令协议，但 service 与 authority 分开。

## 3. 取消、替换、接管

停止责任与停止物理运行分两段：

- **控制面收口**：`abandon_execution`、Goal retarget、Execution replacement 和 `take_over_work` 推进 successor/superseded，释放 Assignment 或封住旧 revision。
- **物理中断**：[cancellation_dispatch.go](../../internal/service/orchestration/cancellation_dispatch.go) 把 exact Attempt/runtime round 的取消写入 outbox，由 [background_coordinator.go](../../internal/service/orchestration/background_coordinator.go) 领取和投递。
- **中断结果**：[internal/runtime/interrupt.go](../../internal/runtime/interrupt.go) 区分 provider interrupt、local cancellation、no-op 和 unsupported。无法证明旧 round 已停止时，authority fence 仍拒绝迟到输出；旧 Attempt、取消回执和迟到 terminal 保留。

Plan replacement 需要单独的 `operation: replace`、完整 successor boundary 和 `replacement_reason`；`plan_document.go`、`plan_proposal.go`、`execution_transition.go` 禁止跨 Execution 搬运 Work Item、Acceptance 或旧授权。同目标的结构变化走 replan；停止且无 successor 走 abandon。

## 4. 完成图提炼

### 4.1 入口条件

[service.go](../../internal/service/workgraphworkflow/service.go) 的 `Service.PreviewFromExecution`：

- owner、source session、source execution 必须 exact；
- source `Execution.Status == completed`；
- active Plan 存在且含 Work Items；
- 同一 source 已有 Draft 时直接复用，不重复调用模型；
- owner 命名图数量不能超过上限。

### 4.2 模型输入

`buildSourceWorkflowGraph` 从 `ExecutionView.WorkItems` 构造 [abstraction.go](../../internal/service/workgraphworkflow/abstraction.go) 的 `AbstractionInput`：objective、completion criteria，以及每个节点的 logical key、kind、subject/objective/deliverable、acceptance criteria、required/terminal、parent 与 dependency logical keys、是否 delegated 到 Room member、是否有独立 review、Attempt count 和粗粒度状态信号。

不包含：Tool 输入/输出、Agent 凭证、完整结果正文、Artifact 内容、Assignment/Attempt identity、Submission、Review 或 Acceptance 事实。

### 4.3 模型抽象，宿主校验

`NewLLMAbstractor` 用 owner 默认 Provider/Model，温度 0，最多 16,384 tokens，超时 3 分钟，要求返回单个 JSON 对象。

`applyAbstraction` 及后续校验强制：

- 输出 logical key 是源节点的非空子集；
- 不新增、合并、拆分或虚构节点；
- 保留 `must_preserve` 节点；
- 至少一条 key 主路径、至少一个 terminal 最终交付；
- 节点字段、角色和验收条件完整；
- slash name 匹配 `^[a-z][a-z0-9-]{0,63}$`，不与保留命令或 owner 已有名称冲突。

任一失败则不保存。

## 5. Draft、命名图与编辑 Session

- **Draft 字段**：`preview_id`、`source_execution_id`、`source_session_key`、`head_revision`（并发写 CAS 基线）、`selected_revision`、`saved_workflow_id`、`saved_revision`、`editor_session_key`、`expires_at`，以及 save state、lease、`origin_workflow_id`。
- **持久化与缓存**：`workgraphworkflow.Service` 维护 `previews`、`editors`、`editorBySession` 等进程内 cache，用于快速编辑状态和过期清理；配置 `DraftRepository` 后，数据库 Draft 才是重启、跨窗口和恢复的真相源。`GetDraftByEditorID`、`RenewDraftLease`、`AppendDraftVersion` 从数据库重载并执行 CAS（[storage/workgraphworkflow/draft_repository.go](../../internal/storage/workgraphworkflow/draft_repository.go)）。
- **保存**：`SavePreview`（service.go）与 `ConfirmSave`（[save_confirmation.go](../../internal/service/workgraphworkflow/save_confirmation.go)）是 UI 与对话两条保存入口共用的事务边界。
- **隐藏编辑 DM**：[internal/app/workgraph/adapter.go](../../internal/app/workgraph/adapter.go) 的 `CreateWorkGraphEditorSession` 创建用途为 `workgraph_editor` 的 Session；工具面边界由 [metadata_editor.go](../../internal/service/workgraphworkflow/metadata_editor.go)、[session_option.go](../../internal/protocol/session_option.go) 和 `BuildWorkGraphEditor`（workflow.go）实现。
- **内置模板**：[builtins.go](../../internal/service/workgraphworkflow/builtins.go) 只读加载，不写 owner 数据库。

## 6. 存储分层

### 6.1 Orchestration aggregate

SQLite/PostgreSQL 同步迁移，起点 `00061_execution_orchestration.sql`：

| 表 | 负责的事实 |
| --- | --- |
| `executions` | owner、session、scope、objective、Goal binding、状态、版本、root round |
| `execution_plan_revisions` | immutable Plan revision 及 active/superseded/cancelled 状态 |
| `execution_work_items` | stable Work Item ID、logical key、kind |
| `execution_work_item_specs` | 每个 Work Item 的 immutable spec 版本 |
| `execution_plan_items` | revision 成员、parent、required、terminal、position |
| `execution_plan_dependencies` | hard/soft DAG 依赖 |
| `execution_plan_output_claims` | file/dir/semantic output scope 及 exclusive/shared |
| `execution_work_item_states` | open/waiting_input/cancelled/superseded |
| `execution_work_assignments` | 当前与历史 owner、strategy、release、takeover |
| `execution_dispatches` | Room/Subagent durable outbox、lease、投递尝试 |
| `execution_attempts` | Agent/Subagent 执行尝试及 runtime identity |
| `execution_submissions` | append-only 交付声明、结果引用、证据 |
| `execution_review_dispatches` | Submission 到 reviewer 的独立投递 |
| `execution_acceptances` | 每条 Submission 的验收决策与 criteria results |
| `execution_events` | append-only command/event audit 与跨实体引用 |

`execution_completion_audits` 由 `00103_execution_completion_audits.sql` 建立。`internal/storage/orchestration` 的 repository 在每次 mutation 中重读 aggregate，执行 owner、binding、version/CAS 和 terminal fence，再在一个 SQL transaction 中提交状态与事件。

### 6.2 Runtime Graph

`00064_runtime_execution_graph.sql`：

- `runtime_graph_node_runs`：Agent/Subagent/Tool/Gate NodeRun；
- `runtime_graph_edge_runs`：invoke/spawn/guard/loop_back 等运行边。

Artifact 以 exact `agent_round_id + tool_use_id` 在读取时回挂到 Tool NodeRun。运行图只记录已发生的运行，不授权也不触发重试。

### 6.3 Workflow 与 Draft

- `00116_workgraph_workflows.sql`：
  - `workgraph_workflows`：owner、slash name、title、description、source provenance、objective、completion criteria、version；
  - `workgraph_workflow_nodes`：logical key、role、kind、抽象后的语义字段、required/terminal、parent、position；
  - `workgraph_workflow_dependencies`：模板内 hard/soft DAG。
- `00119_workgraph_workflow_drafts.sql`：`workgraph_workflow_drafts`（source scope、head/selected revision、editor identity、save state、lease）与 `workgraph_workflow_draft_versions`（每个 revision 的完整 `preview_json`）。
- `00134_workgraph_draft_saved_origins.sql`：`origin_workflow_id`。同一 owner/source Execution 的提炼 Draft 唯一复用；从不同命名 Workflow 恢复编辑时按 `origin_workflow_id` 隔离，避免两个命名图共享一份草稿。
- 命名图与 Draft 不承担执行状态机，不保存 Runtime Graph、Tool I/O、Agent、Assignment、Attempt、Submission、Review、Acceptance 或执行状态。

### 6.4 消息快照

普通 DM/Room 中成功的 authoring command 由 [internal/message/workgraph_artifact.go](../../internal/message/workgraph_artifact.go) 投影成 `workgraph_artifact` assistant content block，保存当时的完整草图/命名图快照，不随 Draft head 漂移。它不是真相：责任真相在 Orchestration SQL，草图真相在 Draft，命名图真相在 Workflow aggregate。
