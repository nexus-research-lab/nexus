# Execution Orchestration 协议

## 1. 文档目标

本文定义 Nexus 在单 Agent、子智能体和 Room 多 Agent 场景下统一的执行编排协议。它回答：

- Goal 何时存在、为何持续以及何时完成。
- Plan 如何把目标组织成有依赖的工作。
- Work Item、Assignment、Attempt、Submission 与 Acceptance 分别代表什么。
- 当前 Agent、子智能体、Room 成员和 Room Lead 各自拥有什么权限。
- 模型通过哪些稳定提示、动态上下文和工具观察、改变执行状态。
- 后端如何通过状态机、Hook 和 Room handoff 准入阻止重复、越权和错误完成。

本文暂不规定前端组件、布局或交互。前端未来只能投影本文的后端事实，不得创建第二套执行语义。

相关协议：

- [Room 协作协议](./room-collaboration-spec.md) 定义公共/私域通信、wake、reply route 和 handoff ledger。
- [Room 模块规范](./room-spec.md) 定义 Room、conversation、session 和成员结构。
- [Session Key 规范](./session-key-spec.md) 定义执行上下文的稳定作用域身份。
- [主智能体规范](./main-agent-spec.md) 定义 Nexus 主智能体的系统身份和平台职责。

## 2. 产品宪法

面向用户和模型的稳定规则是：

> Goal 决定什么必须跨越当前执行边界持续追求；Plan 决定工作怎样展开；Work Item 决定要交付什么；Assignment 决定由谁负责；子智能体帮助一个 Agent 完成自己的工作项；Room 负责让多个 Agent 的工作项可见地交接和协同；Acceptance 决定何时解锁下游以及完成 Goal。

更短的产品表述可以省略 Assignment 和 Acceptance，但后端协议不能省略它们。

## 3. 设计原则

### 3.1 一个事实只保留一个真相源

- 跨 Agent、跨 Room round、跨 runtime session 或绑定 Goal 的执行状态，以 Nexus Execution Orchestration 持久化状态为唯一真相源。
- SDK Task/Todo 是模型操作和局部进度投影，不是共享协作真相源。
- Room message、`@`、directed message 和 input queue 是通信与投递事实，不是 Plan 或完成状态。
- Hook 是 runtime 事件适配器和准入护栏，不是持久化真相源。
- Prompt 教模型如何判断，不能替代服务端状态机。

### 3.2 Goal 是持续性边界，不是复杂度容器

复杂任务可能在一个 execution boundary 内完成，因此只需要 Plan。简单任务也可能因为等待、重试或跨轮恢复而需要 Goal。

Goal 的判定问题是：

> 当前 objective 是否必须在本轮执行、当前上下文、预算边界、外部等待或 Agent 交接之后继续存在？

### 3.3 Plan 是 Work Item 图，不是权限模式

- Execution Plan 是一个有 revision 的 Work Item 有向无环图。
- Plan Mode 是只允许检查和提出方案的 runtime permission mode。
- Plan Mode 可以验证 Proposed Plan、transient replacement 或 abandonment，但不能写 Execution/Plan、激活 Assignment/Attempt 或触发 Goal continuation。

### 3.4 委派转移执行，不转移最终责任

- 把 Work Item 分配给 Room 成员时，Lead 仍负责依赖正确性、验收和最终整合。
- 当前 Agent 使用子智能体时，父 Agent 仍是 Assignment owner；子智能体只是该 Assignment 的 Attempt executor。
- worker 的结果是 Submission，不是 Acceptance。

### 3.5 并行必须显式且可证明

两个 Work Item 只有同时满足以下条件才可并行：

- 所有依赖已 Accepted。
- 输入在本次执行期间稳定。
- deliverable 不重合。
- exclusive output scope 不冲突。
- 不存在一项等待另一项结果的隐含依赖。

每个 `produce` Work Item 必须至少声明一个 typed canonical output scope；缺少 scope 的生产项不能进入 Plan，否则后端无法可靠阻止两个执行者重复交付同一结果。语法固定为 `file:<workspace-relative-posix-path>`、`dir:<workspace-relative-posix-path>` 或 `semantic:<nonempty-key>`：file 只按同一路径重叠，dir 与自身及任意后代 file/dir 重叠，semantic 按 key 精确重叠；只有双方都声明 `shared` 才允许重叠，任一方为 `exclusive` 都必须拒绝。

需要独立复核时必须显式声明 `review` 或 `verify` 关系，不能用两个相同的 `produce` Work Item 模拟复核。

### 3.6 Room/DM 是对话底座，Execution 是可选叠加层

系统不能把“当前 Room/session 存在 active Execution”解释成整个会话进入 managed mode。一个 Room 或 DM 在同一时间可以同时承载普通聊天、协调、受管工作、验收和后台 Goal continuation；Execution 只约束携带其可信 capability 的具体 round。

全局分为四个彼此正交的平面：

| 平面 | 真相 | 不负责 |
| --- | --- | --- |
| Conversation | Room/DM message、public/private handoff、`@`、用户定向消息与普通 input queue | Work Item ownership、依赖、Acceptance、Goal completion |
| Execution | Execution、Plan、Work Item、Assignment、Attempt、Submission、Acceptance 及其 outbox | 决定谁应参与闲聊、把消息正文解释成状态 |
| Persistence | Goal objective/revision、continuation、等待、恢复和 completion audit | 把复杂度或多人参与自动等同为 Goal |
| Runtime | 当前 round、slot、SDK session、subagent hook、interrupt 与 usage | 作为业务完成或责任真相源 |

因此 active Execution 是当前会话的后台事实，不是 Room 的全局开关；Goal 也是持续性 envelope，不是聊天模式。

### 3.7 每个 round 的 lane 只由可信 envelope 决定

模型每次被唤醒前，后端必须根据服务端注入的身份和 binding 选择 lane，不能从正文动词、`@` 数量、参与人数或模型自述猜测：

| lane | 可信依据 | 可做的事 |
| --- | --- | --- |
| conversation | Room round 没有 WorkBinding、ReviewBinding、exact Goal binding 或 round-scoped CoordinationBinding | 回应当前对话、使用普通任务工具；成员不能读取 WorkGraph，coordinator 只可显式 `get_execution` / `plan_execution` 进入协调面 |
| coordination | exact Goal ID/revision，或 Execution coordinator 在当前物理 round 显式调用 `get_execution` / 成功 `plan_execution` 后由后端 mint 的 CoordinationBinding | 建 Plan/replan、分配、解阻、接管、审计完成；普通聊天本身仍不成为工作证据 |
| work | 完整 WorkBinding | 只执行、阻塞、提交该 Assignment；不能修改 sibling 或自行验收 |
| review | 完整 ReviewBinding | 只审查 binding 指定的 immutable Submission；Room reviewer 不能用普通 coordinator 消息替代该 binding |
| subagent | parent WorkBinding + server-created child Attempt binding | 只帮助父 Agent 完成同一 Work Item；stop 只终结 child Attempt |

稳定不变量：

- 裸 `@` 永远产生 conversation round；即使目标已有 Assignment、Room 有 active Execution，也不激活责任。
- `assign_work` 的 durable Dispatch outbox 是创建 work lane 的唯一 Room 入口；`submit_work` 的 durable review-return outbox 是创建 review lane 的唯一 Room 入口。
- 普通消息不复制 source round 的 WorkBinding/ReviewBinding。worker 在受管 round 中 `@` 另一个成员，只会建立新的 conversation round。
- non-coordinator Room member 没有 binding 时不挂载 Execution MCP，后端直接调用也返回 `conversation_only`；Prompt 不是唯一护栏。
- coordinator 身份使 Agent 有资格进入协调面，但不是 round-start 的隐式 CoordinationBinding。普通 coordinator Room round 仍从 conversation 开始，只开放 `get_execution` 与 `plan_execution`；前者显式读取现有责任，后者显式建立或修订协调图。后端为成功转换 mint `(owner, session, agent, physical round, execution)` capability，其他 mutation 在 capability 缺失时返回 `conversation_only`，round 结束立即释放。Room `review_work` 仍要求目标 Submission 的 ReviewBinding。DM 单 Agent中 coordinator、self worker 与 reviewer 可以由同一 Agent 承担，责任仍由 self Assignment/Submission/Acceptance 显式表达。
- `get_execution` 在当前 scope 没有 Execution 时返回权威 `state=unmanaged`，不会凭一次读取创建 Execution 或 CoordinationBinding；该 round 仍是 conversation，只有完整且成功的 `plan_execution` 才建立首个 WorkGraph 并进入 coordination。
- 实时 steering/guidance 只进入已经存在的 target round，并继承 target round 的 lane；它不能把 conversation round 升级为 work，也不能把一个 Assignment 的 capability 带到另一个 slot。
- 携带 WorkBinding/ReviewBinding 的 queue item 必须保持 `delivery_policy=queue`、单一精确 target，并单独启动 round；它不能被改成 guidance、与相邻消息合并或在恢复时降级成普通 directed message。workspace write 与 realtime consume 两端都必须校验，坏的历史 envelope fail closed 后继续处理后续消息。
- Goal continuation 以 Goal Lead/coordinator 身份和 Goal/Execution binding 进入 coordination lane；Goal revision 不能经普通聊天或旧 round 静默改变。
- 普通 DM/Room round 不从 session 上的 ambient active Goal 获得 Goal capability。只有 internal continuation 的 exact Goal ID/revision、exact Work/Review Execution binding，或本轮成功 `create_goal` 后后端授予的 immutable authority 可以写 Goal progress、usage 或 completion evidence。
- `ExecutionWorkBinding` / `ExecutionReviewBinding` 只携带不可变 Execution responsibility chain，不复制 Goal ID/revision。若绑定的 Execution 属于 Goal，后端从 SQL Execution 解析 Goal identity，并同时校验当前 objective revision；Goal capability 不能从消息、队列正文或模型参数补写。
- cancellation 是后端控制消息，只按捕获时的 Attempt/runtime binding 生效，不向模型授予新 lane。

全局消息入口必须遵守同一矩阵：

| 生产者 → 消费者 | envelope | 结果 |
| --- | --- | --- |
| 用户 → Room member / DM Agent | 无 WorkBinding；可带用户目标和 Goal steering | conversation；DM 可直接决定，Room coordinator 通过显式 get/plan transition 决定是否进入 coordination |
| Agent → Agent 裸 `@` / directed message | 无 WorkBinding/ReviewBinding | conversation-only handoff，可多人并行聊天，不产生工作证据 |
| Assignment outbox → Room member | 完整 WorkBinding | 幂等 work wake，严格 admission 后启动既有 pending Attempt |
| Submission outbox → reviewer | 完整 ReviewBinding | 幂等 review wake，只开放指定 Submission 的 review |
| Goal continuation → Lead | Goal ID/revision + bound Execution identity | coordination continuation，不复用 stale revision |
| input queue / contextual guidance → active round | target round identity，不新增业务 binding | 只 steering 原 lane；ACK/重放不能扩大权限 |
| Agent tool → subagent | parent WorkBinding + tool_use_id → child Attempt | 同 Work Item 内的局部执行，不产生独立 Assignment |
| replacement/abandon/retarget → runtime | exact Cancellation binding | 终止旧物理执行；不作为对话、Submission 或 Acceptance |

## 4. 核心对象与关系

```text
Goal (0..1 current per session scope)
└── Execution (1..n)
    ├── Work Item (stable logical identity)
    │   ├── Work Item Spec (immutable revisions)
    │   ├── Assignment (0..1 current, n historical)
    │   │   ├── Dispatch outbox (0..n)
    │   │   └── Attempt (0..n)
    │   │       └── Cancellation outbox (0..1 per logical Attempt)
    │   └── Submission (0..n)
    │       ├── Review return outbox (Room, 0..1)
    │       └── Acceptance decision (0..1)
    ├── Plan Revision (0..1 active, n immutable)
    │   ├── Plan Item (Work Item + exact Spec)
    │   └── Dependency edges
    └── Execution Event (append-only)
```

### 4.1 Goal

Goal 是可跨 execution boundary 持续存在的 objective envelope。

Goal 负责：

- 保存 objective 和 objective revision。
- 保存持续推进、暂停、阻塞、预算和 usage 状态。
- 关联一个或多个 Execution。
- 在 objective 完成前触发受控 continuation。
- 根据 Objective、WorkGraph 和 runtime quietness 完成最终审计。

Goal 不负责：

- 直接表达步骤。
- 直接表达 Agent 分工。
- 用“存在 Goal”强迫 Room 委派。
- 扩大用户原始授权。

### 4.2 Execution

Execution 是一次实际推进用户请求或 Goal objective 的执行实例。

每个行动型请求可以创建一个 transient Execution；不是所有 Execution 都需要 Goal。

通过 `plan_execution` 首次创建 transient Execution 必须同时给出明确 objective、至少一条归一化后非空的顶层 completion criterion 和完整 Plan；普通首次 `plan_execution` 在同一 SQL transaction 创建 Execution 与 active Plan，不能先留下无 Plan 的 transient Execution。Plan Mode 对同一 proposal 执行结构化校验但不持久化。objective 与顶层 completion criteria 是该 Execution 的 immutable fence：同一 outcome 和完成边界下改变路线、拆分、owner、dependency 或 evidence strategy，属于已有 Execution 的 replan；显式改为不同 transient objective，必须由 `plan_execution` 同时提交 `replace_current_execution=true`、`replacement_reason` 和完整 successor graph；只放弃而不继续时使用 `abandon_execution`。Plan revision 不得借普通 replan 改写这些边界。

Execution 负责：

- 绑定 owner、session scope 和触发来源。
- 绑定可选 Goal ID 与 Goal objective revision。
- 表示一次执行是否 active、waiting、paused、completed、failed、cancelled 或 superseded。
- 提供跨 compact、resume、Room handoff 和 Goal continuation 的幂等身份。

同一 Goal 可以经历多个 Execution，但一次 continuation 必须明确恢复或新建哪个 Execution，不能从自然语言猜测。
当前 Plan 只由 `PlanRevision.status=active` 的唯一约束决定；Execution 不再保存第二个 active-plan 指针。

### 4.3 Plan Revision

Plan Revision 是某个 Execution 在一个时间点的工作图快照。

- 同一 Execution 同一时刻只有一个 active Plan revision。
- Plan Revision 一经创建即不可变；修改名称、依赖、必需项、deliverable、acceptance criteria 或展示文案都创建新 revision。
- Plan Item 把稳定 Work Item 与该 Plan 使用的精确 immutable Work Item Spec 绑定。
- 新 revision 必须明确哪些 Accepted Work Item 在 `spec_hash`、输入与 objective fence 一致时复用，哪些被 supersede。
- Goal objective revision 变化时，旧 Plan revision 默认 stale；只有经过 rebase 的结果可以复用。

### 4.4 Work Item

Work Item 描述一个可执行、可分配、可提交、可验收的交付单元。

最小字段：

| 字段 | 语义 |
| --- | --- |
| id | 稳定 Work Item 身份 |
| execution_id | 所属 Execution |
| current_spec_revision | 当前逻辑项的最新 Spec revision |
| status | open、waiting_input、cancelled、superseded；其余执行态均派生 |
| version | optimistic concurrency version |

Stable Work Item 保存不会随 revision 漂移的 `kind`；Work Item Spec 保存 `subject`、`objective`、`deliverable`、`acceptance_criteria` 与 `input_refs`。Plan Item 保存当前 Plan 中的 parent、required、terminal、顺序和 dependency membership；规范化 Plan Output Claim 保存该 Spec 在当前 Plan 的 exclusive/shared output scope。

Work Item 不记录“模型说自己正在做什么”；真实运行由 Assignment 和 Attempt 表达。`ready`、`blocked-by-dependency`、`assigned`、`running`、`submitted` 和 `accepted` 是由当前 Plan、Acceptance、Assignment、Attempt 与 Submission 推导的 projection，不作为第二套可随意写入的 lifecycle state。

所有会进入模型执行契约的单个集合统一以 protocol 常量限制为 32 项：Execution completion criteria、Work Item acceptance criteria、单个 Work Item 的直接 dependency、`input_refs`、`output_scopes`、Submission `result_refs` / evidence、Acceptance criteria results 及每条 result 的 evidence、Resume evidence。MCP schema 声明 `maxItems=32`，service 在 normalization 前以稳定 `projection_limit_exceeded` 拒绝第 33 项，storage 对直接 command 重复同一校验；Plan Mode 也执行完整限制校验但不写入。新写入受限后，动态 context 对异常历史数据只可用 `truncated="true" total="N"` 明示有界投影，structured WorkBinding 与 Dispatch contract 则 fail closed，禁止无标记 slice、假装完整或依赖模型猜遗漏项。

Plan Work Items 集合本身也使用同一个 32 项上限；schema 与 service normalization 必须在任何单项校验和持久化之前一致拒绝第 33 项。

`file:` / `dir:` scope 的 canonical 展示和持久化保留用户原大小写，但冲突、同 Work Item 去重和 ancestor/equal 判断统一使用 Unicode NFC 后的 case-fold comparison key，以保守覆盖 macOS/Windows 等大小写不敏感 workspace；`semantic:` key 始终保持大小写敏感 exact comparison。comparison key 不是另一个持久化 scope。

### 4.5 Assignment

Assignment 表示当前由谁对 Work Item 的交付负责。

最小字段：

| 字段 | 语义 |
| --- | --- |
| id | 稳定 Assignment 身份 |
| work_item_id | 目标 Work Item |
| plan_id / work_item_spec_id | Assignment 获得授权时的精确工作图 fence |
| owner_agent_id | 对交付负责的持久 Nexus Agent；当前 Agent 与 Room member 使用同一身份类型 |
| assigned_by_agent_id | 分配者 |
| return_to_agent_id | 提交后负责验收/整合的 Agent |
| status | assigned、active、released、completed、cancelled |
| assignment_reason | 选择该 owner 的理由 |
| takeover_reason | 接管或重分配原因 |
| version | optimistic concurrency version |

稳定不变量：

- 一个 Work Item 同一时刻最多一个 current Assignment。
- Assignment 只能引用同一 Execution、同一 Plan 中的 Work Item/Spec 组合。
- Assignment owner 与 Attempt executor 可以不同，但 owner 始终承担结果责任；`subagent` 只允许出现在 Attempt executor_kind，不是 Assignment owner/strategy。
- Room Lead 可以分配共享 Work Item；普通成员不能重分配 sibling Work Item。
- 单 Agent managed Execution 中，自己执行也需要显式或派生的 self Assignment，避免“无人负责但状态 running”。Room self Assignment 还必须有独立 reviewer 路径。

### 4.6 Attempt

Attempt 表示一次真实执行尝试。

最小字段：

| 字段 | 语义 |
| --- | --- |
| id | 稳定 Attempt 身份 |
| assignment_id | 对应 Assignment |
| executor_kind | agent、subagent |
| executor_agent_id | 实际执行主体 |
| parent_agent_id | 子智能体的父 Agent |
| runtime_session_key / sdk_session_id | Nexus 与 SDK 会话 identity |
| runtime_round_id / root_round_id / agent_round_id | 物理 wake、逻辑因果与 slot identity |
| parent_attempt_id | 子智能体 Attempt 的父 Attempt |
| child_session_id / sdk_task_id / tool_use_id | 子会话、SDK Task 与工具调用关联 |
| status | pending、running、succeeded、failed、interrupted、cancelled、timed_out |
| started_at / finished_at | 执行时间 |
| parent_round_exited_at / reconcile_after | parent round 已退出的 durable grace 起点与 UTC deadline；只允许 child Attempt |
| failure_reason | 失败证据 |
| version | optimistic concurrency version |

稳定不变量：

- 一个 Assignment 默认最多一个 running Attempt。
- 只有显式的 redundant verification policy 才允许多个并行 Attempt。
- `SubagentStart` 只能激活一个已经绑定 Assignment 的 Attempt。
- `SubagentStop` 只结束 child Attempt，并把结果证据交还父 Agent；不能自动创建 Submission，更不能直接 Acceptance。
- child terminal evidence 以当前 Assignment 下最新的 `subagent_result` 重新投影，只包含 Attempt/status、是否有最后消息与 transcript ref；实际 Agent tool result 仍属于父 round。若父 round 已结束，迟到结果保留为 child Attempt evidence，后继父 round可以显式读取 transcript、重试或整合，但系统不自动唤醒、不自动 Submission，也不把它归给 successor Assignment。
- parent physical round 退出时，runtime manager 用退出时刻 `T` 计算固定 30 秒 grace，并在释放 round callback 前通过冻结的 `tool_use_id → callbacks/parent round` binding 持久化 `parent_round_exited_at=T`、`reconcile_after=T+30s`。该 deadline 是 SQL truth，不是进程内 timer 或可配置的模型参数。
- 独立于 Room realtime/dispatch 是否装配，backend 在启动时以及每 1 秒、每批最多 32 条扫描过期 running child；每次用 Execution/Attempt CAS 把它收束为 `interrupted`，CAS/数据库暂态失败留给下一轮无限重试，terminal/stale graph 则幂等跳过。runtime manager 同时保留进程内低延迟 fallback：按同一绝对 deadline 在 T+30s 第一次尝试，失败后分别等待 60s、120s，最多三次；durable callback 耗时不顺延第一次尝试，重启会丢失该 timer，但不会丢失 SQL deadline。
- deadline 前到达的 Stop 正常写入真实终态并使恢复器跳过；deadline 后到达的 Stop 命中同一 terminal Attempt 幂等返回，不能被新 round 吸收，也不能把已审计的 interrupted 重新写成 succeeded。持久化 deadline 失败时 runtime 立即依赖同一 frozen failure callback 收口；任何路径都不得把未知 child 静默视为成功。
- 任何主动要求把 `pending | running` Attempt 改为 `interrupted | cancelled` 的 orchestration control transition，都必须在同一个 SQL transaction、且在状态更新之前，为本次将被收束的每个逻辑 Attempt 写入唯一 `ExecutionCancellationDispatch`。不能先写 terminal 状态，再依赖进程内回调猜测曾经 live 的 target。runtime/slot/subagent callback 在物理执行已经结束后回写终态只是 terminal evidence，不再反向创建 cancellation intent。
- Cancellation dispatch 同时保存完整 WorkGraph fence、逻辑 `attempt_id` 与实际可中断的 `runtime_attempt_id`。root Attempt 两者相同；subagent child Attempt 保留自己的 child/task/tool identity，但物理 target 指向承载它的 root Attempt/runtime round。
- `pending` Attempt 明确落为 `target_kind=not_started`，consumer 以 `not_required/not_started` 收束；已经 `running` 但缺少精确 runtime identity 时落为 `target_kind=unavailable`，以 `unsupported` 暴露限制。两者都不能伪报已经物理中断。

### 4.7 Submission、Review 与 Acceptance

Submission 是 worker 对 deliverable 的完成声明及证据。

最小字段：

- submission id
- work item id
- assignment id
- attempt id
- submitter agent id
- result summary
- result refs
- evidence
- created at

Review 由有权限的 reviewer 对 Submission 作出：

- accept
- reject with feedback

Acceptance 是独立、append-only 的审查决定，保存 submission、decision、reviewer、逐条 criteria result、feedback 和 causal round。Submission 本身不可变，也不保存可被覆盖的 review 状态。一个 Submission 最多一个 Acceptance decision；reject 后通过新的 Attempt 和 Submission 返工。

Room Submission 还会在同一 SQL transaction 创建独立的 `ExecutionReviewDispatch`。它只负责把待审 Submission 可靠回交给 Assignment 预先规范化的 `return_to_agent_id`，不复用 worker Dispatch，也不创建 reviewer Assignment/Attempt。投递到 Room 后使用独立 `ReviewBinding`，完整绑定 Execution、Plan、Work Item、Spec、Assignment、Submission、review dispatch 与 reviewer target。

只有 `decision=accept` 的 Acceptance 才能：

- 完成 Assignment。
- 解锁 dependent Work Item。
- 计入 Execution/Goal 完成审计。
- 作为可复用结果进入后续 Plan revision。

## 5. 状态机

### 5.1 Execution 状态

```text
active ───────→ waiting ───────→ active
  │                 │
  ├──────────────→ paused
  ├──────────────→ failed
  ├──────────────→ cancelled
  ├──────────────→ superseded
  └──────────────→ completed
```

- `waiting` 表示仍有未完成工作，但当前没有可立即运行的本地动作，例如等待 Room member、subagent 或外部状态。
- `paused` 是用户或系统明确暂停，不自动恢复。
- `failed` 表示本次 Execution 失败；若绑定 active Goal，Goal 可以创建 recovery Execution。
- `cancelled` 表示 coordinator 依据用户明确放弃通过 `abandon_execution` 终结 transient Execution，且没有 successor。
- `superseded` 表示用户明确用新的 transient objective 替换本次 Execution；它必须指向同一 owner/session 下由同一 command 创建的 successor，不能作为普通 replan 的终态。
- `completed` 必须通过 required Work Item 审计。

### 5.2 Work Item 状态

```text
stable lifecycle:
open ↔ waiting_input
open | waiting_input → cancelled | superseded

derived delivery projection:
ready → assigned → running → submitted → accepted
                    │             └─ reject/changes_requested → ready
                    └─ failed/interrupted → ready | cancelled
```

只有第一组写入 stable Work Item state。第二组由当前 active Plan、dependency Acceptance、current Assignment、Attempt、Submission 与 Acceptance 推导；调用方不能任意写入或跳转。显式外部阻塞作为 `waiting_input` 及带证据的 blocker 保存，不与 dependency readiness 混为一谈。

### 5.3 Goal 状态与完成条件

Goal 继续使用 active、paused、blocked、budget_limited、usage_limited 和 complete。

Goal 可以进入 complete，当且仅当：

```text
Goal objective revision 与 Execution binding 一致
AND 当前 active/recovery Execution 已 completed
AND 所有 required Work Item 已 accepted
AND terminal integrate/verify Work Item 已 accepted
AND 没有 running Attempt
AND 没有绑定当前 Execution 的待激活 Dispatch、Room handoff
AND 没有绑定当前 Execution 的必须消费 input queue item
AND 没有 pending Submission
AND terminal Goal usage 已结算
```

Room 中“某个非 Lead Agent 回复过”不是完成证据。多 Agent 协作只由被非 Lead owner 提交并最终 Accepted 的 required Work Item 证明。

## 6. Goal 激活与自适应提升

### 6.1 激活来源

Goal 必须记录：

```text
origin:
  user_explicit
  adaptive_initial
  adaptive_promoted

activation_reason:
  persistence_requested
  observed_boundary
  room_dependency_chain
  external_wait
  scheduled_retry
  context_boundary
  recovery_required
```

`context_boundary` 是当前唯一的上下文容量类枚举：后端观察到的 compact/context boundary 与受信 usage exhaustion 都映射到它，不新增 `usage_boundary`。二者必须通过不同的 runtime command/event provenance 供审计，但 Goal 记录的 activation reason 相同。

### 6.2 判定规则

```text
should_promote_execution_adaptively =
    every(hard_gate)
    AND (
        any(observed_persistence_signal)
        OR any(predicted_durable_signal)
    )
```

这是 adaptive promotion 的判定。用户显式要求 Goal 仍走 `create_goal` 的显式路径；显式路径不需要先观察 durable signal，但仍必须满足 objective clarity、原始授权、当前 permission mode 和 Goal service policy。

Adaptive promotion 的硬门槛全部满足：

- objective 明确且 completion criteria 可形成。
- 原始用户 scope 授权继续推进；promotion 不增加权限。
- 当前不是 Plan Mode，用户没有关闭自动 Goal。
- 当前有可绑定的 Execution，且同一 scope 没有另一个 active Goal。
- 剩余工作是 required，而不是可忽略的后台优化。

至少一个结构化持续性信号成立：

- 已观察：当前 boundary 到达时仍有 required Work Item、pending Submission、未完成 Assignment/Attempt、待消费的绑定 handoff，或 recovery 正在恢复旧 Execution。
- 可预测且 durable：已创建顺序 Room dependency/dispatch、必须等待外部状态、已安排跨 turn retry，或已知 context/usage boundary 前无法完成 required work。

`context_boundary` 不能由模型传入布尔值证明。runtime 收到真实 `compact_boundary` system message，或后端观察到受信 usage exhaustion 时，以 runtime actor 写入带 CAS、typed command provenance 和审计事件的同一种 Execution evidence；模型刷新 `get_execution` 后，才可能看到对应 promotion affordance。当前实现已接入 `compact_boundary`；usage source 只有在 backend usage accountant 明确记录后才能走同一入口，不能由 token 猜测代替。`scheduled_retry` 同理只能由实际 scheduler 调用内部 evidence 入口，单次 API retry 或模型口头承诺稍后重试都不成立。

Room 中只有已绑定到其他 Agent 的 required Assignment/Dispatch，或尚未完成的真实跨 Agent dependency chain，才能构成 Room durable signal。房主自己的顺序 hard dependency、未分配的草稿依赖和 optional Work Item 的等待都不触发 Goal。

以下只能作为建图证据，不能单独触发 Goal：

- 提示词很长、步骤很多或模型主观认为“复杂”。
- 使用了 Plan 或子智能体。
- Room 中有多个 Agent，但所有交付可在当前 boundary 内完成。
- 只剩 runtime-local、optional 或不可验收的后台工作。

“自动提升”不是后台定时器凭复杂度自行创建 Goal。后端先从 WorkGraph、runtime evidence、Goal policy 与配置计算并投影 `goal_promotion` affordance；只有模型在 coordination lane 选择并调用 `promote_execution_to_goal`，后端才重新验证同一证据并提交 promotion。用户显式要求持续追求则走独立 `create_goal` 路径。没有足够证据时保持 transient Execution；未知情况 fail closed，不猜测持久化意图，也不绕过权限、Plan Mode 与 objective clarity 门槛。

### 6.3 Execution promotion

不确定任务先创建 transient Execution。当继续推进需要跨 execution boundary 时，将同一个 Execution 原子提升为 Goal-bound Execution：

- 保留 Execution ID。
- 保留唯一 active Plan revision。
- 保留 Work Item、Assignment、Attempt、Submission 和 Acceptance。
- 记录 `goal_promoted` 事件。
- 禁止重建或复制 Plan。
- 同一 Execution 的 promotion 使用 idempotency command ID，重复调用返回同一 Goal binding。
- 记录判定来源、满足的 persistence signals 与 objective/Execution version fence，便于恢复和审计。

Goal 延长 intent 的生命期，不扩大权限。遇到需要新授权、危险操作或重大用户选择时，Goal 必须 blocked。

### 6.4 显式 Goal 与 Execution 收敛

用户显式要求持续追求时走 `create_goal`，不伪造 adaptive signal。显式路径与 Execution 仍只有一条状态链：

- scope 中已有兼容 transient Execution：`create_goal` 创建或复用 Goal 后，以 `user_explicit + persistence_requested` 绑定该 Execution，保留其 Plan 和全部执行历史。
- 先有 Goal、后调用 `plan_execution`：Goal metadata 先 CAS 预留唯一 Execution ID，Ensure 用该 ID 创建已绑定 Execution；中途失败重试仍复用同一 identity。
- 重复调用只修复尚未落地的一侧，不创建第二个 Goal、Execution 或 Plan。
- objective、scope 或已有 binding 不兼容时，分别返回稳定的 `goal_objective_conflict`、`goal_scope_conflict`、`goal_binding_conflict`，不能静默分叉。
- Goal completion 必须找到 objective revision 一致的绑定 Execution，并通过同一 WorkGraph completion audit；缺 binding、binding 冲突或审计器不可用均 fail closed。

### 6.5 Goal objective revision rebase

Goal 一旦绑定 managed Execution，objective 变化不再是 Goal row 的普通字段更新，也不是当前 Execution 内的 replan。MCP `retarget_goal`、HTTP Goal PATCH 与 app-server `thread/goal/set` 必须在各自完成来源授权后进入同一个 application coordinator；任何缺少 coordinator 的 managed Goal mutation 都 fail closed。Room 模型入口只有当前 Goal Lead 可以调用，用户 HTTP 入口保留 server-owned Room creator/lead metadata，不能借 metadata replacement 改写 Execution binding、activation provenance、completion criteria 或 transition state。

跨 Goal 与 Execution 两个持久化域采用可重放 saga，不伪装成单库 transaction：

1. **prepare Goal transition。** 以 `(goal_id, old_objective_revision, normalized_requested_objective)` 生成稳定 transition、command 与 reserved successor Execution ID；CAS 写入 server-owned `objective_transition`，状态为 `prepared`。Transition 至少保存 old/new revision、old/reserved successor Execution ID、用户 requested objective、首次确定的 canonical target objective、reason 与 source。重试先匹配持久化 requested objective，不能因后台 objective rewriter 再次运行并产生不同措辞而分叉第二个 revision。此时 canonical Goal objective/revision 尚未改变。
2. **supersede old WorkGraph。** 按精确 Goal ID 与 old objective revision CAS terminalize 旧 Execution：Execution/active Plan/未完成 Work Item 进入 `superseded`，current Assignment release，live Attempt 逻辑进入 `interrupted`，未送达 Assignment/Review outbox cancel；同一 SQL transaction 必须先为每个将被中断的 Attempt 写入 exact-target Cancellation dispatch，由独立 consumer 重试物理中断。Submission、Acceptance 与 event 历史保持 immutable。旧 Execution 的 `execution_superseded` event 必须保存 reserved successor ID 与 old/new Goal revision。
3. **commit Goal revision。** 只有旧图完成 fencing 后，Goal 才 CAS 切换 target objective/new revision，把 reverse `execution_id` 指向 reserved successor，删除旧 completion criteria，并进入 `awaiting_plan`。这不是完成状态。
4. **create successor graph。** 下一次非 Plan Mode `plan_execution` 必须复用 reserved successor ID，并在一个 SQL transaction 内创建 Goal-bound successor Execution、完整 WorkGraph 与第一版 active Plan。Successor 的 typed `replaces_execution_id` 必须指向 `superseded` predecessor，且 owner/session/scope/Goal/revision 必须连续；数据库还必须验证 predecessor supersede event 预留的就是该 successor，不能从任意 terminal Execution 伪造 revision chain。
5. **confirm binding。** Successor Execution 与 Plan commit 后，Goal transition 从 `binding_reserved` 进入 `bound`，写入新 completion criteria。只有 `bound` 后 Goal completion 与自动 continuation 才重新开放。

`prepared | awaiting_plan | binding_reserved` 均视为 pending：Goal complete、自动 continuation、旧 revision mutation 与迟到 Room wake/child result 必须 fail closed；模型上下文在 `prepared` 明确要求用同一 target 重试 `retarget_goal`，在 `awaiting_plan | binding_reserved` 明确给出 `plan_execution` next action。相同 command 重试只修复未完成阶段并复用同一 identity；不同 target、revision 或 successor 冲突。首次 successor Plan 在 Plan Mode 只验证结构，不预留 binding、不创建 Execution/Plan，也不确认 transition。新 graph 默认不 carry 旧 Work Item、Assignment、Attempt、Submission 或 Acceptance；未来若支持 reuse，必须是独立、显式且重新验收的协议。

## 7. 权限与角色

### 7.1 单 Agent

当前 Agent 默认同时承担：

- coordinator
- self Assignment owner
- reviewer
- final integrator

它可以把自己 Assignment 下的独立子工作交给 subagent Attempt，也可以作为 coordinator 替换或放弃当前 transient Execution，但不能把用户关系、Execution replacement/abandonment 权限和最终验收转移给 subagent。

### 7.2 Room Host 与 Goal Lead

- 没有 Goal 时，Room host 是当前 root execution 的默认 coordinator。
- active Room Goal 存在时，Goal Lead 是 Goal-bound Execution 的 coordinator 和 shared WorkGraph reviewer。
- creator、host 与 Goal Lead 的身份必须由后端上下文明确给出，不能让模型从名字推断。

Lead 负责：

- 创建和修订 shared Plan。
- 定义依赖和验收标准。
- 分配 Ready Work Item。
- 验收或退回 Submission。
- 处理失败、阻塞、接管和重分配。
- 确保最终 integrate/verify Work Item 被独立执行和验收，并据此完成最终交付。
- 发起 Goal completion audit。
- 在用户明确改变或放弃当前 non-Goal objective 时发起 transient Execution replacement/abandonment；Goal-bound objective 变化仍走 Goal revision/rebase。

Lead 若亲自执行 shared Work Item，仍不能 Acceptance 自己的 Submission；该 Work Item 必须由另一个获授权 reviewer、用户或系统验收。Lead 作为 coordinator 的权限不能消除 Room 中生产与验收的分离。当前协议尚未持久化独立 `reviewer_agent_id/reviewer_kind`，也没有 user/system Review admission，因此后端暂时以 `room_independent_reviewer_required` 拒绝所有 Room self Assignment；正式的 produce、integrate 和 terminal verify Work Item 必须分配给其他成员，由 Lead 验收后负责最终回复。只有独立 reviewer 身份、唤醒和权限链全部落地后才能重新开放。

### 7.3 Room 成员

Room 成员可以：

- 读取与自己 Assignment 有关的稳定输入。
- 开始、阻塞、提交自己的 Work Item。
- 在自己 Assignment 范围内使用 subagent。
- 向 Lead 提议 Plan 修改或新 Work Item。

Room 成员不能：

- 修改 sibling Work Item。
- 分配共享 Work Item，除非被显式授予 coordinator 权限。
- Acceptance 自己的 Submission。
- 完成 shared Goal。
- 替换或放弃当前 shared Execution。
- 重复执行已经归属其他成员的 produce Work Item。

### 7.4 子智能体

子智能体是父 Agent Assignment 下的一次 Attempt executor。

子智能体不能：

- 创建、重定向、暂停或完成 Goal。
- 创建、替换、取消或完成 Execution。
- 修改 shared Plan。
- 分配 Room 成员。
- 使用 Room public/directed communication 工具。
- Acceptance 自己的结果。

子智能体可以在本地使用临时 Task/Todo，但这些局部步骤不成为 shared WorkGraph 的第二真相源。

原生 `Agent` 工具必须经过 `PreToolUse` 强准入，不能只依赖 Prompt 自觉。后端只在以下条件同时成立时放行：

1. 当前不是 Plan Mode，且 actor 属于权威 managed Execution。
2. 当前 Agent 恰好拥有一个 bounded、未完成的 current Assignment；零个或多个都拒绝。
3. 该 Assignment 恰好存在一个 pending/running parent Agent Attempt。
4. 该 Assignment 不存在 pending/running child subagent Attempt。
5. 后端用本次 `tool_use_id` 原子预留 child Attempt；模型不提供 Execution、Assignment、Attempt 或 version。

动态 `<nexus_execution_context>` 必须投影同一准入结果：`subagent_admission` 包含 `eligible`、原生工具名 `Agent`、`candidate_assignment_count`，以及 eligible 时的唯一 Assignment/parent Attempt 或拒绝时的稳定 `reason_code`。只有 eligible 时，`allowed_actions` 才包含 `Agent`。这只是原生 Agent/Task 能力的准入提示，不新增 Orchestration MCP 工具。

`tool_use_id → child Attempt` 是 launch binding，也承担重复调用 fence。runtime manager 按物理 parent round 注册 callback；`PreToolUse` 成功时把 callback、parent round、tool ID 与当时可见的 SDK session/task identity 冻结为 immutable lifecycle binding。后续 `SubagentStart/Stop/PostToolUseFailure` 只能命中该 binding：优先使用可信 callback correlation，再使用唯一的 task/SDK Agent identity；零匹配或多匹配都 fail closed。parent round 结束只撤销新的 launch capability，已创建的 child binding 保留到迟到 Stop/重复事件完成幂等收口，不能被 successor round 覆盖。不能把 SDK `agent_id` 猜写成 `child_session_id` 或 `sdk_task_id`。`SubagentStop` 只终结 child Attempt，不自动产生 Submission、Acceptance 或 Goal completion evidence。

当前 WorkGraph admission 仍明确限制同一 parent Assignment 同时最多一个 active child subagent；不同物理 parent round 可以安全共存，但无 correlation 的 lifecycle 绝不按“最新 round”路由。需要真正并行的独立 Work Item 时，优先分配给不同 Room Agent。

## 8. 模型决策协议

模型在 substantial execution 前按固定顺序判断：

1. 当前是 Plan Mode 吗？如果是，只验证 Proposed Plan、transient replacement 或 abandonment，不执行任何 mutation。
2. 当前 round 是 conversation、coordination、work、review 还是 subagent？conversation 先正常回应；只有 Room coordinator 判断请求确需可追踪交付时，才显式调用 `get_execution` / `plan_execution` 进入 coordination。
3. 若已进入 coordination/work/review，是否已有 Execution Context？如果有，先恢复而不是重建。
4. 用户是在改变同一 objective 的执行路线、改为不同 transient objective，还是只放弃？前者普通 replan；第二种由 coordinator 用带 `replace_current_execution` 的完整 `plan_execution` 原子创建 successor graph；第三种用 `abandon_execution` 只取消旧 graph。Goal-bound Execution 对后两者返回 `goal_retarget_required`。
5. 用户是否明确要求持久追求，或当前 Execution 是否需要自适应 Goal？
6. 请求是否是一个原子 deliverable？
7. 如果不是，创建或修订 Plan revision。
8. 读取 Ready Work Item。
9. 为每个 Ready Work Item 选择 self、subagent 或 Room member。
10. 并行前验证 dependency、input stability 和 output scope。
11. 等待 Submission，执行 Review。
12. 解锁下游并重复。
13. 执行 terminal integration/verification。
14. 运行 Execution/Goal completion audit。

执行策略边界：

| 策略 | 使用条件 |
| --- | --- |
| direct/self | 原子工作，或协调成本高于委派收益 |
| Plan | 多个有意义步骤、依赖、并行分支或需要恢复 |
| subagent | 临时、边界明确、结果只需返回当前 Agent 的工作 |
| Room member | 需要稳定角色、公开责任、跨轮交接或共享可见结果 |
| Goal | objective 需要跨 execution boundary 持续存在 |

## 9. Prompt 分层

### 9.1 Stable execution contract

所有主智能体、普通 Agent、DM、Room member 和 Goal continuation 共享同一份稳定核心规则。

稳定规则只描述：

- Direct、Plan、subagent、Room assignment 和 Goal 的选择边界。
- 一个 Work Item 一个 primary owner。
- dependency 与并行约束。
- 委派后不重复执行。
- Submission 与 Acceptance 的区别。
- takeover 必须有原因。
- 同 objective replan、different transient objective replacement 与 abandonment 的选择边界。
- Plan Mode 禁止执行。

Main、Base、Room 和 Goal continuation 不得各自复制并演化另一套相同规则。

### 9.2 Surface-specific stable contract

- Main Agent 只追加平台 routing 和联系人/Room 管理职责。
- Room 只追加 public/private visibility、turn、`@` 和 reply route 语义。
- Goal continuation 只追加持续推进、预算、blocked audit 和 completion audit。
- subagent 只追加父 Assignment、返回格式和禁止共享状态修改。

### 9.3 Dynamic execution context

每轮生成一个权威 `<nexus_execution_context>`：

```xml
<nexus_execution_context execution_version="17">
  <scope type="room" session_key="room:group:conversation-1" />
  <actor agent_id="analyst" role="member" />
  <mode plan_only="false" />
  <goal id="goal-1" objective_revision="3" />
  <execution id="execution-1" status="active"
             plan_id="plan-4" plan_revision="4">
    <objective>交付一份经过验收的 M3/M4 对比报告</objective>
    <completion_criteria>
      <criterion>terminal verification accepted</criterion>
    </completion_criteria>
  </execution>
  <assigned_work>
    <item id="work-2"
          logical_key="W2"
          spec_id="spec-2"
          assignment_id="assignment-2"
          attempt_id="attempt-2"
          status="running"
          kind="produce">
      <subject>M3 与 M4 差异分析</subject>
      <objective>只消费已经验收的 W1 证据</objective>
      <deliverable>带证据强度的差异矩阵</deliverable>
      <acceptance_criteria>
        <criterion>每项结论可追溯到已验收来源</criterion>
      </acceptance_criteria>
      <depends_on>
        <work_item_id>work-1</work_item_id>
      </depends_on>
    </item>
  </assigned_work>
  <active_assignments />
  <ready_work />
  <pending_reviews />
  <goal_promotion eligible="false">
    <blocker>goal_already_active</blocker>
  </goal_promotion>
  <execution_transition replace_current_allowed="false"
                        abandon_allowed="false"
                        validation_only="false"
                        reason_code="wrong_owner" />
  <subagent_admission eligible="false"
                      native_tool="Agent"
                      candidate_assignment_count="1"
                      reason_code="active_child_attempt">
    <reason>the Assignment already has an active child Attempt</reason>
  </subagent_admission>
  <action_scope>allowed_actions and forbidden_actions govern Execution orchestration controls only; normal task tools remain governed by the task and tool policy</action_scope>
  <allowed_actions>
    <action>get_execution</action>
    <action>submit_work</action>
    <action>block_work</action>
  </allowed_actions>
  <forbidden_actions>
    <action>plan_execution</action>
    <action>abandon_execution</action>
    <action>assign_work</action>
    <action>review_work</action>
    <action>resume_work</action>
    <action>take_over_work</action>
    <action>promote_execution_to_goal</action>
    <action>Agent</action>
    <action>mutate_sibling_work</action>
  </forbidden_actions>
  <completion_blockers>
    <blocker>W2 has not been accepted</blocker>
  </completion_blockers>
</nexus_execution_context>
```

动态上下文只包含当前 actor 做决定所需的有界状态，不转储完整事件日志。

`allowed_actions` 只能表达工具级 affordance，但 `plan_execution` 同时承载普通 replan 与 Execution replacement，因此动态上下文还必须投影 `execution_transition`。`replace_current_allowed` 控制 `replace_current_execution` flag，`abandon_allowed` 控制 `abandon_execution`，`validation_only` 标记 Plan Mode 的 write-free proposal。transient coordinator 为 true；Room member/subagent 为 false 并带 authority reason；Goal-bound coordinator 仍可拥有普通 `plan_execution` replan，但 replacement/abandonment 为 false 且稳定 `reason_code=goal_retarget_required`。

每个 Execution mutation 与 `get_execution` 也必须返回最新 actor-specific context。模型在同一 turn 内始终服从最高 `execution_version`：较新的 tool result 覆盖 round-start context；若返回 `context_status=refresh_required`，必须先调用 `get_execution`，不能拿旧的 forbidden/allowed 列表继续猜。

runtime 发出 `compact_boundary` 后，DM/Room 执行器先把该事实写入 SQL Execution evidence；模型必须在下一次编排决策前调用 `get_execution`。这次刷新会返回新的 `execution_version` 和由后端重算的 Goal promotion affordance，模型不从 system message 文本自行推断权限。

注入时机：

- runtime/session start
- user prompt submit
- compact 后由新的 `SessionStart(source=compact)` 恢复
- Goal continuation 前
- Work Item、Assignment、Attempt、Submission 或 Review 改变后
- Room handoff 接受、拒绝或失败后

当前 runtime 的 `PostCompact` additional context 不会进入下一次模型上下文，因此不能把它当恢复入口。恢复必须重新读取 SQL snapshot，并由 compact 后的 SessionStart 注入；transcript 和 Hook payload 都不是真相源。

### 9.4 Agent 可调用性原则

如果把模型当成 Execution Orchestration 的真实使用者，控制面必须满足：

1. **先给定位，再给工具。** 每轮上下文必须明确 actor role、当前唯一 Assignment、dependency、deliverable、acceptance criteria、completion blocker 和 exact allowed actions；不能要求模型从聊天历史重建这些事实。
2. **模型只做语义决策。** 模型负责 Plan、分配、提交、验收、阻塞、接管和 promotion proposal；`running`、Attempt identity、outbox claim、event sequence、version increment 与恢复由工具适配器、Hook 和服务端自动维护。
3. **原子任务零工具税。** 一个当前 boundary 内可完成的小交付可直接执行；不能为了展示“进度”强迫模型创建 Goal、Plan、Work Item 或手工 start/complete 状态。
4. **一次提交完整意图。** `plan_execution` 通过一个 `work_graph_json` 字符串批量提交完整 WorkGraph，服务端严格解码其中的 JSON array、mint opaque IDs，并返回 logical key → ID 映射、Ready 项和下一动作。禁止让模型用多个低级 create/update 调用拼半成品 Plan。
5. **不让模型维护并发控制。** MCP adapter 从当前 snapshot 注入 expected revision；优先用 `tool_use_id` 形成 idempotency command ID，bridge 未提供时使用 server session/round、工具名、canonical input 与 snapshot revision 的稳定 hash。应用服务再按具体 mutation 派生不同的 event command key，因此一次复合工具调用的 Ensure、Plan 或 Goal binding 子步骤不会互相冲突。底层内部 API 仍强制 CAS。发生 stale 时返回新 snapshot 和可重试动作，不让模型猜 version。
6. **工具结果必须可恢复。** 每个 mutation 返回 `applied | no_op | rejected`、稳定 reason code、最新 snapshot revision、实际改变和有序 `next_actions`。错误必须说明怎样修复，例如修正 `work_graph_json`、先验收哪个 dependency、由哪个 owner 提交、或先刷新哪个 Plan。
7. **只暴露当前可用 affordance。** dynamic context 的 `allowed_actions` 使用真实工具名，并同时受当前状态与 round lane 约束：没有 Ready Work 就不暴露 `assign_work`，没有自己的 current Assignment/WorkBinding 就不暴露 `submit_work`，Room coordinator 没有目标 Submission 的 ReviewBinding 或没有待审 Submission时不暴露 `review_work`；有 unreviewed Submission 的 Work Item 不再成为 `block_work` / `take_over_work` 候选，但其他安全 Work Item 仍可继续并行，`active_assignments` 逐项标记 `responsibility_mutation_allowed` 与 pending Submission。conversation-only member 不挂载 Execution MCP；conversation-only coordinator 即使保留固定 MCP registry，也只允许 `get_execution` / `plan_execution`，其他 mutation 由 round-scoped CoordinationBinding 在服务端拒绝。只有进入 coordination 的当前 transient Execution coordinator 才能获得 `execution_transition.replace_current_allowed` / `abandon_allowed`；Plan Mode 用 `validation_only=true` 强制 write-free proposal。Goal-bound coordinator 仍可拥有 `plan_execution` 做普通 replan，但 replacement/abandonment affordance 为 false 并稳定返回 `goal_retarget_required`；Room member 与 subagent 两者均为 false。配置关闭或存在 Goal 冲突时不暴露 promotion。该列表只约束 Execution Orchestration 控制动作，不限制研究、编码和浏览等普通 conversation/task 工具。即使 runtime 不能动态隐藏工具，服务端也必须按 round binding、目标 Work Item/current Spec 以同一权限表拒绝。
8. **把下一动作所需语义放在动作旁。** `assigned_work`、`ready_work`、coordinator `active_assignments` 与 `pending_reviews` 投影 current Spec 的 logical key、objective、deliverable、acceptance criteria、`input_refs`、canonical `output_scopes` 与有序 `resolved_dependencies`；每条依赖给出 upstream Work Item/logical key/current Spec 与 kind，只有已经 Accepted 的 upstream 才携带 immutable Submission summary/refs/evidence 和 Acceptance criteria results，未验收依赖只暴露 status/blocker。structured Room WorkBinding 保留自身 mutation capability 以及这些直接上游的只读 WorkItem/Spec/PlanItem/dependency/output claim/accepted delivery 投影，继续过滤 sibling live Assignment/Attempt/Dispatch/state 和任何 unreviewed payload。若上次 Submission 被 rejected / changes requested，还必须携带 latest review decision、feedback 与逐条 criteria results。`resume_work` 记录的 resolution/evidence 同时进入后续 `ready_work` 和新 owner 的 `assigned_work`，避免模型依赖聊天历史猜返工要求或刚补齐的外部输入。任何带 `truncated=true` 的异常历史投影都表示上下文不完整，模型不得据此提交、验收或宣称 completion，应等待后端修复或改走 fail-closed 恢复。
9. **人类可读名不是关联键。** 模型可以看到 `logical_key=W2` 和 subject，但 Assignment、Attempt、Dispatch 依赖服务端 ID；不得靠显示名、自然语言相似度或 `@` 文本猜 binding。
10. **结构化路径是主路径。** Room 分工由 `assign_work` 创建 Assignment + Dispatch；子智能体由 pending Attempt + `PreToolUse: Agent` permit 绑定。自然语言 `@` 与 SDK Task 只做兼容/投影，不承担正确性。
11. **完成应由审计触发。** 接受 terminal Work Item 后服务端自动重算 Execution readiness；Stop/Goal complete 只消费 blocker audit。模型不需要手工同步一串“已完成”状态。

因此，模型面对的决策路径应始终接近：

```text
直接做一个原子交付
OR plan_execution → assign_work → submit_work → review_work
                                    └→ block/takeover when needed

用户明确换成 different transient objective
→ plan_execution(replace_current_execution=true, complete successor graph)
→ backend atomically supersedes old Execution
  and creates successor Execution + active Plan

用户明确放弃 transient objective 且没有 successor
→ abandon_execution
→ backend cancels old graph and creates nothing

required work 必须跨 boundary
→ propose promotion
→ backend policy decides
→ resume the same Execution
```

## 10. Tool 协议

Execution Orchestration 提供产品级 MCP 工具：

### 10.1 `get_execution`

读取当前 Execution、Plan、actor Assignment、Ready Work Item 和 completion blockers。

只读；模型在恢复、compact 后或状态冲突时使用。

### 10.2 `plan_execution`

批量创建或修订 Plan 和 Work Item；带明确 replacement fields 时原子创建不同 objective 的 successor Execution + active Plan。

必须：

- 提供完整 dependency edges。
- 提供 deliverable 和 acceptance criteria。
- 每个 `produce` Work Item 至少提供一个 output scope。
- 标出 required 与 terminal integrate/verify Work Item。
- 提交 base revision，防止覆盖并发修改。

runtime Plan Mode 下，`plan_execution` 只做完整 normalize/validate 并返回 proposal 结果，不 mint Plan/Work Item/Spec ID、不写 SQL、不创建 Execution，也不启动任何工作；离开 Plan Mode 后重交同一完整 draft，才创建 Execution 并激活 immutable Plan revision。SQL 中的 `proposed` 状态保留给未来由用户或控制面显式保存的草案，不由当前模型工具暗中持久化。
任何文字、Spec 或 dependency 修改都产生新的 immutable Plan revision；不得原地改写历史 revision。

模型输入使用本次 Plan 内唯一的 `logical_key` 表达 dependency；服务端生成 opaque Plan/WorkItem/Spec ID。一次调用必须原子成功或整体拒绝，不能留下半张工作图。

模型可见 schema 不再直接暴露容易被部分 Provider 丢失的 `array<object>` 参数，而要求把完整 WorkGraph 编码为 `work_graph_json` JSON array string。每项至少包含 `logical_key`、`kind`、`subject`、`objective`、`deliverable`、非空 `acceptance_criteria`、`required` 和 `terminal`；produce 项还必须给 typed output scope，整张图必须包含 required terminal integrate/verify 项。adapter 使用 `DisallowUnknownFields` 严格解码字符串，service 继续执行 32 项上限、DAG、terminal 与 output-scope 校验，因此 transport 兼容不会弱化领域契约或原子性。旧 `items` 仅保留为 decoder-only 的进程内兼容输入，不再进入模型 schema。

默认情况下，current Assignment、live Attempt 或待投递 Dispatch 会阻止 ordinary Plan revision replacement。若同一 objective 的旧图已经不再有效，coordinator 必须显式提交 `supersede_active_work=true` 和非空 `revision_reason`；同一事务先捕获每个 live/pending Attempt 的 Cancellation dispatch，再释放旧 Assignment、终结 Attempt、取消 pending/claimed Assignment/Review Dispatch，最后激活新 revision。任何 unreviewed Submission 都继续阻止这种 replan，不能借改 Plan 擦除已交付但尚未验收的结果。与当前 active Plan 规范化后完全一致的 draft 返回 semantic no-op，即使 retry 使用了新的 tool-use ID 也不产生空 revision。

普通首次 planning 在同一 SQL transaction 创建 Execution、完整 WorkGraph 与 active Plan；若任一步失败则整体拒绝，不允许先创建无 Plan 的 transient Execution。

当用户明确把当前 transient objective 替换为不同 objective 时，coordinator 在同一 `plan_execution` 输入中额外提交 `replace_current_execution=true`、非空 `replacement_reason`、新 objective、至少一条非空顶层 completion criterion 和完整 successor WorkGraph；`supersede_active_work` / `revision_reason` 与 replacement fields 互斥。模型必须把语义上相同的 outcome/completion boundary 路由到普通 replan，不能靠改写措辞伪造新 objective；服务端至少拒绝归一化后相同的 objective，并返回普通 replan 作为 next action，不依赖 LLM 相似度猜测。Room member、subagent 与非 coordinator 调用必须拒绝；当前 Execution 绑定 Goal 时返回 `goal_retarget_required`，不能静默解绑或改写 Goal。

Execution replacement 是一个 SQL transaction：CAS current revision，先为旧图每个 pending/running Attempt 写入 exact-target Cancellation dispatch，再 supersede 旧 Execution/active Plan，收束旧 Work Item、Assignment、Attempt 与未送达 Assignment/Review Dispatch，创建带 typed predecessor `replaces_execution_id` 的 fresh successor Execution、完整 WorkGraph 与 active Plan，并追加旧 `execution_superseded` 和新 `execution_created` / `plan_revised` 事件。旧 Execution 的 supersede event payload 记录 `successor_execution_id`，反向关系由 successor 的 typed predecessor 查询；同一 `(owner_user_id, session_key)` 的 current 唯一约束始终成立，不能先 terminalize 后依赖第二个命令补 Plan。

旧 Plan、Attempt、Submission、Acceptance 和 event 保持 immutable history。新 Execution 不继承 Work Item/Assignment，旧 Accepted 结果不计入 successor completion；当前协议没有 automatic carry，未来复用必须由独立显式 import/rebase 重新验证 objective fence、input 与 `spec_hash`。用户明确 replacement 时未审 Submission 留在旧历史，不进入 successor，也不能被原地改写。

Room handoff/queue/slot 属于 workspace ledger，不能和 SQL replacement 宣称一个跨存储 transaction。SQL commit 后由幂等 reconciliation 清理投影；在此之前，旧 Execution terminal status、superseded responsibility chain 与完整 WorkBinding admission fence 必须拒绝迟到 wake、queued slot、child launch 或 mutation result。

Plan Mode 对普通首次 Plan、replan 和 replacement draft 都只 normalize/validate；包括 replacement flag 在内的输入不得 terminalize 当前 Execution、写新 Execution/Plan、取消 Dispatch 或触发 Room reconciliation。

### 10.3 `assign_work`

把 Ready Work Item 分配给当前 Agent 或 Room member；当前 Agent 可以随后选择自己执行或在该 Assignment 下启动 subagent Attempt。

前置条件：

- Work Item 为 Ready。
- dependency 全部 Accepted。
- 没有 current Assignment。
- output scope 没有冲突。
- 调用者有 coordinator 权限；`self` 只用于单 Agent/DM Execution，Room 暂时要求 `room_member` owner。

Room member Assignment 在同一事务创建 Assignment 与 Dispatch outbox；DM self Assignment 与 Room member Assignment 都创建 pending root Agent Attempt。当前 Agent 之后选择 subagent 时，`PreToolUse: Agent` 在父 Attempt 上原子创建并绑定 running child Attempt。事务提交前不得发送 Room wake 或启动子智能体。

结构化 Room Assignment 成功后，由 outbox 自动投递；模型不需要再手写 `@member` 才让分工生效。

### 10.4 自动 Attempt 激活

不向模型暴露 `start_work`、`mark_running` 或 `complete_attempt`。普通 self work 在首个相关执行动作时由 runtime 自动 claim；Room root Attempt 只有在 target slot 的 runtime query 真正被接受后才从 pending 原子变为 running，单纯排队、handoff admission 或 SQL outbox delivery 都不能伪造“已开始”；子智能体由 `PreToolUse: Agent` 创建后端持有的 launch binding，`SubagentStart/Stop` 只校验或补充实际 runtime evidence。

### 10.5 `submit_work`

提交结果、引用和证据。成功后 Work Item 进入 submitted，Assignment 等待 reviewer。

### 10.6 `review_work`

有权限的 reviewer 接受或退回 Submission。

- accept：Work Item 进入 accepted，解锁 dependent。
- reject：记录 feedback；当前 owner 通过新的 Attempt/Submission 返工，或由 coordinator 显式 takeover，不覆盖历史尝试。

### 10.7 `block_work`

记录确定的阻塞原因、需要的输入和可恢复条件。不能用 blocked 代替执行失败或模糊不确定。

`block_work` 会在同一事务先为当前 Assignment 的 pending/running Attempt 写入 Cancellation dispatch，再收束未完成执行链；旧 Attempt 不再可复活。

若 current Spec 已有 unreviewed Submission，`block_work` 必须在服务层与 Repository 事务层同时拒绝；责任链保持 active，先由 `review_work` 产生 Acceptance。

### 10.8 `resume_work`

只有 current spec 处于 `waiting_input` 且原阻塞事实已经解决时才能重新开放 Work Item。调用者提供 resolution 与至少一项 evidence；coordinator 或该 current spec 的最新责任 owner 可以调用。`resume_work` 只执行 `waiting_input → open`，不创建 Assignment、不恢复旧 Attempt，随后必须按当前 Plan 重新 `assign_work`。

### 10.9 `take_over_work`

在同一事务先为旧 Assignment 的 pending/running Attempt 写入 Cancellation dispatch，再关闭旧 Assignment/Attempt，并以 unavailable、failed、blocked、urgent 或 user_directed 原因创建新 Assignment。新 Assignment/Attempt 使用新 identity，旧 cancellation target 不得指向它。

若 current Spec 已有 unreviewed Submission，`take_over_work` 必须在服务层与 Repository 事务层同时拒绝，不能释放 Submission 所依赖的 Assignment；先完成 review，之后才能按 Acceptance 结果决定返工或重新分配。

### 10.10 `promote_execution_to_goal`

请求把当前 transient Execution 绑定到新 Goal。工具调用只是 proposal；服务端必须重新执行 adaptive hard gate + durable signal 判定。

模型只提供可选 objective proposal 与一个当前上下文已经列出的 activation reason。adapter 注入当前 Execution identity/revision，后端从权威 snapshot 读取 objective、completion criteria、Plan 和 durable signal，并再次检查配置开关、当前 Goal 冲突与用户 authority。proposal 不能扩展 objective，也不能创建第二份 Plan。

### 10.11 `abandon_execution`

当用户明确放弃当前 transient objective 且不提供 successor 时，coordinator 用 `abandon_execution(reason)` 取消旧 graph。adapter 从 actor context 注入 current Execution identity/revision 和 idempotency command ID；模型不提供 Plan、Work Item、Assignment、Attempt 或 Dispatch identity。

服务在一个 SQL transaction 内 CAS current revision，先为每个 pending/running Attempt 写入 exact-target Cancellation dispatch，再把 Execution 标为 `cancelled`，收束 active Plan、未完成 Work Item、current Assignment、live Attempt 和未送达 Assignment/Review Dispatch，并追加 `execution_cancelled` event。它不创建 successor Execution 或 Plan。旧 Submission、Acceptance 与 event 保持 immutable history；取消不是伪造 completion。

只有当前 transient Execution 的 coordinator 可以调用。Room member、subagent 与非 coordinator 拒绝；Goal-bound Execution 返回 typed rejection `goal_retarget_required` 和 Goal lifecycle/objective retarget next action，不能静默解绑 Goal。

Plan Mode 只校验非空 reason、角色与 current/Goal binding proposal，不写 SQL、不取消 Dispatch，也不触发 Room reconciliation。SQL commit 后的 Room workspace ledger 仍按幂等 reconciliation 收敛；在收敛前，cancelled Execution 和完整 WorkBinding admission fence 必须拒绝迟到 wake、queued slot、child launch 或 mutation result。

### 10.12 统一 mutation 结果

所有模型可见 mutation 使用同一结果 envelope：

```json
{
  "outcome": "applied | no_op | rejected",
  "reason_code": "dependency_not_accepted",
  "execution_id": "execution-1",
  "snapshot_revision": 18,
  "changed": ["assignment-2"],
  "next_actions": [
    {
      "tool": "review_work",
      "work_item_id": "work-1",
      "reason": "accept the required upstream submission first"
    }
  ]
}
```

`next_actions` 是有序建议，不绕过工具权限或服务端状态机。底层 raw SQL、Go error 与内部 stack 不暴露给模型。

## 11. SDK Task/Todo 适配

SDK Task/Todo 继续用于：

- 模型熟悉的局部计划工具体验。
- 当前 Agent 的进度展示。
- subagent 内部临时步骤。

产品边界：

- shared/Goal-bound Work Item 必须先存在于 Nexus WorkGraph。
- TaskCreated/TaskCompleted Hook 可以把 Nexus metadata 绑定到对应 Work Item。
- Task completion 不能越过 Nexus Acceptance。
- owner、claim、Room collaboration 和 handoff 语义属于 Nexus，不写回 SDK core 作为第二套 team protocol。
- 无 Nexus work item metadata 的 Task 是 runtime-local task，不参与 shared Goal completion。

## 12. Room handoff 绑定

Room 通信协议保持 transport-only。不同 envelope 的 schema 必须分开，下面不是字段并集：

```text
WorkBinding:
  execution_id
  plan_id
  work_item_id
  spec_id
  assignment_id
  attempt_id
  dispatch_id

ReviewBinding:
  execution_id
  plan_id
  work_item_id
  spec_id
  assignment_id
  submission_id
  review_dispatch_id
  target_agent_id

GoalContinuationBinding:
  goal_id
  goal_objective_revision
  execution_id
```

`handoff_id` / `queue_item_id` 是 transport receipt identity，不属于 WorkBinding 或 ReviewBinding。Work/Review payload 禁止出现 `goal_id` / `goal_objective_revision`；若 exact Execution 属于 Goal，消费端从 SQL Execution 解析并校验。只有 internal Goal continuation envelope 携带 Goal identity。

Goal continuation 的三个字段是不可拆分的 exact capability：调度器在 claim continuation plan 之前要求全部非空，并在 runtime launch 前按 `execution_id` 读取 SQL Execution，重新校验同一 Goal ID、objective revision 与 coordinator。DM 和 Room 使用同一约束；ambient Goal、仅 ID/revision、仅当前 session 或旧 Execution 都不能启动 continuation。

### 12.1 结构化分配

推荐流程：

1. Lead 调用 `assign_work`。
2. 一个 SQL 事务创建 Assignment、queued Attempt/Dispatch outbox 和 append-only event。
3. 事务提交后，幂等 outbox consumer 先用 lease/CAS claim Dispatch，再按显式 `target_agent_id` 写入带完整 binding 的 Room handoff ledger 和 durable target input queue；正文和 `@` 都不参与目标解析。
4. 只有 durable Room receipt/queue 已落盘后，SQL Dispatch 才确认 delivered 并保存 receipt。Room queue/slot 携带 `execution_id/plan_id/work_item_id/spec_id/assignment_id/attempt_id/dispatch_id` 的完整 `work_binding`；进程在 started 后崩溃时，带 binding 的 handoff 仍可重开并幂等恢复。legacy 无 binding 的 public handoff 继续服务聊天 `@`，但不是受管分工真相源。
5. Assignment 写库前先校验 Room membership；slot admission 再校验 owner、current Assignment、Attempt、Dispatch 与 Plan/Spec fence。数据库、workspace 或 runtime 暂时不可用属于 transient failure，释放 lease 并按同一 outbox row 退避重试；Execution/Plan/Spec/root Attempt 已永久失效、target admission 已权威拒绝或 durable identity 冲突属于 permanent failure，当前 claim 原子转为 `cancelled` 并保存 reason，不能无限重试旧责任。
6. target slot 幂等 claim 已存在的 Attempt，而不是另外猜测或创建一份工作。

### 12.2 裸 `@` 兼容

Agent final reply 中的裸 `@member` 仍按 Room 协议解析。

- 空格不是 mention 正确性的条件：服务端按成员目录最长别名匹配，支持 `请@Researcher继续`、`@研究员请继续` 和相邻已知 mention；ASCII 标识符后缀、转义文本、代码、URL 与 email 不触发。
- 无论 Room 是否同时存在 active shared Execution，一个或多个裸 `@` 都只用于多人对话、brainstorm、投票或不追踪的一次性请求，并按 conversation-only wake 投递；它不携带 WorkBinding/ReviewBinding，不创建或激活 Work Item/Assignment/dependency/Acceptance，不形成 completion/Goal evidence。参与人数本身绝不触发 Plan。
- 如果这些输出预期成为受追踪 deliverable、解锁 dependency、进入最终 Acceptance 或推进 Goal，coordinator 必须先创建 Plan，再用 `assign_work` 建立结构化责任；不能事后把 casual 回复追认为 Acceptance。
- 裸 `@` 目标即使已经持有 Assignment，也只收到 conversation round；Assignment 的 work wake 只能来自 durable structured Dispatch，不能靠 mention 补发或重放。
- source round 的 WorkBinding/ReviewBinding 不沿 public reply、directed message 或 mention 传播。普通 handoff ledger row 的 binding 必须为空。
- 多个 `@` 不要求 WorkGraph 独立性；它们只是多路对话。只有 coordinator 建立多个 Assignment 时，后端才校验 Ready、dependency、Spec 和 output scope 是否允许并行。
- 如果 mention 无法解析或目标不可用，按 Room transport 规则处理；不能借 Execution 状态把普通聊天解释成工作拒绝原因。

B 依赖 A 时，B 的结构化 Dispatch 只能在 A Accepted 后创建或投递；任何写着“让 A、B 同时做”的对话消息都不改变该状态机。

### 12.3 Submission 与回交

Room member 调用 `submit_work` 时，一个 SQL transaction 同时写入 immutable Submission、`ExecutionReviewDispatch` outbox 与 append-only event。review target 来自 Assignment 创建时已经规范化的 `return_to_agent_id`；当前 Room reviewer 协议要求它是 coordinator，未来若引入独立 reviewer，也必须先扩展持久身份与权限链。

事务提交后，独立 consumer 使用 lease/CAS claim、retry 和 receipt，把待审 Submission 幂等写入 target 的 durable Room handoff 与 input queue。该回交携带 `execution_id/plan_id/work_item_id/spec_id/assignment_id/submission_id/review_dispatch_id/target_agent_id` 的完整 `review_binding`；它不携带 worker Attempt，也不伪造 reviewer Assignment/Attempt。

coordinator slot 只有在服务端重新校验 Execution、active Plan、current Assignment、未审 Submission、review target 与 outbox binding 后才会启动。其 actor context 直接投影 `pending_reviews` 并开放 `review_work`。因此正确性不依赖 worker 在正文中手写 `@Coordinator`；公开 deliverable 或 mention 仍可用于人类可见沟通和 legacy transport，但既不创建 review truth，也不产生 Acceptance。

SQL transaction 的原子边界止于 Submission 与 review-return outbox。Room workspace ledger 是另一持久化域，只能在 commit 后幂等投递；SQL outbox 是待投递真相源，稳定 dedupe key 防止重试生成第二条 handoff/queue。Execution terminal、Plan/Assignment replaced 或 Submission 已被 review 时，pending/claimed/failed return 被取消；已投递但迟到的 Room queue 在实际 wake 前也必须重新 admission 并拒绝，不能进入 successor Execution。

### 12.4 物理中断 outbox

Execution/Plan/Work Item/Assignment/Attempt 的 terminal fence 属于 SQL 状态机；Room slot 与 SDK runtime 的真实停止属于 commit 后副作用。所有主动要求把 `pending | running` Attempt 写成 `interrupted | cancelled` 的控制路径——包括 Plan `supersede_active_work`、`block_work`、`take_over_work`、transient replacement、abandonment 与 Goal revision supersede——必须调用同一个 capture/enqueue 边界。物理 runtime 已经结束后由 slot/subagent callback 回写 terminal evidence 不属于新的 cancellation request：

1. 在状态更新前枚举本次 scope 中每个仍 live 的逻辑 Attempt。
2. 在同一个 orchestration SQL transaction 写入唯一 `ExecutionCancellationDispatch`，保存 `execution_id/plan_id/work_item_id/spec_id/assignment_id/attempt_id`、executor、reason 与不可变物理 target。
3. SQL terminal mutation 随即提交，因此任何迟到 mutation、wake 或 successor admission 立刻被当前状态与 binding fence 拒绝；物理 runtime 是否已经响应不影响逻辑正确性。
4. app lifecycle consumer 使用 lease/CAS claim、过期 lease recovery、retry/backoff 和稳定 receipt 处理 outbox，重启后继续未完成投递；每次恢复/tick 先 drain cancellation，再投递新的 Assignment/Review，避免 successor 抢先占用同一 runtime session 并把原本可安全执行的 provider interrupt 降级为 local cancel。

Room root target 必须保存 exact `scope_session_id + room_id/conversation_id + target_agent_id + runtime_session_key + runtime_round_id/root_round_id/agent_round_id + dispatch_id`，consumer 重新验证完整 WorkBinding 后才进入该 slot 的 runtime cancellation。DM target 必须使用 exact `runtime_session_key + runtime_round_id`。本地 bridge 的 Query/Receive context cancellation 只结束 Nexus round，不会自动向 provider 发 interrupt，因此 runtime manager 必须区分两种结果：只有 exact target 仍是该 runtime session 的唯一 running round 时，才在阻止并发 successor admission 的 fence 内安全调用 session/provider interrupt，并记录 `provider_interrupted`；若同 session 已有其他 running round，只能调用旧 round 自己的 local cancel，记录 `local_round_cancelled` 与 typed limitation，绝不能用 session-wide interrupt 误伤另一个 round。Room slot 与 DM 使用同一规则；典型的 sole Room slot 可以得到 provider interrupt，而 DM/Room multi-round overlap 只能得到 local outcome。

dispatch 延迟到同 session 的 successor 已经运行时，old round 缺失返回 `already_ended`，identity 不匹配返回 `stale_target`，都视为幂等完成，绝不能取消 successor。若 provider interrupt 返回错误但 exact local cancel 可用，则保存 `local_round_cancelled/provider_interrupt_failed`；两种能力都不可用时保存 `unsupported`，不能把 Nexus context 退出伪报为 provider 已停止。

subagent child 仍以 child Attempt 作为逻辑被收束对象，但当前物理中断能力通过 parent root round 承载：dispatch 保存 child session/task/tool identity，同时把 `runtime_attempt_id`、runtime/Room round 与 WorkBinding 指向 root Attempt。每个 child 和 root 都可以有自己的幂等 outbox row；对同一物理 round 的重复中断在 consumer 端必须安全收敛。

尚未开始的 pending Attempt 明确收束为 `not_required/not_started`，不调用 runtime；缺少足够 exact identity 的 running Attempt 明确收束为 `unsupported` 并保存 typed limitation，不能调用模糊 target，也不能声称已经停止。consumer failure 保持 retryable；`provider_interrupted | local_round_cancelled | already_ended | stale_target | not_started | unsupported` 都必须保留可审计 outcome。

## 13. Hook 与服务端强制边界

Hook 只处理 runtime 边界；最终合法性由 Orchestration Service 在事务内判断。

| Hook / 边界 | 行为 |
| --- | --- |
| SessionStart | 注入 actor 当前 Execution Context |
| UserPromptSubmit | 重新读取 revision 与 allowed actions |
| PreToolUse: Agent | 要求绑定 Ready Assignment/pending Attempt，拒绝重复或 stale work |
| PreToolUse: execution mutation | 先做 authority、dependency、scope、version 检查 |
| TaskCreated | 绑定或投影 Nexus Work Item metadata |
| TaskCompleted | 在 SDK 状态写入前检查 Nexus submission/acceptance 规则 |
| SubagentStart | 只按 PreToolUse 冻结的 immutable parent binding 原子激活 Attempt并记录 runtime identity；歧义 fail closed |
| SubagentStop | 按同一 immutable binding 结束 child Attempt并把结果证据交还父 Agent；不自动创建 Submission |
| PostToolUse | 注入状态变化后的有界 Context |
| PostToolUseFailure | 记录 Attempt/command failure evidence，不自动 block Goal |
| SessionStart(source=compact) | 从 SQL snapshot 重新注入 Assignment、禁止重复执行范围和 blockers |
| Stop | 检查未完成 required work、running Attempt 和 submitted-unaccepted work |
| Room handoff admission | 检查 Assignment binding、依赖、revision、幂等 claim |
| Cancellation outbox consumer | lease/retry 绑定旧 Room slot/runtime round；仅当它仍是 session 唯一 running round 时发 provider interrupt，否则只做 exact local cancel 并记录 limitation |
| Goal completion service | 检查 WorkGraph、runtime quietness 和 usage settlement |

不能依靠 PreToolUse 检查普通 public `@`，因为 `@` 在 final assistant message 持久化后才解析；必须在 Room handoff admission 服务层检查。
`SubagentStart` 本身不能阻止一个违规启动；硬门禁必须位于 `PreToolUse: Agent`，`SubagentStart` 只负责对已获准的 Attempt 记录实际 runtime identity。

Hook 只拒绝确定性违规：

- dependency 未 Accepted
- wrong owner / wrong reviewer
- duplicate current Assignment
- duplicate running Attempt
- output scope 冲突
- stale plan/goal revision
- incomplete required work

“这个工作是否值得委派”仍由模型按 Prompt 判断；服务只要求结构完整和状态合法。

## 14. 单 Agent 执行流程

### 14.1 原子任务

1. 动态上下文明确当前为 `unmanaged`，当前 Agent 仍保留本轮用户请求的直接责任。
2. 当前 Agent 在同一个 execution boundary 内直接执行并回复。
3. 不创建 Goal、Plan、Work Item、Assignment 或 Attempt，也不要求模型同步机器进度。
4. 普通回复完成本轮；它不产生 shared/Goal-bound Acceptance 证据。

### 14.2 多步但单轮可完成

1. 创建 transient Execution。
2. 创建 Plan revision 和 Work Item DAG。
3. 当前 Agent依次执行 Ready Work Item。
4. 可把独立 Work Item 交给 subagent Attempt。
5. 父 Agent验收结果并完成 terminal integration/verify。
6. Execution completed；不自动保留 Goal。

### 14.3 自适应 Goal

1. 先按 transient Execution 工作。
2. 预测或观察到需要跨 boundary。
3. 原子执行 `promote_execution_to_goal`。
4. 保留 Plan 与所有状态。
5. continuation 恢复相同 Execution 或创建显式 recovery Execution。
6. required Work Item 与 completion audit 全部通过后 Goal complete。

## 15. Room 执行流程

以 `Researcher → Analyst → Lead` 为例：

```text
W1 Researcher: 收集官方规格与证据
W2 Analyst: 基于 W1 Accepted 结果完成差异分析
    depends_on: W1
W3 Analyst: 整合并验证最终报告草稿
    depends_on: W2
```

流程：

1. Lead 建立 shared Plan。
2. W1 为 Ready，W2/W3 为 Blocked。
3. Lead 分配并只激活 W1。
4. Lead 不执行 W1 output scope。
5. Researcher 提交 W1。
6. Lead Review；接受后 W2 变 Ready。
7. Lead 激活 Analyst 的 W2。
8. Analyst 提交，Lead 接受。
9. W3 变 Ready，由 Analyst 执行并提交，Lead 不重复执行同一 output scope。
10. Lead 验收 terminal integration/verification，并基于 Accepted 结果完成最终回复。
11. Execution/Goal completion audit。

Lead 在第一条消息同时 `@Researcher` 和 `@Analyst` 作为普通聊天完全合法；它只产生两个 conversation round。若 Lead 试图把这条裸 mention 当成正式并行派工，则该派工无效：裸 `@` 没有创建任何 Assignment，而且 W2 尚未 Ready，只有 W1 的 structured Dispatch 可以启动受管工作。

## 16. 失败、重试与接管

### 16.1 Attempt 失败

- 记录 failure reason 和 evidence。
- Work Item 不自动 Accepted。
- coordinator 可以 retry 同一 Assignment 或 release 后重分配。
- 重试创建新 Attempt，不能覆盖历史 Attempt。

### 16.2 Agent 不可用

- Assignment 进入 released/cancelled。
- 记录 unavailable takeover reason。
- 新 Assignment 获得新的 owner。
- 旧 owner 后续迟到结果只能成为 orphan Submission，不得覆盖 current Assignment。

### 16.2.1 Room Dispatch 永久失败

- stale/terminal Execution、superseded/stale Plan/Spec，以及已经被其他控制路径释放或替换的 Assignment 属于 **stale graph permanent failure**：只把旧 Dispatch 置为 `cancelled`。旧 graph 已由 replacement/replan/cancellation transaction 收束，consumer 不推进 Execution、不恢复旧 Assignment、不把旧 Work Item重新置 Ready。
- current active Plan/Spec 下的 target admission 永久拒绝、durable identity 冲突，或尚未启动时发现缺失 root Attempt，属于 **current responsibility permanent failure**：仅当 exact Assignment 仍为 `assigned`、Work Item仍为 current open spec 且没有 running Attempt 时，才在同一 SQL transaction 取消 Dispatch、终结存在的 pending root Attempt、释放 Assignment、推进 Execution version并写审计事件，使当前 Work Item重新成为 Ready。只取消 outbox row会留下幽灵责任。
- 数据库、workspace 或 runtime 暂时不可用属于 transient failure，释放 lease 并重试同一 Dispatch。
- 若 Attempt 已经 running，Dispatch consumer 不得借“投递失败”中断真实工作；它只终结 stale outbox，物理执行由 exact cancellation protocol 处理。
- 同步 `assign_work` 路径在 drain 后刷新 snapshot，因此当前 coordinator round立即看到 released/Ready。后台恢复器只写 durable event 和可恢复状态：Goal continuation 或下一次 coordinator/user wake 读取它；transient conversation 不因内部 outbox 失败凭空制造一条新的模型消息。

### 16.3 用户改变 objective

必须先区分四种语义，不能把它们都实现为“改 Plan”：

1. **同一 objective 改路线。** outcome 与 completion boundary 不变，只改变步骤、文案、owner、dependency、工具或 evidence strategy，继续使用当前 Execution，并通过 `plan_execution` 创建 immutable Plan revision。必要时显式 `supersede_active_work`；不得改写 Execution objective。
2. **Goal-bound objective revision。** 必须执行 6.5 的 durable rebase：先 prepare 并预留 successor，terminalize 整个旧 WorkGraph并在同一 orchestration transaction 写入 exact-target Cancellation outbox，再 commit Goal revision，最后由 `plan_execution` 原子创建 fresh successor graph 并确认 binding。旧 live Attempt 由独立 consumer 可靠重试物理中断，旧 revision 的完成事件不能写入新 revision，任何 Work/Acceptance 都不自动 carry。`plan_execution.replace_current_execution` 与 `abandon_execution` 都返回 `goal_retarget_required`，不能把 Goal continuity 退化为 transient replacement/cancellation。
3. **不同 transient objective replacement。** 只有用户明确提供不同 non-Goal objective 时，coordinator 才能用 `plan_execution` 同时提交 `replace_current_execution=true`、`replacement_reason`、新 objective/completion criteria 和完整 successor graph。一个 SQL transaction supersede 旧 Execution 并创建 successor Execution + active Plan；成员和 subagent 禁止调用，Plan Mode 只验证不写入，旧 Accepted 结果不自动 carry。
4. **只放弃 transient objective。** 用户明确不再继续且没有 successor 时，coordinator 调用 `abandon_execution(reason)`；一个 SQL transaction cancel 旧 graph，不创建新 Execution/Plan。Goal-bound、成员和 subagent 请求均拒绝，Plan Mode 只验证。

Room handoff、durable input queue、slot 与 runtime 位于 SQL 事务之外，不与 replacement/abandonment 共享 transaction。SQL commit 后 workspace projection 通过 reconciliation 收敛，runtime 则通过 durable Cancellation outbox 收敛；任何 admission 都必须先读取 current Execution/Plan/Assignment/Attempt/Dispatch fence。旧 Execution terminal 后的迟到工作只能被拒绝或归入旧历史，旧 cancellation 也只能命中捕获时的 exact round，不能进入或中断 successor。

### 16.4 Compact、重启与恢复

- 使用 Execution、Assignment、Attempt 和 handoff 稳定 ID 恢复。
- runtime transcript 不是状态恢复真相源。
- 每个 mutation 带 idempotency key 或 version。
- 重复 Hook、重复 handoff wake 和重复 tool result 不产生第二次状态变化。
- resume 后 dynamic context 必须先显示 current Assignment，再允许模型调用 Agent 或 Room 工具。
- 带 WorkBinding/ReviewBinding 的 queue item 不允许 generic guide、delete、reorder、merge 或 delivery policy mutation；这些控制只适用于普通 conversation queue。历史坏 envelope 必须逐项 terminalize 后继续后续消息，不能整队阻塞或降级为聊天。

## 17. 持久化与事件

跨 session 的 Orchestration 状态使用关系数据库：

| 数据 | 真相源 |
| --- | --- |
| Execution | SQL |
| immutable Plan revision / Plan item / dependency | SQL |
| stable Work Item / immutable Work Item Spec | SQL |
| Assignment | SQL |
| Dispatch outbox | SQL |
| Attempt | SQL |
| Attempt cancellation outbox | SQL |
| immutable Submission / Review return outbox / append-only Acceptance | SQL |
| Orchestration event | append-only SQL event ledger |
| runtime Task/Todo | SDK/runtime local projection |
| Room handoff / queue | 现有 Room workspace ledger |

所有 mutable aggregate 使用 optimistic version；每个 command 都携带调用方稳定的 idempotency command ID。普通首次 Plan 必须在一个 SQL transaction 内创建 Execution、完整 WorkGraph 与 active Plan；Room `submit_work` 必须在一个 SQL transaction 内创建 immutable Submission 与 review-return outbox；任何把 pending/running Attempt 置为 interrupted 的 mutation 必须先在同一 transaction 写入 per-Attempt Cancellation outbox；transient replacement 必须在一个 SQL transaction 内 terminalize 旧 Execution 并创建 successor Execution + active Plan；Goal revision rebase 必须先以 durable Goal transition 保留意图和 reserved successor，再分别幂等提交 old graph supersede、Goal revision commit、successor Execution+Plan 与 Goal binding confirmation；abandonment 在一个 SQL transaction 内只取消旧 graph。Room workspace ledger 只在 commit 后幂等 reconciliation，runtime physical interrupt 只在 commit 后由 cancellation consumer 投递；正确性由 SQL outbox、terminal state 与 admission fence 保证。

数据库必须保证：

- 同一个 `(owner_user_id, session_key)` 最多一个 `active | waiting | paused` Execution；读取 current Execution 不依赖“最近一条”猜测。
- 一个 Execution 最多一个 active Plan；Execution 不保存重复 active-plan 指针。
- dependency 两端必须属于同一个 Plan，Assignment 必须绑定该 Plan 中的精确 Work Item/Spec。
- Attempt、Submission 与 Acceptance 必须沿同一 Assignment/Work Item 链。
- 一个 Work Item 最多一个 current Assignment；一个 Assignment 默认最多一个 pending/running Attempt。
- 每个被 orchestration transition 中断的 logical Attempt 最多一个 Cancellation dispatch；dispatch 必须保留完整逻辑 chain 和 immutable physical target，且写入先于同事务的 Attempt terminal update。
- 每个 Execution Event 具有单调 `(execution_id, sequence)`、entity identity/version 和唯一 command ID。
- 接受 Submission 与解锁下游、创建后续 Assignment/Dispatch 必须原子提交。
- replacement 必须原子写入旧 Execution terminal state、旧 `execution_superseded` event payload 中的 `successor_execution_id`、successor 的 typed predecessor `replaces_execution_id` 和 successor active Plan，不能出现两个 current Execution，也不能先终结旧 Execution 后依赖第二个命令创建 successor/Plan。
- Goal revision successor 必须匹配同 Goal 的连续 objective revision、完全相同的 owner/session/scope、`superseded` predecessor 及其 supersede event 中的 reserved successor ID；transition pending 时 Goal completion 与 continuation 均不可越过。
- abandonment 必须只产生 terminal old graph；重复 command 幂等返回同一结果，不能误建 successor。
- Cancellation dispatch 的过期 lease 必须可恢复，重复 delivery 必须幂等；old exact round 已结束或 binding stale 时收束为稳定 outcome。provider interrupt 只允许在 exact target 仍是 session 唯一 running round且 successor admission 已暂时 fenced 时调用；否则只能记录 local/unsupported，禁止命中 successor。

事件至少包括：

- execution_created
- execution_cancelled
- execution_superseded
- execution_promoted
- plan_revised
- work_item_ready
- work_assigned
- attempt_started
- attempt_reconciliation_scheduled
- attempt_terminal
- work_submitted
- work_accepted
- work_rejected
- work_taken_over
- execution_completed
- execution_failed

事件用于审计、恢复解释和未来 UI 投影，不替代 aggregate tables。

## 18. 后端模块边界

建议目录：

```text
internal/protocol/
  execution.go

internal/service/orchestration/
  doc.go
  service.go
  execution.go
  plan.go
  assignment.go
  attempt.go
  cancellation_dispatch.go
  review.go
  context.go
  goal_promotion.go
  errors.go

internal/storage/orchestration/
  doc.go
  repository.go
  scan.go
  cancellation_dispatch.go

internal/mcp/execution/
  contract/
  tool/
  server.go

internal/runtime/
  orchestration_hooks.go
  interrupt.go
```

依赖方向：

```text
app -> dm/room runtime -> orchestration service -> orchestration storage
                          |
                          +-> protocol

mcp execution -> narrow orchestration service contract
goal service -> narrow orchestration readiness interface
room realtime -> narrow handoff/attempt interface
```

- Orchestration 不反向依赖 DM、Room realtime 或 handler。
- Room realtime 仍拥有通信和 slot 生命周期，只调用 Orchestration 的窄接口。
- Goal service 仍拥有 Goal 生命周期，只委托 Orchestration 提供 execution readiness。
- MCP 注册保持薄装配，不承载业务状态机。

## 19. 分阶段交付

### 阶段一：语义与状态

- 固定本文协议。
- 增加 protocol、migration、repository 和 Orchestration Service。
- 实现 Plan、Work Item、Assignment、Attempt、Submission、Review 状态机。
- 实现首次 Execution + active Plan 创建、transient replacement 和 abandonment 的原子 SQL commands。
- 实现 immutable Spec/Plan membership、Acceptance、幂等 event ledger 与 Dispatch outbox。
- 实现所有 Attempt interruption path 共用的 same-transaction Cancellation outbox capture。
- 不依赖 Prompt 正确性证明状态约束。

### 阶段二：模型控制面

- 注入统一 stable execution contract。
- DM 与 Room 都使用 static/dynamic prompt 分层。
- 增加 `<nexus_execution_context>`。
- 增加 `execution_transition` flag-level affordance，区分普通 replan、replacement 与 abandonment。
- 增加 execution MCP 工具。
- SDK Task/Todo 只做投影适配。

### 阶段三：运行时闭环

- 绑定 subagent Attempt。
- 绑定 Room handoff 与 Assignment。
- 实现裸 `@` 准入。
- 实现 failure/retry/takeover/recovery。
- 用 SQL terminal state + WorkBinding admission fence 拒绝 replacement/abandonment 后的迟到 Room/runtime 工作，并幂等 reconciliation workspace ledger。
- 用 lease/retry/recovery consumer 精确中断旧 Room slot/DM runtime round，证明延迟 cancellation 不会命中 successor。
- 实现 adaptive Goal promotion。
- 用 WorkGraph 替换粗粒度 Room collaboration completion evidence。
- 接入 Goal completion readiness。

### 阶段四：UI 投影

不在本文当前交付范围。未来 UI 只消费 Execution/Plan/WorkItem/Assignment/Attempt 事件和查询，不定义新状态。

## 20. 必须通过的行为场景

1. 单个原子任务不创建多余 Plan 或 Goal。
2. 多步单 Agent 任务建立 Plan，但可在单轮完成而不保留 Goal。
3. 明显跨轮任务从 transient Execution 原子提升为 Goal。
4. compact 前后不重复创建 Plan、Assignment 或 subagent。
5. 两个独立 Work Item 可由不同 Room Agent 并行；同一 parent Agent 在当前 SDK correlation 限制下最多一个 active child subagent。
6. 有依赖的 Work Item 在上游 Accepted 前不能启动。
7. 相同 exclusive output scope 的 produce Work Item 不能并行。
8. 显式 review/verify 可以读取被审查 Work Item 的相同结果。
9. subagent stop 只终结 child Attempt；父 Agent 整合并显式 Submission，授权 reviewer 验收后才 Accepted。
10. Room Lead 分配后不重复执行成员的 produce scope。
11. 一条含两个 `@` 的消息只启动两个 conversation round，不启动 `Researcher → Analyst` 的任何 Assignment；依赖链只由 structured Dispatch 推进。
12. Room member 不能修改 sibling Work Item 或完成 shared Goal。
13. member Submission 被 Lead reject 后可以返工并保留历史 Attempt。
14. Agent failure 后 takeover 不产生两个 current Assignment。
15. 迟到的旧 owner Submission 不覆盖新 Assignment。
16. Goal objective revision 变化会拒绝 stale Plan/Attempt mutation。
17. Goal completion 在 required Work Item 未 Accepted 时被拒绝。
18. Goal completion 在 running subagent、pending handoff 或 usage 未结算时被拒绝。
19. 服务重启后 pending Assignment/handoff 可幂等恢复。
20. Plan Mode 只能验证 Proposed Plan、replacement 或 abandonment，不能写入、开始执行或自动续跑。
21. 无论是否存在 managed Execution，裸 `@` 都只做 conversation transport，不创建、不激活 Work Item/Assignment，也不继承 source binding。
22. 同一 command 或重复 Hook delivery 不产生第二个 Assignment、Attempt、Acceptance 或 event sequence。
23. dependency、Assignment、Attempt、Submission 不能跨 Execution、Plan 或 Spec chain 写入。
24. Submission 不可变，reject/accept 只能新增 Acceptance decision；reject 后返工产生新 Submission。
25. Plan 的任何文本、Spec 或 dependency 修改都产生新 revision，历史快照不可改写。
26. 自动 Goal 只有在全部 hard gate 和至少一个 durable persistence signal 成立时创建，复杂度本身不触发。
27. 自动 Goal 在用户禁用、Plan Mode、objective 模糊、需新授权或只剩 optional 后台工作时不创建。
28. `SessionStart(source=compact)` 恢复同一 SQL snapshot；`PostCompact` 不被当成持久状态注入。
29. 普通首次 `plan_execution` 原子创建 Execution、完整 WorkGraph 与 active Plan；失败时不留下 planless Execution。
30. 同一 objective 只改变路线时使用普通 `plan_execution` replan；设置 `replace_current_execution` 被拒绝，不创建 successor Execution。
31. coordinator 替换不同 transient objective 时，旧 Execution supersede 与 successor Execution + active Plan creation 在同一 SQL transaction 完成；旧 Accepted 结果不进入 successor。
32. `abandon_execution` 只取消当前 transient graph，不创建 successor；重复 command 幂等。
33. Goal-bound Execution 的 replacement/abandonment 返回 `goal_retarget_required`，Room member 与 subagent 调用返回 authority rejection。
34. Plan Mode 的 replacement/abandonment 只返回 validation proposal，SQL、Dispatch outbox 与 Room ledger 均不变化。
35. replacement/abandonment commit 后的迟到 Room handoff、queued slot、child launch 或 mutation result 被旧 Execution/WorkBinding fence 拒绝，即使 workspace reconciliation 尚未完成。
36. MCP、HTTP 与 app-server 的 managed Goal objective mutation 都进入同一个 retarget coordinator；Room 模型非 Lead 被拒绝，HTTP metadata 不能覆盖 server-owned binding/lead/transition。
37. Goal rebase 在 prepare、old graph supersede、Goal commit 或 binding confirmation 后失败时，使用同一 command 重试会复用同一 transition 与 successor，并只修复缺失阶段。
38. `prepared | awaiting_plan | binding_reserved` Goal 不能 complete 或自动 continuation；模型在 `prepared` 获得 `retarget_goal` retry，在后两阶段获得 `plan_execution` next action。
39. Goal revision successor 只有在 predecessor 已按同一 transition supersede、typed predecessor/revision/scope 连续且首 Plan 同事务创建时才接受；任意 terminal predecessor 或不同 reserved ID 被拒绝。
40. Goal revision successor 的 Plan Mode proposal 对 Goal metadata、Execution、Plan 与 outbox 零写入；旧 Work/Submission/Acceptance 不自动 carry。
41. Plan active-work supersede、block、takeover、transient replacement、abandonment 与 Goal supersede 在把 Attempt 写为 interrupted/cancelled 前，均在同一事务写入每个逻辑 Attempt 的 exact-target Cancellation dispatch。
42. cancellation consumer 在 delivery 失败或进程崩溃后可回收 lease 并重试；同一 row 不产生第二个逻辑 cancellation。
43. sole Room/DM runtime round 的 provider interrupt 保存 `provider_interrupted`；同 session 已有另一 running round时只取消 exact local context，保存 `local_round_cancelled` 与 limitation，不调用 shared provider/session interrupt。
44. provider interrupt 的唯一性检查与调用之间禁止 successor admission；延迟的 Room/DM cancellation 只命中捕获时的 WorkBinding/runtime round，旧 target 已结束或 stale 时幂等收束，不中断同 session、同 Agent 的 successor。
45. pending Attempt 不调用 runtime并落为 `not_required/not_started`；running Attempt 缺 exact identity 或 exact local/provider 能力时落为 `unsupported`，不伪报 physical interruption。
46. 同一 Room 同时存在 active Execution 时，无 binding 的用户定向消息、裸 `@` 和 directed message 仍可正常聊天；non-coordinator member 不获得 Execution MCP，直接 mutation 返回 `conversation_only`。
47. Room `review_work` 只接受 review-return outbox 注入的 exact ReviewBinding；普通 coordinator 聊天或 WorkBinding round 不能验收 Submission。
48. input queue/guidance 不改变 target round lane，普通消息链路不会复制 WorkBinding/ReviewBinding；Assignment 与 review 只能由各自 durable outbox 创建。
49. active Execution 存在时，普通 Room coordinator round 仍从 conversation 开始；跳过 `get_execution` / successful `plan_execution` 直接 `assign_work`、takeover、promotion 或 abandonment 返回 `conversation_only`。显式转换后同一物理 round 可协调，round 结束 capability 被释放。
50. internal Goal continuation 只有 exact Goal ID/revision/Execution ID 三元组与 SQL Execution/coordinator 全部匹配时直接进入 coordination；任一字段缺失会在 claim 前失败，ambient Goal、旧 revision、错误 Execution 和普通聊天不能获得 Goal mutation authority。
51. 旧 parent round 的迟到 SubagentStart/Stop 不会路由给 successor callbacks；复用 SDK Agent identity 或缺少唯一 correlation 时 fail closed。
52. Work Dispatch 命中 stale/terminal Execution、superseded Plan/Spec 或已释放责任时只取消旧 outbox，绝不 reopen old Work；只有 current active Plan/Spec 下、尚未启动且仍为 `assigned` 的永久 target/root Attempt failure 才同事务 release/cancel 并让当前 Work Item Ready；running Attempt不被 Dispatch consumer 中断，Room/runtime 暂时不可用仍 retry。
53. 带 WorkBinding/ReviewBinding 的 queue item 不能通过 generic queue control 改成 guidance、删除、重排或合并；每条 durable directed message 独立 dispatch。
54. `get_execution` 在没有 current Execution 时返回 unmanaged conversation context，不创建空 Execution；完整 `plan_execution` 成功后才进入 coordination。
55. Goal-bound Work/Review round 的 Goal ID/revision 只由后端从 exact Execution binding 解析，消息正文和 binding payload 不能伪造或采用 ambient revision。
56. 父 round结束时 runtime/service/storage 共同强校验并持久化精确 `T+30s` child grace deadline；迟到 Stop仍终结原 child Attempt，终态永久缺失时独立于 Room realtime 的启动/每秒恢复器跨进程将其置为 interrupted，后续 context投影 terminal `subagent_result` 而不自动 Submission/Acceptance。

## 21. 非目标

本文当前不规定：

- 前端步骤条、进程卡片、Room feed 或 Goal panel 的组件实现。
- 用 LLM 文本相似度自动猜测所有语义重复；模型必须声明 deliverable 和 output scope。
- 用 Execution Orchestration 替代 Room public/private 通信协议。
- 用 Goal 替代 Automation 的定时、重复或日历调度。
- 允许 Goal continuation 获得超出用户原始请求的新权限。
- 将子智能体提升为持久 Room 成员或独立用户关系主体。
