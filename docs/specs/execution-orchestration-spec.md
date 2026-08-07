# Execution Orchestration 协议

## 1. 文档目标

本文定义 Nexus 在单 Agent、子智能体和 Room 多 Agent 场景下统一的执行编排协议。它回答：

- Goal 何时存在、为何持续以及何时完成。
- Plan 如何把目标组织成有依赖的工作。
- Work Item、Assignment、Attempt、Submission 与 Acceptance 分别代表什么。
- 当前 Agent、子智能体、Room 成员和 Room Lead 各自拥有什么权限。
- 模型通过哪些稳定提示、动态上下文和工具观察、改变执行状态。
- 后端如何通过状态机、Hook 和 Room handoff 准入阻止重复、越权和错误完成。

本文同时规定前端投影边界：UI 只把本文的后端事实压缩成图标优先的工作图，不创建第二套执行语义。

相关协议：

- [Execution Graph 协议](./execution-graph-spec.md) 定义 WorkGraph 责任如何展开为 Agent、Subagent、Tool、Gate、Hook 事件和可恢复运行图。
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

### 3.2 硬约束只保护事实，不规定工作风格

后端代码只强制安全、身份与权限、幂等、不可变历史、状态一致性，以及 Agent 已经声明的真实依赖或资源冲突。下列选择属于 Agent 的执行策略，不是服务端准入条件：

- 是否为当前请求创建 Goal 或 Plan。
- 是否声明 terminal 节点、逐条 acceptance criteria 或 output scope。
- Ready 工作由 Lead 自己做、交给 Room 成员，还是在自己的 Assignment 内使用一个或多个 subagent。
- 由 owner、Lead 或另一个 Room member review；自审不自动成立，但可以通过独立的 Submission/Acceptance 事实显式完成。
- 是否并行、是否增加复核节点、是否根据 objective alignment 再运行一轮。

编排知识采用 progressive disclosure，而不是把整套流程重复塞进 Prompt 和每个 MCP 描述：

| 层级 | 只负责 |
| --- | --- |
| 常驻 Prompt | 任务优先、稳定语义、何时加载 Skill、context 权威性和连续执行边界 |
| `execution-orchestrator` / `goal-manager` 入口 | 当前问题应该读取哪一类策略，不复制全部细节 |
| Skill `references/` | 结构选择、图控制、通信连续性，以及 Goal 创建/完成/Room 生命周期的按需知识 |
| 动态 execution context | 当前 actor、lane、binding、revision、图切片、blocker 与 `allowed_actions` |
| MCP schema/description | 一次调用的原子作用、真实参数、状态前置条件和不可破坏的硬语义 |
| Bridge/Hook | Tool、Subagent 与 runtime lifecycle 的自动观测和准入事件 |

Prompt 与 Skill 只提供推荐和例子；动态 execution context 提供当前事实与可用能力。Agent 根据任务内容自行组合。代码不得因为“标准流程应该这样走”而拒绝一个在权限、引用和状态上合法的选择。MCP 描述不得再次收纳 Room 协作方法、用例路由、最终回复风格或完整编排教程；这些内容属于 Skill。Skill 也不得复制瞬时工具 schema、opaque identity 或版本字段。

能力选择使用正交信号而不是用例白名单：局部记忆压力对应 Task，上下文隔离价值对应 Subagent，独立责任和显式拓扑对应 Work Item/Plan，持久身份交接对应 Room Assignment，证据会改变路线时对应 Gate/Loop，跨 boundary 持续性风险对应 Goal。任一信号都只说明一种结构可能有价值，不自动要求其他结构；Agent 从直接执行开始，选择足以表达真实任务的最少结构。

### 3.3 Goal 是持续性边界，不是复杂度容器

复杂任务可能在一个 execution boundary 内完成，因此只需要 Plan。简单任务也可能因为等待、重试或跨轮恢复而需要 Goal。

Goal 的判定问题是：

> 当前 objective 是否必须在本轮执行、当前上下文、预算边界、外部等待或 Agent 交接之后继续存在？

### 3.4 Plan 是 Work Item 图，不是权限模式

- Execution Plan 是一个有 revision 的 Work Item 有向无环图。
- Plan Mode 是只允许检查和提出方案的 runtime permission mode。
- Plan Mode 可以通过 `prepare_plan_execution` 严格校验完整 Plan、transient replacement 或 replan，并把结果 seal 为 durable non-authoritative proposal；这类 proposal 不是 Execution/Plan/Goal 真相，不能激活 Assignment/Attempt 或触发 Goal continuation。
- `plan_execution` 是权威 materialization 边界，在 Plan Mode 必须拒绝；离开 Plan Mode 后可以直接提交此前返回的同一 `proposal_id + proposal_digest`，不要求模型重建文档。

### 3.5 委派转移执行，不转移最终责任

- 把 Work Item 分配给 Room 成员时，Lead 仍负责共享图能够收口和最终交付；具体 Submission 可由 Lead、owner 或另一个被选中的 Room member review。
- 当前 Agent 使用子智能体时，父 Agent 仍是 Assignment owner；子智能体只是该 Assignment 的 Attempt executor。
- worker 的结果是 Submission，不是 Acceptance。

### 3.6 并行由 Agent 选择，声明的冲突由后端保护

Agent 决定是否并行时应检查：

- 所有依赖已 Accepted。
- 输入在本次执行期间稳定。
- deliverable 不重合。
- exclusive output scope 不冲突。
- 不存在一项等待另一项结果的隐含依赖。

typed canonical output scope 是可选的精确冲突声明。Agent 认为重复交付风险值得保护时，可使用 `file:<workspace-relative-posix-path>`、`dir:<workspace-relative-posix-path>` 或 `semantic:<nonempty-key>`；一旦声明，后端严格执行重叠规则：file 只按同一路径重叠，dir 与自身及任意后代 file/dir 重叠，semantic 按 key 精确重叠，只有双方都声明 `shared` 才允许重叠。未声明 scope 不构成无效 Plan，也不能由后端凭自然语言猜出冲突。

需要独立复核时，Agent 可以增加 `review` / `verify` Work Item，或把 `return_to_agent_id` 指向另一名 Room Agent；不需要独立复核时可由 owner 或 Lead 显式自审。两个相同的 `produce` Work Item 不应被当成复核，因为它们表达的是两份生产责任。

### 3.7 Room/DM 是对话底座，Execution 是可选叠加层

系统不能把“当前 Room/session 存在 active Execution”解释成整个会话进入 managed mode。一个 Room 或 DM 在同一时间可以同时承载普通聊天、协调、受管工作、验收和后台 Goal continuation；Execution 只约束携带其可信 capability 的具体 round。

全局分为四个彼此正交的平面：

| 平面 | 真相 | 不负责 |
| --- | --- | --- |
| Conversation | Room/DM message、public/private handoff、`@`、用户定向消息与普通 input queue | Work Item ownership、依赖、Acceptance、Goal completion |
| Execution | Execution、Plan、Work Item、Assignment、Attempt、Submission、Acceptance 及其 outbox | 决定谁应参与闲聊、把消息正文解释成状态 |
| Persistence | Goal objective/revision、continuation、等待、恢复和 completion audit | 把复杂度或多人参与自动等同为 Goal |
| Runtime | 当前 round、slot、SDK session、subagent hook、interrupt 与 usage | 作为业务完成或责任真相源 |

因此 active Execution 是当前会话的后台事实，不是 Room 的全局开关；Goal 也是持续性 envelope，不是聊天模式。

### 3.8 每个 round 的 lane 只由可信 envelope 决定

模型每次被唤醒前，后端必须根据服务端注入的身份和 binding 选择 lane，不能从正文动词、`@` 数量、参与人数或模型自述猜测：

| lane | 可信依据 | 可做的事 |
| --- | --- | --- |
| conversation | Room round 没有 WorkBinding、ReviewBinding、exact Goal binding 或 round-scoped CoordinationBinding | 回应当前对话、使用普通任务工具；成员不能读取 WorkGraph，coordinator 只可显式 `get_execution`，或先 `prepare_plan_execution` 再 `plan_execution` 进入协调面 |
| coordination | exact Goal ID/revision，或 Execution coordinator 在当前物理 round 显式调用 `get_execution` / 成功 materialize `plan_execution` 后由后端 mint 的 CoordinationBinding | prepare/commit Plan 或 replan、分配、解阻、接管、审计完成；普通聊天本身仍不成为工作证据 |
| work | 完整 WorkBinding | 执行、阻塞、提交该 Assignment；不能修改 sibling。若该 owner 也是 Assignment 选定的 reviewer，可在同一可信 binding 内显式 review 自己的 Submission |
| review | 完整 ReviewBinding | 选定 reviewer 审查 binding 指定的 immutable Submission；是否由 Lead、owner 或另一成员承担取决于 Assignment |
| subagent | parent WorkBinding + server-created child Attempt binding | 只帮助父 Agent 完成同一 Work Item；stop 只终结 child Attempt |

稳定不变量：

- 裸 `@` 永远产生 conversation round；即使目标已有 Assignment、Room 有 active Execution，也不激活责任。
- `assign_work` 的 durable Dispatch outbox 是创建跨 Agent work lane 的唯一 Room 入口。只有 reviewer 与 owner 不同，`submit_work` 才创建 durable review-return outbox 和独立 review lane；显式自审继续使用当前 WorkBinding，不把结果重新投递给自己。
- `review_work` 成功写入目标 Submission 的唯一 Acceptance 后会消费已完成的 review lane。若 reviewer 同时是 coordinator，同一物理 round 可继续协调；若 reviewer 是另一成员，Acceptance 仍会可靠解锁图，协调者从持久状态和后续 wake 继续，不要求用户逐节点发送“继续”。
- 普通消息不复制 source round 的 WorkBinding/ReviewBinding。worker 在受管 round 中 `@` 另一个成员，只会建立新的 conversation round。
- non-coordinator Room member 没有 binding 时不挂载 Execution MCP，后端直接调用也返回 `conversation_only`；Prompt 不是唯一护栏。
- coordinator 身份使 Agent 有资格进入协调面，但不是 round-start 的隐式 CoordinationBinding。普通 coordinator Room round 仍从 conversation 开始，只开放 `get_execution`、`prepare_plan_execution` 与 `plan_execution`；读取或 seal proposal 都不建立协调 capability，只有 exact-fence materialization 成功后才建立或修订协调图。后端为成功转换 mint `(owner, session, agent, physical round, execution)` capability，其他 mutation 在 capability 缺失时返回 `conversation_only`，round 结束立即释放。Room `review_work` 接受精确 ReviewBinding、选定 self reviewer 的当前 WorkBinding，或当前 coordinator 的有效 CoordinationBinding；DM 单 Agent 中三种角色也可由同一 Agent 承担。
- `get_execution` 在当前 scope 没有 Execution 时返回权威 `state=unmanaged`，不会凭一次读取创建 Execution 或 CoordinationBinding；`prepare_plan_execution` 也只写非权威 proposal。只有完整 proposal 被 `plan_execution` 成功 materialize 后，才建立首个 WorkGraph 并进入 coordination。
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
ExecutionPlanProposal (durable, immutable, non-authoritative)
├── canonical Nexus Plan Document v1
├── owner/session/scope/coordinator + originating-round provenance
├── exact Execution/version/base-Plan + Goal activation/reserved-successor/predecessor fence
└── materialization receipt / Goal-confirmation recovery state

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

首次创建 transient Execution 必须先用 `prepare_plan_execution(plan_document)` 提交明确 objective、至少一条归一化后非空的顶层 completion criterion 和完整 Plan，再用返回的 `proposal_id + proposal_digest` 调用 `plan_execution`。commit 在同一 SQL transaction 创建 Execution 与 active Plan，不能先留下无 Plan 的 transient Execution。对历史或其他合法路径已经存在、但尚无 active Plan 的 Execution，`operation: replan` 可以在 exact Execution/version 且 empty base-Plan fence 下原子写入其第一个 Plan；该 document 不得复用不存在的旧 Work Item identity。Plan Mode 可以持久化前一步的 sealed proposal，但不能 materialize。objective 与顶层 completion criteria 是该 Execution 的 immutable fence：同一 outcome 和完成边界下改变路线、拆分、owner、dependency 或 evidence strategy，属于 `operation: replan`；显式改为不同 transient objective，必须 seal `operation: replace`、`replacement_reason` 和完整 successor graph；只放弃而不继续时使用 `abandon_execution`。Plan revision 不得借普通 replan 改写这些边界。

Execution 负责：

- 绑定 owner、session scope 和触发来源。
- 绑定可选 Goal ID 与 Goal objective revision。
- 表示一次执行是否 active、waiting、paused、completed、failed、cancelled 或 superseded。
- 提供跨 compact、resume、Room handoff 和 Goal continuation 的幂等身份。

同一 Goal 可以经历多个 Execution，但一次 continuation 必须明确恢复或新建哪个 Execution，不能从自然语言猜测。
当前 Plan 只由 `PlanRevision.status=active` 的唯一约束决定；Execution 不再保存第二个 active-plan 指针。

### 4.2.1 Execution Plan Proposal

Execution Plan Proposal 是完整 WorkGraph 从模型文本进入权威事务前的 durable、immutable、non-authoritative envelope。它解决 Provider 参数传输、Plan Mode 审阅、跨 round 恢复与权威 commit 之间的边界，但不属于 active Execution graph。

- proposal 只由一次完整的 strict Nexus Plan Document v1 产生；canonical document 一次 seal，后续 commit 只引用该不可变结果。
- canonical typed document、owner、session、scope、coordinator、originating-round provenance、target Execution/version、base Plan、Goal ID/revision/activation、Goal-reserved successor 与 typed predecessor 一起进入 digest；`proposal_id` 本身不是 capability。
- 读取和 commit 每次都重新校验 owner/session/scope/coordinator；materialization 还必须验证 digest 与 exact target fence。`create` 在 prepare 与 commit 时都从 trusted Goal gateway 解析完整 activation/reservation；即使 actor 没有携带 Goal，seal 后新出现或消失的 ambient Goal 也不能被静默绑定。任一 target/version/base Plan/Goal activation/reserved successor/predecessor 漂移都 fail closed，要求重新 prepare 完整图。
- proposal 在 round 结束、compact、runtime 重连和服务重启后仍可恢复。状态只沿 `sealed → materializing → materialized` 推进；永久 stale/authority 冲突进入 `blocked`，不允许改写原 document 后继续。`materializing` 恢复必须先用 expected proposal version 原子领取有界 lease，未过期的前台重放、后台 reconciler 或另一实例只返回 in-progress，不并发进入权威 materializer。
- authoritative transaction 的归因只认该 proposal stable materialization command 写入的 exact `plan_activated` event receipt；另一 command 产生的语义相同 graph 不是它的 receipt。读到 exact receipt 后可以恢复原 Plan ID；当 active Plan 后续已前进时，reconciler 仍以原 command receipt 完成 proposal，不把当前 graph 的语义相等当作历史归因。
- Goal-bound authoritative write 后的 confirmation 与 materialization receipt 分开持久化。confirmation 网关暂时失败，或当前 gateway 根本没有 confirmer，都保持 `materialized + confirmation=pending`；Goal completion/continuation 继续 fail closed，只由 reconciler 重试确认。
- proposal SQL 写入不创建 Execution、Plan、Work Item、Assignment、Attempt、Goal mutation 或 Room dispatch，因此允许发生在 Plan Mode；只有非 Plan Mode 的 `plan_execution` 可以产生权威业务写入。

### 4.3 Plan Revision

Plan Revision 是某个 Execution 在一个时间点的工作图快照。

- 同一 Execution 同一时刻只有一个 active Plan revision。
- Plan Revision 一经创建即不可变；修改名称、依赖、必需项、deliverable、acceptance criteria 或展示文案都创建新 revision。
- Plan Item 把稳定 Work Item 与该 Plan 使用的精确 immutable Work Item Spec 绑定。
- 新 revision 必须明确哪些 Accepted Work Item 在 `spec_hash`、输入与 objective fence 一致时复用，哪些被 supersede。
- Goal objective revision 变化时，旧 Plan revision 默认 stale；只有经过 rebase 的结果可以复用。
- 图的默认演进方式是单调扩展：新 revision 可以追加节点，让新节点依赖旧节点，并从旧节点连向新增下游；旧 revision、旧边和所有 Assignment/Attempt/Submission/Acceptance 历史永不物理删除。
- “不能删除旧图”约束的是历史事实，不是要求所有未来草案永远留在 active Plan。尚未执行的未来节点可以由带 `revision_reason` 的新 revision 显式 supersede；已运行或已验收节点不能被无声改义，也不能补一条会追溯改变其前置条件的入边。
- 当前实现激活 revision 时整体切换 active Plan，尚未提供 live Assignment/Attempt 跨 revision carry。因而运行中只能等待安全边界再扩图，或用 `supersede_active_work` 显式中断旧责任链；未来若支持热扩图，必须新增 carry identity 与 admission fence，不能仅靠“旧 logical key 还在”推断责任连续。

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

所有会进入模型执行契约的单个集合统一以 protocol 常量限制为 32 项：Execution completion criteria、Work Item acceptance criteria、单个 Work Item 的直接 dependency、`input_refs`、`output_scopes`、Submission `result_refs` / evidence、Acceptance criteria results 及每条 result 的 evidence、Resume evidence。普通 MCP collection schema 声明 `maxItems=32`；Plan Document 则由 strict parser 在 seal 前执行同一限制。service 在 normalization 前以稳定 `projection_limit_exceeded` 拒绝第 33 项，storage 对直接 authoritative command 重复同一校验；Plan Mode 也执行完整限制校验，但只允许写 non-authoritative sealed proposal。新写入受限后，动态 context 对异常历史数据只可用 `truncated="true" total="N"` 明示有界投影，structured WorkBinding 与 Dispatch contract 则 fail closed，禁止无标记 slice、假装完整或依赖模型猜遗漏项。

Plan Work Items 集合本身也使用同一个 32 项上限；Plan Document parser 与 service normalization 必须在任何单项校验和 proposal 持久化之前一致拒绝第 33 项。

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
- 单 Agent 或 Room managed Execution 中，Lead 自己执行也需要显式或派生的 self Assignment，避免“无人负责但状态 running”。`return_to_agent_id` 明确选择 owner、Lead 或另一成员作为 reviewer；后端不强制独立 reviewer。

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
- `(runtime_session_key, runtime_round_id, agent_round_id)` 只在 root Attempt 之间唯一；同一物理 parent round 内启动的 child Attempt 合法共享该三元组，并以 `parent_attempt_id + tool_use_id` 区分。Room round identity 不能误当成整张 Attempt 树的唯一键。
- `SubagentStart` 只能激活一个已经绑定 Assignment 的 Attempt。
- `SubagentStop` 只结束 child Attempt，并把结果证据交还父 Agent；不能自动创建 Submission，更不能直接 Acceptance。
- Snapshot 对 terminal Attempt 的有界压缩必须分别计算 root Agent 与 child Subagent 两条 evidence lane；child 不能因创建时间更新而遮蔽仍需完成 Submission 或幂等收口的 root Attempt。
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

当 Assignment 选择跨 Agent reviewer 时，Room Submission 还会在同一 SQL transaction 创建独立的 `ExecutionReviewDispatch`。它只负责把待审 Submission 可靠回交给 Assignment 预先规范化的 `return_to_agent_id`，不复用 worker Dispatch，也不创建 reviewer Assignment/Attempt。投递到 Room 后使用独立 `ReviewBinding`，完整绑定 Execution、Plan、Work Item、Spec、Assignment、Submission、review dispatch 与 reviewer target。owner 被选为 reviewer 时不创建回投；原 WorkBinding 可以记录 Acceptance，并在 coordinator 自审时把同一物理 round 升回 coordination。

只有 `decision=accept` 的 Acceptance 才能：

- 完成 Assignment。
- 解锁 dependent Work Item。
- 计入 Execution/Goal 完成审计。
- 作为可复用结果进入后续 Plan revision。

### 4.8 Objective Alignment 与条件循环

目标模型允许 WorkGraph 表达循环工作流，但不能把循环伪装成普通前置依赖。系统必须区分两种边：

- `depends_on` 是数据与验收前置条件；只参与 Ready 计算，始终必须构成 DAG。
- `control_edge` 是节点完成后的条件转移；只有受控校验节点可以产生回边，不参与首次 Ready 计算。

循环校验不依赖完整 Goal 生命周期。Goal continuation 中“按 objective、completion criteria 和权威证据逐项证明结果是否达成”的部分必须抽成共享 `ObjectiveAlignmentAudit`，输入固定为 objective、completion criteria、当前状态引用和 evidence，输出固定为：

- `aligned`：每条 criterion 都有足够权威证据，Agent 可据此选择结束、交付或继续优化。
- `not_aligned`：存在已确认差距，保存 gaps 与证据并把控制权返回 Agent；Agent 决定修订、扩图、再运行、交接或阻塞。
- `inconclusive`：证据不足或互相矛盾，保存缺失信息并把控制权返回 Agent；Agent 决定继续取证、等待、询问或采用其他合法路线。

Goal completion 和 Execution Gate 复用同一 audit contract，但权限与生命周期彼此独立：前者作为 Goal 完成审计的一部分，后者只是当前 Execution 的可选语义观测，不替 Agent 决定控制流。没有 Goal 的 transient Execution 也可以循环；只有循环需要跨 execution boundary、外部等待、定时恢复或超出本轮预算时，才由 Agent 按正常 promotion 规则考虑 Goal。

`audit_execution_alignment` 只记录当前 Agent Run → Gate 的 `guard` 边；未对齐或不确定时再记录 Gate → 当前 Agent 的 `loop_back` 观测边，表示工具结果已经把控制权交还调用者。它不会自动创建下一轮。只有 Agent 随后实际选择再运行时，runtime 才创建新的 immutable `NodeRun`/iteration identity。上一轮 Accepted 证据保持不可变，禁止通过清空 Work Item 状态来“重新运行”。

Agent 可以为 transient loop 选择 `max_iterations`、预算或时间边界，尤其在外部工具成本高或收敛不确定时；这是一项推荐策略，不是每张图的固定 schema 要求。系统级预算、取消和权限边界始终有效。无界或长期周期工作通常应由 Goal 或 Automation 承担持续性。可视化可以把 `control_edge` 画成回环箭头，但状态和调度不得把它投影成 `depends_on`。

当前交付状态必须对模型和开发者明确：共享 Objective Alignment 契约、Goal completion 适配和可选 Execution alignment Gate 已经存在；runtime NodeRun/EdgeRun 已能记录 Agent、Tool、Subagent、alignment Gate 与 review 返工回边，但静态 Plan 的 `depends_on` 仍是 DAG，尚未把通用 `control_edge` 作为可写 Plan 结构。alignment 得到任何结论后，都由 Agent 决定修订、追加节点、再运行、交付或阻塞；后端只为实际选择创建新的 immutable run、幂等投递并保持历史。Room 可把投递显示成 `@Agent`，无需用户逐节点查看或发送“继续”。

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
AND 若 Plan 声明了 terminal Work Item，则这些节点已 accepted
AND 没有 running Attempt
AND 没有绑定当前 Execution 的待激活 Dispatch、Room handoff
AND 没有绑定当前 Execution 的必须消费 input queue item
AND 没有 pending Submission
AND terminal Goal usage 已结算
```

Room 中“某个 Agent 回复过”不是完成证据。多 Agent 协作由实际 Assignment、Submission 与 required Work Item 的 Acceptance 证明；Lead 自己拥有的节点同样进入图和审计。

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
  substantial_complexity
```

`context_boundary` 是当前唯一的上下文容量类枚举：后端观察到的 compact/context boundary 与受信 usage exhaustion 都映射到它，不新增 `usage_boundary`。二者必须通过不同的 runtime command/event provenance 供审计，但 Goal 记录的 activation reason 相同。

### 6.2 判定规则

```text
promotion_available = every(authority_and_state_gate)
should_promote = Agent judgment from task facts and optional signals
```

用户显式要求 Goal 仍走 `create_goal`；已有 transient Execution 时，Agent 也可以选择 `promote_execution_to_goal`。两条路径都服从 objective clarity、原始授权、当前 permission mode、用户配置与 Goal 冲突检查，但不要求先命中某个持久性证据白名单。

Promotion 的后端硬门槛只有：

- objective 明确且 completion criteria 可形成。
- 原始用户 scope 授权继续推进；promotion 不增加权限。
- 当前不是 Plan Mode，用户没有关闭自动 Goal。
- 当前有可绑定的 Execution，且同一 scope 没有另一个 active Goal。

是否仍值得继续推进不是额外的后端 Plan-shape 门禁：current active/waiting 状态已经阻止 terminal Execution 被提升，Agent 可以结合目标语义、尚未声明的新范围或恢复成本决定是否建立 Goal。

系统可以向 Agent 提供结构化建议事实：

- 已观察：当前 boundary 到达时仍有 required Work Item、pending Submission、未完成 Assignment/Attempt、待消费的绑定 handoff，或 recovery 正在恢复旧 Execution。
- 可预测且 durable：已创建顺序 Room dependency/dispatch、必须等待外部状态、已安排跨 turn retry，或已知 context/usage boundary 前无法完成 required work。

`context_boundary`、`scheduled_retry` 等系统事实不能由模型传入布尔值伪造。runtime 与 scheduler 仍以 CAS、typed provenance 和审计事件记录这些事实，供 Agent 判断和 UI 解释；它们缺失时工具仍可调用，只要硬门槛成立。`substantial_complexity` 是 Agent 对长时恢复成本、范围和协调负担的语义判断，只作为审计理由，不要求后端证明固定阈值。

Room 中已绑定到其他 Agent 的 Assignment/Dispatch、尚未完成的跨 Agent dependency chain、Lead 自己需要跨轮整合的工作，以及高恢复成本都可以影响判断。参与人数本身不自动创建 Goal。

以下事实不能由后端机械等同为 Goal，但 Agent 可以结合任务语义使用：

- 提示词很长、步骤很多或整体复杂度较高。
- 使用了 Plan 或子智能体。
- Room 中有多个 Agent，但所有交付可在当前 boundary 内完成。
- 只剩 runtime-local、optional 或不可验收的后台工作。

“自适应提升”仍由 Agent 在 coordination lane 主动选择，不是后台定时器。后端投影可用性和建议事实；Agent 调用后，服务端重新验证硬门槛并提交 promotion，不重新裁决“这个任务是否足够值得”。未知权限或状态 fail closed，但任务策略不 fail closed。

### 6.3 Execution promotion

不确定任务先创建 transient Execution。当继续推进需要跨 execution boundary 时，将同一个 Execution 原子提升为 Goal-bound Execution：

- 保留 Execution ID。
- 保留唯一 active Plan revision。
- 保留 Work Item、Assignment、Attempt、Submission 和 Acceptance。
- 记录 `goal_promoted` 事件。
- 禁止重建或复制 Plan。
- 同一 Execution 的 promotion 使用 idempotency command ID，重复调用返回同一 Goal binding。
- 记录 Agent 选择的 activation reason、当时可见的建议 signals 与 objective/Execution version fence，便于恢复和审计；signals 可以为空。

Goal 延长 intent 的生命期，不扩大权限。遇到需要新授权、危险操作或重大用户选择时，Goal 必须 blocked。

### 6.4 显式 Goal 与 Execution 收敛

用户显式要求持续追求时走 `create_goal`，不伪造 adaptive signal。显式路径与 Execution 仍只有一条状态链：

- scope 中已有兼容 transient Execution：`create_goal` 创建或复用 Goal 后，以 `user_explicit + persistence_requested` 绑定该 Execution，保留其 Plan 和全部执行历史。
- 同时需要新 Goal 与首张 WorkGraph 时，两步具有因果顺序而不是并行关系：必须先等待 `create_goal` 成功，再调用 `prepare_plan_execution` seal 绑定该 Goal fence 的 proposal。模型提示、Goal/Execution Skill 与两个工具描述都必须保留这一顺序；后端仍在 prepare/materialize 两阶段重读 active Goal 并拒绝竞态，不能把“失败保护”放宽成隐式猜测未来 Goal。
- 先有 Goal、后 prepare Plan：`create_goal` 在创建 Goal 时先持久化由 explicit command 确定的唯一 Execution ID；`prepare_plan_execution` 只读地把该 Goal ID/revision、activation provenance 与 reserved target seal 进 proposal，`plan_execution` materialize 时复用该 identity。任一阶段失败重试都不能创建第二个 Execution。历史上已经写入 explicit command 但缺少 `execution_id` 的 Goal，从该 server-owned command 确定性恢复同一 reservation，续跑与 proposal seal 共享该身份，并在 materialization CAS 时把反向 binding 写实。
- fresh Goal-bound `operation: create` 的 objective 权威来自 active Goal，而不是 provider 重复提交的 Plan transport：输入可以省略或概述 root objective，service 必须在校验与 digest 前写入 exact Goal objective，并在 materialization 时按 Goal ID/revision/objective/reservation 重新验证。首次 Plan 的 completion criteria 仍由 Plan 把 Goal 操作化，materialize 后写入 Goal/Execution binding 并在同一 objective revision 内冻结。
- 重复调用只修复尚未落地的一侧，不创建第二个 Goal、Execution 或 Plan。
- Goal 与已经存在的 Execution objective、scope 或已有 binding 不兼容时，分别返回稳定的 `goal_objective_conflict`、`goal_scope_conflict`、`goal_binding_conflict`，不能静默分叉；provider Plan transport 与 Goal 原文的措辞差异不属于两个权威对象之间的冲突。
- Goal completion 必须找到 objective revision 一致的绑定 Execution，并通过同一 WorkGraph completion audit；缺 binding、binding 冲突或审计器不可用均 fail closed。
- 模型完成托管 Goal 还必须先保存当前 objective revision、当前 physical round 的 `aligned` Objective Alignment report；completion-tool-miss 的系统兜底消费同一报告，不能从最终回复文字伪造。用户或 app-server 显式设置 Goal 状态仍属于控制面 authority，只执行 WorkGraph/readiness 硬门禁，不要求用户先模拟一次模型语义审计。

### 6.5 Goal objective revision rebase

Goal 一旦绑定 managed Execution，objective 变化不再是 Goal row 的普通字段更新，也不是当前 Execution 内的 replan。MCP `retarget_goal`、HTTP Goal PATCH 与 app-server `thread/goal/set` 必须在各自完成来源授权后进入同一个 application coordinator；任何缺少 coordinator 的 managed Goal mutation 都 fail closed。Room 模型入口只有当前 Goal Lead 可以调用，用户 HTTP 入口保留 server-owned Room creator/lead metadata，不能借 metadata replacement 改写 Execution binding、activation provenance、completion criteria 或 transition state。

跨 Goal 与 Execution 两个持久化域采用可重放 saga，不伪装成单库 transaction：

1. **prepare Goal transition。** 以 `(goal_id, old_objective_revision, normalized_requested_objective)` 生成稳定 transition、command 与 reserved successor Execution ID；CAS 写入 server-owned `objective_transition`，状态为 `prepared`。Transition 至少保存 old/new revision、old/reserved successor Execution ID、用户 requested objective、首次确定的 canonical target objective、reason 与 source。重试先匹配持久化 requested objective，不能因后台 objective rewriter 再次运行并产生不同措辞而分叉第二个 revision。此时 canonical Goal objective/revision 尚未改变。
2. **supersede old WorkGraph。** 按精确 Goal ID 与 old objective revision CAS terminalize 旧 Execution：Execution/active Plan/未完成 Work Item 进入 `superseded`，current Assignment release，live Attempt 逻辑进入 `interrupted`，未送达 Assignment/Review outbox cancel；同一 SQL transaction 必须先为每个将被中断的 Attempt 写入 exact-target Cancellation dispatch，由独立 consumer 重试物理中断。Submission、Acceptance 与 event 历史保持 immutable。旧 Execution 的 `execution_superseded` event 必须保存 reserved successor ID 与 old/new Goal revision。
3. **commit Goal revision。** 只有旧图完成 fencing 后，Goal 才 CAS 切换 target objective/new revision，把 reverse `execution_id` 指向 reserved successor，删除旧 completion criteria，并进入 `awaiting_plan`。这不是完成状态。
4. **seal successor proposal。** `prepare_plan_execution` 用 `operation: create` 提交完整 successor graph，读取 trusted Goal transition 并把 reserved successor ID、predecessor、owner/session/scope、Goal ID/revision 与 activation provenance seal 进 immutable digest。Plan Mode 可以保存该 non-authoritative proposal，但不能创建 Execution/Plan 或推进 Goal transition。
5. **materialize and confirm。** 下一次非 Plan Mode `plan_execution(proposal_id, proposal_digest)` 必须命中同一 exact fence，并在一个 SQL transaction 内创建 Goal-bound successor Execution、完整 WorkGraph 与第一版 active Plan。Successor 的 typed `replaces_execution_id` 必须指向 `superseded` predecessor，且 owner/session/scope/Goal/revision 必须连续；数据库还必须验证 predecessor supersede event 预留的就是该 successor。Execution/Plan commit 后，Goal confirmation 作为 durable saga receipt 从 `pending` 重试到 `confirmed/bound`；进程在两者之间失败时，reconciler 只补 confirmation，不创建第二张图。只有 confirmation 完成后 Goal completion 与自动 continuation 才重新开放。

`prepared | awaiting_plan | binding_reserved` 以及 materialized-but-confirmation-pending 都视为 pending：Goal complete、自动 continuation、旧 revision mutation 与迟到 Room wake/child result 必须 fail closed。模型上下文在 `prepared` 明确要求用同一 target 重试 `retarget_goal`，在 `awaiting_plan` 先给 `prepare_plan_execution`，sealed proposal 再给 `plan_execution`；相同 command/proposal 重试只修复未完成阶段并复用同一 identity，不同 target、revision、digest 或 successor 冲突。Plan Mode 可以产生 durable proposal SQL，但对 Goal metadata、Execution、Plan、outbox 与 Room ledger 零 mutation。新 graph 默认不 carry 旧 Work Item、Assignment、Attempt、Submission 或 Acceptance；未来若支持 reuse，必须是独立、显式且重新验收的协议。

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
- active Room Goal 存在时，Goal Lead 是 Goal-bound Execution 的 coordinator；它可以 review，也可以为具体 Assignment 选择 owner 或另一成员 review。
- creator、host 与 Goal Lead 的身份必须由后端上下文明确给出，不能让模型从名字推断。

Lead 负责：

- 创建和修订 shared Plan。
- 在有帮助时定义依赖、验收标准和输出范围。
- 分配 Ready Work Item。
- 自己验收、或选择另一名 Agent 验收/退回 Submission。
- 处理失败、阻塞、接管和重分配。
- 让自己的协调、整合、复核、接管和最终交付步骤也成为可见 NodeRun；是否拆成独立 Work Item 由任务需要决定。
- 发起 Goal completion audit。
- 在用户明确改变或放弃当前 non-Goal objective 时发起 transient Execution replacement/abandonment；Goal-bound objective 变化仍走 Goal revision/rebase。

Lead 可以亲自执行 shared Work Item，并把 `return_to_agent_id` 设为自己完成显式自审；也可以选择另一名 Room Agent 独立复核。自审不会因 owner 身份自动成立，仍要保存 immutable Submission 和 append-only Acceptance。独立 reviewer 同样只获得目标 Submission 的精确能力，不因此获得整个图的 coordinator 权限。生产与验收是否分离是 Agent 的质量策略，不是 Room self Assignment 的代码门禁。

### 7.3 Room 成员

Room 成员可以：

- 读取与自己 Assignment 有关的稳定输入。
- 开始、阻塞、提交自己的 Work Item。
- 在自己 Assignment 范围内使用 subagent。
- 当自己是 Assignment 选定的 reviewer 时 review 对应 Submission，包括显式自审。
- 被其他 Assignment 选中时，通过精确 ReviewBinding 独立 review 对应 Submission。
- 向 Lead 提议 Plan 修改或新 Work Item。

Room 成员不能：

- 修改 sibling Work Item。
- 分配共享 Work Item，除非被显式授予 coordinator 权限。
- Acceptance 自己的 Submission。
- 完成 shared Goal。
- 替换或放弃当前 shared Execution。
- 重复执行已经归属其他成员的 produce Work Item。

### 7.4 子智能体

子智能体是父 Agent 为隔离上下文、局部并行或专业注意力而启动的本地 executor。父 Agent 仍负责整合和最终交付；当当前 round 存在可精确验证的 managed Assignment 时，Nexus 额外把该运行绑定为 child Attempt，否则它只属于 runtime Graph。

子智能体不能：

- 创建、重定向、暂停或完成 Goal。
- 创建、替换、取消或完成 Execution。
- 修改 shared Plan。
- 分配 Room 成员。
- 使用 Room public/directed communication 工具。
- Acceptance 自己的结果。

子智能体可以在本地使用临时 Task/Todo，但这些局部步骤不成为 shared WorkGraph 的第二真相源。

原生 `Agent` 工具经过 `PreToolUse`，但 Hook 只保护 Plan Mode、身份、授权和受管状态一致性，不能把“当前是否值得委派”变成代码策略。准入分为两种合法模式：

1. `managed`：当前不是 Plan Mode，当前 Agent 恰好有一个可精确验证的 bounded current Assignment、一个 pending/running parent Agent Attempt，并且本次存在唯一 `tool_use_id`。后端原子预留 child Attempt；模型不提供 Execution、Assignment、Attempt 或 version。
2. `runtime_only`：没有 managed Execution、当前是普通 conversation、没有 Assignment、存在零个或多个可委派 Assignment、缺少 durable correlation，或当前责任不适合绑定时，原生子智能体仍可运行。Bridge 继续记录 Agent/Subagent/Tool Node Run 与方向边，但该运行不创建 Assignment/Attempt/Submission/Acceptance，也不能充当 Goal completion evidence。

Plan Mode 仍禁止真实执行；错误 owner、stale WorkBinding、权限缺失和 managed 写入失败仍拒绝。无法精确命中 managed lifecycle 时，后端不猜测、不改写 Attempt，并让事件按 runtime-only 观测继续；已有 child Attempt 由精确迟到事件或 durable reconciliation 收口。

动态 `<nexus_execution_context>` 投影同一事实：`subagent_admission` 包含 `eligible`、原生工具名 `Agent`、`candidate_assignment_count` 和 `binding_mode=managed|runtime_only`。managed 模式给出唯一 Assignment/parent Attempt；runtime-only 模式说明为什么不能记为受管证据。除 Plan Mode 外，`allowed_actions` 可以包含 `Agent`；这只是可用能力和记账模式，不是要求模型一定调用。

`tool_use_id → child Attempt` 只属于 managed 模式，也承担重复写入 fence。runtime manager 按物理 parent round 注册 callback；managed `PreToolUse` 成功时把 callback、parent round、tool ID 与当时可见的 SDK session/task identity 冻结为 immutable lifecycle binding。后续 `SubagentStart/Stop/PostToolUseFailure` 只有精确命中时才更新该 Attempt：优先使用可信 callback correlation，再使用唯一的 task/SDK Agent identity；零匹配或多匹配都不得猜写受管状态，但不会撤销 runtime-only 图观测。parent round 结束只撤销新的 managed launch capability，已创建的 child binding 保留到迟到 Stop/重复事件完成幂等收口，不能被 successor round 覆盖。不能把 SDK `agent_id` 猜写成 `child_session_id` 或 `sdk_task_id`。`SubagentStop` 只终结精确 child Attempt，不自动产生 Submission、Acceptance 或 Goal completion evidence。

同一 parent Assignment 可以拥有多个并行 child subagent Attempt，前提是每次 launch 都有唯一 `tool_use_id`，后续 lifecycle 能精确命中该 binding；缺失或歧义 correlation 的单个事件 fail closed，不能按“最新 child”猜测。真正彼此独立、需要跨 Agent 交接或单独验收的责任通常更适合拆成多个 Work Item，但这仍是 Agent 的选择。

## 8. 模型决策协议

模型在 substantial execution 前参考以下问题；它们是决策清单，不是要求每次逐项输出或固定流水线：

1. 当前是 Plan Mode 吗？如果是，可以用 `prepare_plan_execution` 校验并持久 seal 完整 proposal，也可以验证 abandonment；不得 materialize Execution/Plan、取消工作或自动续跑。
2. 当前 round 是 conversation、coordination、work、review 还是 subagent？conversation 先正常回应；只有 Room coordinator 判断请求确需可追踪交付时，才显式调用 `get_execution`，或完成 `prepare_plan_execution → plan_execution` 进入 coordination。
3. 若已进入 coordination/work/review，是否已有 Execution Context？如果有，先恢复而不是重建。
4. 用户是在改变同一 objective 的执行路线、改为不同 transient objective，还是只放弃？前者 seal `operation: replan`；第二种由 coordinator seal 带 `operation: replace`、`replacement_reason` 的完整 successor graph，再用 proposal receipt 原子 materialize；第三种用 `abandon_execution` 只取消旧 graph。Goal-bound Execution 对后两者返回 `goal_retarget_required`。
5. 用户是否明确要求持久追求，或当前 Execution 是否需要自适应 Goal？
6. 请求是否是一个原子 deliverable？
7. 如果图能明显改善并行、交接、恢复或可见性，创建或修订 Plan revision；否则直接执行。
8. 读取 Ready Work Item。
9. 为每个 Ready Work Item 选择 self、subagent 或 Room member。
10. 并行前验证已声明 dependency，并在有价值时声明 input stability 或 output scope。
11. 在任务需要时提交和 Review；reviewer 可以是 owner、Lead 或另一成员。
12. 根据新证据、ready 节点和 alignment gaps 自行决定继续、扩图、返工或收口。
13. 在有价值时执行独立 integration/verification，而不是为了固定流程制造节点。
14. 对受管 Execution/Goal 运行完成审计。

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
  <graph_digest notation="nexus-dag-v1"
                scope="actor_slice"
                plan_revision="4">
    <nodes>
      <node key="W1" subject="Collect sources"
            kind="produce" status="accepted" />
      <node key="W2" subject="Compare evidence"
            kind="produce" status="running"
            owner_agent_id="analyst" current_actor="true" />
    </nodes>
    <edges>
      <edge from="W1" to="W2" kind="hard" />
    </edges>
  </graph_digest>
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
    <action>prepare_plan_execution</action>
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

动态上下文只包含当前 actor 做决定所需的有界状态，不转储完整事件日志。`graph_digest` 是从 typed WorkGraph 单向生成的确定性读模型：coordinator 得到完整当前 DAG，member/subagent 只得到自己的节点与所需上游切片。它帮助模型快速理解拓扑，但不是写入格式或真相源；`prepare_plan_execution` 的 strict Nexus Plan Document v1 经 parser 规范化后形成 sealed typed proposal，只有 `plan_execution` exact-fence materialization 后的服务端实体才是权威 Execution/Plan。UI/调试可以从同一读模型派生 Mermaid，不得把 Mermaid 文本反向解析成执行状态。

`allowed_actions` 只能表达工具级 affordance，但 Plan Document 的 `operation` 同时表达 create、replan 与 transient replacement，因此动态上下文还必须投影 `execution_transition`。`replace_current_allowed` 控制是否可 prepare `operation: replace`，`abandon_allowed` 控制 `abandon_execution`；`validation_only` 表示 Plan Mode 只允许 seal non-authoritative proposal。transient coordinator 可 prepare replan/replace；Room member/subagent 为 false 并带 authority reason；Goal-bound coordinator 仍可 prepare 普通 replan，但 replacement/abandonment 为 false 且稳定 `reason_code=goal_retarget_required`。

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

1. **先给定位，再给工具。** 每轮上下文明确 actor role、当前 Assignment、已声明的 dependency/deliverable/criteria、completion blocker 和当前可用能力；不能要求模型从聊天历史重建这些事实，也不能把建议伪装成 `allowed_actions` 权限。
2. **模型只做语义决策。** 模型负责 Plan、分配、提交、验收、阻塞、接管和 promotion proposal；`running`、Attempt identity、outbox claim、event sequence、version increment 与恢复由工具适配器、Hook 和服务端自动维护。
3. **原子任务零工具税。** 一个当前 boundary 内可完成的小交付可直接执行；不能为了展示“进度”强迫模型创建 Goal、Plan、Work Item 或手工 start/complete 状态。
4. **完整文档一次 seal，opaque receipt 一次 commit。** `prepare_plan_execution` 只接收一个包含完整 WorkGraph 的 strict Nexus Plan Document v1 YAML scalar；服务端严格解析、规范化并返回 `proposal_id + proposal_digest`。`plan_execution` 只接受这两个 opaque scalar；该 receipt 唯一引用已 seal 的 canonical typed document，不在 commit 时重传 Work Item 或 graph 字段。
5. **不让模型维护并发控制。** prepare adapter 从 trusted current snapshot 捕获 Execution version、base Plan、scope/coordinator 与 round provenance，并从 trusted Goal gateway 捕获 Goal ID/revision/activation、reserved successor 及 predecessor，一起纳入 digest；commit 重新读取 authoritative Execution/Plan 与 Goal state 做 exact-fence CAS。优先用 `tool_use_id` 形成 idempotency command ID，bridge 未提供时使用 server session/round、工具名、canonical input 与 snapshot revision 的稳定 hash。应用服务再按具体 mutation 派生不同的 event command key，因此 materialization、Execution/Plan transaction 或 Goal confirmation 子步骤不会互相冲突。recovery 前使用 proposal version CAS 领取有界 lease；stale 时 proposal 进入 stable blocked/reprepare 路径，不让模型猜 version。
6. **工具结果必须可恢复且不重复状态。** 每个 mutation 返回 `applied | no_op | rejected`、稳定 reason code、最新 snapshot revision、实际改变、有序 `next_actions` 与一份 actor-specific `execution_context`。完整 Snapshot 保留给同进程协调和 HTTP/UI 读模型，Execution MCP 不再把它与派生 context 同时发给模型。若任一 MCP 结果仍超过 runtime 上限，SDK 只保存原始结果并提示按 schema、搜索、`jq` 或 offset/limit 定向读取；只有用户明确要求 exhaustive whole-result audit，或正确性确实依赖每条记录时，才要求全文读取。
7. **只用硬事实收窄 affordance。** dynamic context 的 `allowed_actions` 使用真实工具名，只按身份/权限、round binding、Plan Mode、当前状态和引用完整性收窄：没有 Ready Work 就不提供 `assign_work`，没有自己的 current Assignment 就不提供 `submit_work`；`review_work` 可由精确 ReviewBinding、选定 self reviewer 的 WorkBinding，或当前 coordinator 的有效 CoordinationBinding获得。conversation-only member 不挂载 Execution MCP；conversation-only coordinator 只可读取或显式建图。replacement、abandonment、Goal 冲突和用户禁用等仍是硬权限/状态边界。是否应调用某项合法工具、是否独立 review、是否再跑一轮由 Agent 决定，不能再用产品偏好做第二层隐藏白名单。该列表不限制研究、编码和浏览等普通 conversation/task 工具。
8. **把下一动作所需语义放在动作旁。** `assigned_work`、`ready_work`、coordinator `active_assignments` 与 `pending_reviews` 投影 current Spec 的 logical key、objective、deliverable、acceptance criteria、`input_refs`、canonical `output_scopes` 与有序 `resolved_dependencies`；每条依赖给出 upstream Work Item/logical key/current Spec 与 kind，只有已经 Accepted 的 upstream 才携带 immutable Submission summary/refs/evidence 和 Acceptance criteria results，未验收依赖只暴露 status/blocker。structured Room WorkBinding 保留自身 mutation capability 以及这些直接上游的只读 WorkItem/Spec/PlanItem/dependency/output claim/accepted delivery 投影，继续过滤 sibling live Assignment/Attempt/Dispatch/state 和任何 unreviewed payload。若上次 Submission 被 rejected / changes requested，还必须携带 latest review decision、feedback 与逐条 criteria results。`resume_work` 记录的 resolution/evidence 同时进入后续 `ready_work` 和新 owner 的 `assigned_work`，避免模型依赖聊天历史猜返工要求或刚补齐的外部输入。任何带 `truncated=true` 的异常历史投影都表示上下文不完整，模型不得据此提交、验收或宣称 completion，应等待后端修复或改走 fail-closed 恢复。
9. **人类可读名不是关联键。** 模型可以看到 `logical_key=W2` 和 subject，但 Assignment、Attempt、Dispatch 依赖服务端 ID；不得靠显示名、自然语言相似度或 `@` 文本猜 binding。
10. **结构化路径是可追责交付的主路径。** Room 分工由 `assign_work` 创建 Assignment + Dispatch；子智能体若存在 exact managed responsibility，则由 pending Attempt + `PreToolUse: Agent` 自动绑定。普通 `@`、runtime-only Subagent 与 SDK Task 可以完成对话、局部探索和节点内工作，但不承担共享 Assignment、Acceptance 或 Goal completion 的正确性。
11. **完成应由审计触发。** 任一 Acceptance、runtime terminal event 或图修订后服务端重算 Execution readiness；若 Plan 声明 terminal 节点则纳入审计。Stop/Goal complete 只消费 blocker audit，模型不需要手工同步一串“已完成”状态。

因此，模型面对的决策路径应始终接近：

```text
直接做一个原子交付
OR Agent 自行组合 plan / assign / subagent / submit / review / alignment
   后端记录实际发生的 NodeRun/EdgeRun，并只拒绝不安全或不一致的动作

用户明确换成 different transient objective
→ prepare_plan_execution(operation: replace, complete successor graph)
→ plan_execution(proposal_id, proposal_digest)
→ backend atomically supersedes old Execution
  and creates successor Execution + active Plan

用户明确放弃 transient objective 且没有 successor
→ abandon_execution
→ backend cancels old graph and creates nothing

required work 必须跨 boundary
→ propose promotion
→ Agent 说明选择，backend 只校验权限/状态/用户配置
→ resume the same Execution
```

## 10. Tool 协议

Execution Orchestration 提供产品级 MCP 工具：

### 10.1 `get_execution`

读取当前 Execution 的紧凑 actor-specific action view：revision、确定性 graph digest、当前 Assignment、Ready Work Item、pending review、completion blockers 和允许动作。完整 Snapshot 不进入模型工具结果。

只读；模型在恢复、compact 后或状态冲突时使用。

### 10.2 `prepare_plan_execution` 与 `plan_execution`

Plan 创建、replan 与 transient replacement 共用固定两步协议：

1. `prepare_plan_execution(plan_document)` 接收一段完整 strict Nexus Plan Document v1 YAML text，严格解析、规范化和校验后持久 seal immutable non-authoritative proposal，返回 `proposal_id` 与 `proposal_digest`。
2. `plan_execution(proposal_id, proposal_digest)` 不再接收 objective、items 或 revision flags；它重新读取 proposal 与 authoritative snapshot，验证 exact fence 后幂等、原子 materialize。

第一步承载完整语义，第二步只提交 opaque receipt。YAML string 是唯一模型传输格式；解析后的 canonical typed document 与 trusted authority/target envelope 共同计算 digest，只有 service/storage 生成的 typed 实体可以进入 domain validation 和权威 transaction。

root boundary 的权威来源由 operation 与当前状态决定：active Goal 下的 fresh `create` 从 Goal 继承 exact objective，Goal-free `create` 与 `replace` 使用文档 objective，`replan` 从当前 Execution 继承 objective 与 completion criteria。新建 Goal-bound graph 时必须先完成 `create_goal`，不能与 `prepare_plan_execution` 并行；Goal-bound create 的输入 objective 可以省略。改变 Goal 必须先走 `retarget_goal`，不能通过 Plan transport 改写。

Nexus Plan Document v1 的形状如下：

```yaml
nexus_plan: 1
operation: create # create | replan | replace
objective: "Produce and verify a small report" # active Goal create 或任意 replan 可省略
completion_criteria:
  - "The verified report exists"
revision_reason: ""          # replan 时必须非空
supersede_active_work: false # replan 时按需填写
replacement_reason: ""      # replace 时必须非空
items:
  - logical_key: produce
    kind: produce
    subject: "Produce the report"
    objective: "Write the requested report"
    deliverable: "report.md"
    acceptance_criteria:
      - "The report covers the requested scope"
    required: true
    terminal: false
    parent_logical_key: ""
    depends_on: []
    soft_depends_on: []
    input_refs: []
    output_scopes:
      - "file:report.md"
    shared_output_scopes: []
```

`operation: create` 用于当前 scope 没有 current Execution；`replan` 必须命中 current Execution、提供非空 `revision_reason`，并保持其 immutable objective/completion boundary；当该 Execution 合法存在但尚无 active Plan 时，empty base-Plan fence 表示给它原子写入第一个 Plan，document 中所有 Work Item 都必须是新 identity。`replace` 只用于用户明确改成不同 transient objective，必须提供新的 objective、completion criteria、`replacement_reason` 与完整 successor graph。Goal-bound Execution 不允许 `replace`，必须进入 Goal retarget/rebase。

`items` 必须非空；每项至少提供唯一 `logical_key`、`kind`、`subject`、`objective` 与 `deliverable`。dependency 只用本 document 内 logical key 表达；服务端生成 opaque Plan/WorkItem/Spec ID。acceptance criteria、parent、hard/soft dependency、input ref、exclusive/shared output scope、required/terminal 标记按任务需要声明。parser 只接受一个 UTF-8 YAML mapping，拒绝空文档、未知或重复 key、multi-document、null、隐式 timestamp、anchor/alias、merge key、自定义 tag、placeholder、资源上限超限和无效 graph，并返回稳定 path 与 line/column。随后 service 继续执行身份、核心字段、32 项集合上限、引用完整性、DAG、output scope 冲突、operation 与 authoritative state 校验。

parser 的 allowed/required 字段集合同时是 MCP `plan_document` schema 的唯一字段真相源；模型说明不得再用 `dependencies`、`description`、`acceptance`、`scopes` 等概括词代替真实 key。Plan Document 被拒绝时，结果除精确 path/line/column 外还返回同一真相源生成的 `document_contract`：全部 root/item allowed fields、每项 required fields、常见错误别名修正和一份 parser-valid create example。调用方应据此一次重写完整 YAML，不得根据首个错误逐字段删除或猜测下一字段。

成功 prepare 后，proposal 持久保存 canonical document、originating-round provenance、owner/session/scope/coordinator、target Execution ID/version、base Plan ID、Goal ID/revision/activation provenance、Goal-reserved successor、typed predecessor 与覆盖这些 immutable 字段的 digest。proposal ID 不是 capability；每次读取/commit 仍须匹配 trusted access identity。document 一经 seal 不可编辑，模型若改变任何节点、边、文字或 operation，必须重新 prepare 得到新 proposal。对 `create` proposal，prepare 与 commit 都必须通过 trusted resolver 获得完整 Goal activation；一份 seal 时 Goal-free 的 proposal 不会在 commit 时自动吸附当时的 ambient Goal。

Plan Mode 允许 `prepare_plan_execution` 写 proposal SQL，因为它不 mint Execution/Plan/WorkItem/Spec ID，不改变 Goal metadata，不激活 Assignment/Attempt，不写 Dispatch/Cancellation outbox，也不触发 Room reconciliation。`plan_execution` 在 Plan Mode 必须拒绝，并提示离开 Plan Mode 后重用同一 receipt；proposal 跨 round、compact、runtime 重连与进程重启保留，不需要重新生成 YAML。

materialization 执行以下不可分割的 fence：

- `proposal_id + proposal_digest` 必须成对匹配，且当前 owner/session/scope/coordinator 与 proposal access 一致；trusted Goal resolver 还必须取得非空 `owner_user_id` metadata 并与当前 owner 精确相等，不能仅凭可能碰撞的 session key 选择 Goal。
- `create` 要求仍无 current Execution；`replan` / `replace` 要求 current Execution ID、version 与 base Plan 仍和 seal 时完全一致。`create` 还会重新解析 Goal ID/revision/activation、reserved successor 与 predecessor；任一字段出现、消失或变化都拒绝该 proposal。
- proposal 以 CAS 从 `sealed` 进入 `materializing`，并取得由 proposal 派生的稳定 command 与 reserved Execution identity。首次尝试自带一段有界 lease；同一 reservation 的并发 replay 只能得到 `not due` 并重新读取状态，不能继承首个 caller 的 lease。恢复或前台重放必须在 deadline 到期后用 expected proposal version CAS claim 新 lease，未持有 lease 的调用不得进入 authoritative command。权威存储继续用已有的 create-with-plan、write-plan 或 replace-with-plan transaction，一次完整成功或整体拒绝。create/replace 不留下 planless Execution；已有 planless Execution 则可以通过 empty base-Plan fence 的 replan transaction 获得第一个 Plan。
- 同一 receipt 的重复调用返回同一 materialization；服务在 commit 后、proposal row 回执前崩溃时，reconciler 只通过 `(reserved Execution ID, stable materialization command ID)` 查找该 command 写入的 exact authoritative `plan_activated` event 和 Plan ID。当前 graph 与 proposal 语义相同不足以证明归因，后续 active Plan 已前进也不会抹去原 command receipt。临时错误按 durable deadline 重试；永久 target drift/authority conflict 把 proposal 置为 `blocked`。若并发 worker 在 authoritative commit 可见前误赢得 blocked CAS，只有随后出现 exact command receipt 的 blocked proposal 才进入恢复并原子收敛为 `materialized`；真正没有 receipt 的 blocked proposal 保持终态，必须重新 prepare 当前完整图。
- 若 Execution/Plan transaction 已提交而 Goal binding confirmation 失败，proposal 记录 `materialized + confirmation=pending`。未实现 confirmer 也是显式 confirmation failure，不能降级为已确认。Goal 仍 fail closed，不开放 completion/continuation；启动和周期 reconciler 只重试 confirmation，成功后记为 `confirmed`，不会重放出第二个 Execution 或 Plan。

普通追加仍 prepare 一张完整 successor graph，而不是发送若干“加节点”命令。服务端按 `logical_key` 复用旧 Work Item：新增 logical key 创建节点；旧节点可以作为新节点的上游，新边也可以连接新节点；任何新边都不能指向旧节点，因为那会追溯改写旧节点的 Ready 条件。已有节点的 spec、parent、required/terminal 和入边保持一致时属于单调扩展。所有 replan 都必须解释非空 `revision_reason`；省略旧节点、修改旧节点或改变其入边还属于 superseding replan，必须具备 supersede authority，并继续受 active work、未审 Submission 和 revision fence 保护。任何文字、Spec 或 dependency 修改都产生新的 immutable Plan revision，不得原地改写历史。

当前持久层只在没有 current Assignment 和未审 Submission 的 quiescent boundary 激活 ordinary successor Plan revision。运行中热扩图不能只放宽这道校验：Assignment、Attempt、Dispatch、Room WorkBinding 都携带原 Plan identity，必须先定义同一 Work Item/spec 如何显式 carry 的持久协议和 admission 规则。该协议落地前，模型应等待当前责任链收束；`supersede_active_work: true` 会真实捕获 Cancellation dispatch、释放旧 Assignment、终结 Attempt 并取消未送达 Dispatch，不能拿它伪装 carry。与当前 active Plan 规范化后完全一致的 document materialize 为 semantic no-op，不产生空 revision。

transient replacement 仍是一个 SQL transaction：CAS current revision，先为旧图每个 pending/running Attempt 写入 exact-target Cancellation dispatch，再 supersede 旧 Execution/active Plan，收束旧 Work Item、Assignment、Attempt 与未送达 Assignment/Review Dispatch，创建带 typed `replaces_execution_id` 的 fresh successor Execution、完整 WorkGraph 与 active Plan，并追加旧 `execution_superseded` 和新 `execution_created` / `plan_revised` 事件。同一 `(owner_user_id, session_key)` 的 current 唯一约束始终成立，不能先 terminalize 后依赖第二个 command 补 Plan。

旧 Plan、Attempt、Submission、Acceptance 和 event 保持 immutable history。新 Execution 不继承 Work Item/Assignment，旧 Accepted 结果不计入 successor completion；未来复用必须由独立显式 import/rebase 重新验证 objective fence、input 与 `spec_hash`。Room handoff/queue/slot 仍属于 workspace ledger，不与 SQL transaction 伪装成跨存储原子提交；SQL commit 后由幂等 reconciliation 清理投影，在此之前旧 terminal fence 必须拒绝迟到 wake、queued slot、child launch 或 mutation result。

### 10.3 `assign_work`

把 Ready Work Item 分配给当前 Agent 或 Room member；当前 Agent 可以随后选择自己执行或在该 Assignment 下启动 subagent Attempt。

前置条件：

- Work Item 为 Ready。
- dependency 全部 Accepted。
- 没有 current Assignment。
- 已声明 output scope 时不存在冲突。
- 调用者有 coordinator 权限；`self` 可用于单 Agent、DM 或 Room coordinator 自己承担工作。

Room member Assignment 在同一事务创建 Assignment 与 Dispatch outbox；DM self Assignment 与 Room member Assignment 都创建 pending root Agent Attempt。当前 Agent 之后选择 subagent 且责任与 correlation 可精确命中时，`PreToolUse: Agent` 在父 Attempt 上原子创建并绑定 running child Attempt；否则仅形成 runtime-only 子图。managed 事务提交前不得把 child launch 宣称为已绑定，也不得发送 Room wake。

结构化 Room Assignment 成功后，由 outbox 自动投递；模型不需要再手写 `@member` 才让分工生效。

### 10.4 自动 Attempt 激活

不向模型暴露 `start_work`、`mark_running` 或 `complete_attempt`。普通 self work 在首个相关执行动作时由 runtime 自动 claim；Room root Attempt 只有在 target slot 的 runtime query 真正被接受后才从 pending 原子变为 running，单纯排队、handoff admission 或 SQL outbox delivery 都不能伪造“已开始”；managed 子智能体由 `PreToolUse: Agent` 创建后端持有的 launch binding，`SubagentStart/Stop` 只校验或补充实际 runtime evidence，runtime-only 子智能体不写 Attempt。

### 10.5 `submit_work`

提交结果、引用和证据。成功后 Work Item 进入 submitted，Assignment 等待其 `return_to_agent_id` 选定的 reviewer；该 reviewer 可以与 owner 相同。

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

请求把当前 transient Execution 绑定到新 Goal。Agent 决定 persistence 是否值得；服务端只重新校验身份/权限、用户配置、current Execution、Goal 冲突、objective/completion boundary 与剩余工作。

模型只提供可选 objective proposal 与 activation reason。adapter 注入当前 Execution identity/revision，后端从权威 snapshot 读取 objective、completion criteria 和 Plan，并检查配置开关、当前 Goal 冲突与用户 authority。Room dependency、外部等待、恢复成本、上下文边界和复杂度都是可参考理由，不是必须被后端证明的白名单。proposal 不能扩展 objective，也不能创建第二份 Plan。

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
- UI 可以把 runtime-local Task 作为对应 Work Item 的局部步骤展开，但必须由当前 WorkAttempt 的 `executor_agent_id + agent_round_id` 与消息精确关联；缺少任一身份或不匹配时不展示，不能按 Agent、消息位置或时间猜测。
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
3. 事务提交后，幂等 outbox consumer 先用 lease/CAS claim Dispatch，再按显式 `target_agent_id` 写入带完整 binding 的 Room handoff ledger 和 durable target input queue；该投递可以在人类可见消息中投影为定向 `@Agent`，但目标与责任只取 Dispatch，正文和 `@` 不参与权威解析。
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

Room Agent 调用 `submit_work` 时，一个 SQL transaction 写入 immutable Submission 与 append-only event；仅当 `return_to_agent_id` 不同于 owner 时，同一事务再写入 `ExecutionReviewDispatch` outbox。review target 来自 Assignment 创建时已经规范化的 `return_to_agent_id`，可以是 owner、Lead 或另一名当前 Room member。

事务提交后，独立 consumer 使用 lease/CAS claim、retry 和 receipt，把待审 Submission 幂等写入 target 的 durable Room handoff 与 input queue。该回交携带 `execution_id/plan_id/work_item_id/spec_id/assignment_id/submission_id/review_dispatch_id/target_agent_id` 的完整 `review_binding`；它不携带 worker Attempt，也不伪造 reviewer Assignment/Attempt。

reviewer slot 只有在服务端重新校验 Execution、active Plan、current Assignment、未审 Submission、review target 与 outbox binding 后才会启动。其 actor context 直接投影 `pending_reviews` 并开放 `review_work`。因此正确性不依赖 worker 在正文中手写 `@Coordinator`；公开 deliverable 或 mention 仍用于真实内容沟通和人类可见性，但不创建 review truth，也不产生 Acceptance。

`review_work` 成功写入 Acceptance 后，新的 Ready 状态立即成为持久事实。若 reviewer 是 coordinator，当前物理 round 可转入 CoordinationBinding 并直接继续；若 reviewer 是 owner 或另一成员，实质性结果通过 Room 通信回到需要它的 Agent，而 reviewer 不自动获得额外协调权限。用户不需要查看或确认每个中间节点。只有没有可继续的工作、Agent 选择返工，或确实需要用户决策/输入/授权时，状态机才停在对应 blocker。

跨 Agent review 的 SQL transaction 原子边界止于 Submission 与 review-return outbox；自审事务只有 Submission，不制造自投递。Room workspace ledger 是另一持久化域，只能在 commit 后幂等投递；SQL outbox 是待投递真相源，稳定 dedupe key 防止重试生成第二条 handoff/queue。Execution terminal、Plan/Assignment replaced 或 Submission 已被 review 时，pending/claimed/failed return 被取消；已投递但迟到的 Room queue 在实际 wake 前也必须重新 admission 并拒绝，不能进入 successor Execution。

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
| PreToolUse: Agent | 精确责任可用时绑定 child Attempt；否则 runtime-only 放行；Plan Mode、错误授权或 managed 持久化失败才拒绝 |
| PreToolUse: execution mutation | 先做 authority、dependency、scope、version 检查 |
| TaskCreated | 绑定或投影 Nexus Work Item metadata |
| TaskCompleted | 在 SDK 状态写入前检查 Nexus submission/acceptance 规则 |
| SubagentStart | 精确 binding 可用时记录 managed runtime identity；否则只记录 runtime Graph，不猜写 Attempt |
| SubagentStop | 精确 binding 可用时结束 child Attempt；否则只关闭 runtime Node Run；均不自动创建 Submission |
| PostToolUse | 注入状态变化后的有界 Context |
| PostToolUseFailure | 记录 Attempt/command failure evidence，不自动 block Goal |
| SessionStart(source=compact) | 从 SQL snapshot 重新注入 Assignment、禁止重复执行范围和 blockers |
| Stop | 检查未完成 required work、running Attempt 和 submitted-unaccepted work |
| Room handoff admission | 检查 Assignment binding、依赖、revision、幂等 claim |
| Cancellation outbox consumer | lease/retry 绑定旧 Room slot/runtime round；仅当它仍是 session 唯一 running round 时发 provider interrupt，否则只做 exact local cancel 并记录 limitation |
| Goal completion service | 检查 WorkGraph、runtime quietness 和 usage settlement |

不能依靠 PreToolUse 检查普通 public `@`，因为 `@` 在 final assistant message 持久化后才解析；必须在 Room handoff admission 服务层检查。
`SubagentStart` 本身不能阻止启动；确定性的 Plan Mode、授权和 managed 状态写入门禁位于 `PreToolUse: Agent`。`SubagentStart` 只负责把实际 runtime identity 精确关联到已有 Attempt，关联不了就保留为 runtime-only 观测。

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
W3 Lead: 整合并交付最终报告草稿
    depends_on: W2
```

流程：

1. Lead 建立 shared Plan。
2. W1 为 Ready，W2/W3 为 Blocked。
3. Lead 分配并只激活 W1。
4. Researcher 拥有 W1；Lead 不重复生产同一份结果。
5. Researcher 提交 W1。
6. W1 由 Assignment 选中的 reviewer 验收；可以是 Researcher 自审、Lead 或另一成员。接受后 W2 变 Ready。
7. Lead 激活 Analyst 的 W2。
8. Analyst 提交，由选中的 reviewer 接受。
9. W3 变 Ready，Lead 通过 self Assignment 执行整合；该 Lead 节点和其工具/子智能体运行同样显示在工作图中。
10. Lead 可显式自审 W3，也可选择另一 reviewer；是否把它声明为 terminal 由 Lead 决定。
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

1. **同一 objective 改路线。** outcome 与 completion boundary 不变，只改变步骤、文案、owner、dependency、工具或 evidence strategy，继续使用当前 Execution；先 prepare `operation: replan` 的完整 document，再用 receipt 创建 immutable Plan revision。必要时声明 `supersede_active_work`；不得改写 Execution objective。
2. **Goal-bound objective revision。** 必须执行 6.5 的 durable rebase：先 prepare Goal transition 并预留 successor，terminalize 整个旧 WorkGraph并在同一 orchestration transaction 写入 exact-target Cancellation outbox，再 commit Goal revision；随后 `prepare_plan_execution` seal 绑定该 reserved successor/Goal fence 的 fresh graph，`plan_execution` 原子 materialize，Goal confirmation 由 durable pending saga 收口。旧 live Attempt 由独立 consumer可靠重试物理中断，旧 revision 的完成事件不能写入新 revision，任何 Work/Acceptance 都不自动 carry。Plan Document 的 `operation: replace` 与 `abandon_execution` 对 Goal-bound Execution 都返回 `goal_retarget_required`，不能把 Goal continuity 退化为 transient replacement/cancellation。
3. **不同 transient objective replacement。** 只有用户明确提供不同 non-Goal objective 时，coordinator 才能 prepare `operation: replace`、`replacement_reason`、新 objective/completion criteria 和完整 successor graph，再以 `proposal_id + proposal_digest` commit。一个 SQL transaction supersede 旧 Execution 并创建 successor Execution + active Plan；成员和 subagent 禁止调用。Plan Mode 可以保存 sealed proposal，但不能 materialize；旧 Accepted 结果不自动 carry。
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
| immutable non-authoritative Execution Plan Proposal / materialization receipt / Goal confirmation state | SQL |
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

所有 mutable aggregate 使用 optimistic version；每个 command 都携带调用方稳定的 idempotency command ID。Plan preparation 先写一条 immutable、non-authoritative proposal 及 exact digest fence；它与之后的 authoritative transaction 由 durable `sealed/materializing/materialized/blocked` receipt 连接。进入未完成 authoritative materializer 前还要通过 proposal version CAS 取得有界 claim lease；commit 后的恢复只认 stable materialization command 对应的 authoritative Plan event receipt，不用 graph 语义相等代替命令归因。普通首次 Plan 必须在一个 SQL transaction 内创建 Execution、完整 WorkGraph 与 active Plan；已有但尚无 active Plan 的 Execution 通过 exact empty base-Plan fence 的 replan transaction 原子写入首个 Plan。Room `submit_work` 必须在一个 SQL transaction 内创建 immutable Submission，并仅为跨 Agent review 同事务创建 review-return outbox；任何把 pending/running Attempt 置为 interrupted 的 mutation 必须先在同一 transaction 写入 per-Attempt Cancellation outbox；transient replacement 必须在一个 SQL transaction 内 terminalize 旧 Execution 并创建 successor Execution + active Plan；Goal revision rebase 必须先以 durable Goal transition 保留意图和 reserved successor，再分别幂等提交 old graph supersede、Goal revision commit、successor proposal/materialization 与 Goal binding confirmation。若 authoritative Execution/Plan commit 成功而 confirmation 失败，包括 gateway 没有 confirmer，proposal receipt 保存 pending 状态供 reconciler 重试。abandonment 在一个 SQL transaction 内只取消旧 graph。Room workspace ledger 只在 commit 后幂等 reconciliation，runtime physical interrupt 只在 commit 后由 cancellation consumer 投递；正确性由 proposal digest/target fence、SQL outbox、terminal state 与 admission fence 保证。

数据库必须保证：

- sealed Plan proposal 的 canonical document 不可变；同一 proposal 的 owner/session/scope/coordinator 与 Execution/version/base Plan/Goal ID/revision/activation/reserved successor/predecessor fence 必须 exact-match，materialization command 与 reserved identity 稳定，重复 recovery 不产生第二个 Execution/Plan。
- Goal-free create proposal 也要在 seal 和 commit 时重新解析 trusted Goal state；ambient Goal 的出现、消失或变更只能使该 proposal stale，不能在 materializer 内临时绑定。
- materializing proposal 每次恢复前必须用 expected version CAS claim 过期 lease；未到期或 CAS 失败的调用不进入 authoritative transaction。
- proposal materialization receipt 必须来自它的 stable command 写入的 exact authoritative Plan event；语义相同的当前 Plan 不能代替该 receipt。materialization receipt 与 Goal confirmation state 分开持久化；`materialized + confirmation=pending` 可跨进程重试 confirmation，但不可回退为可编辑 proposal 或再次创建图。
- 已有 planless Execution 的首个 Plan 必须以 empty base-Plan exact fence 原子激活，且不能声称复用任何旧 Work Item identity。
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

- 已用同一 runtime graph/read model 投影单 Agent、DM 和 Room 工作图。
- 默认只显示 Agent、Subagent、关键 Gate 与折叠后的 Tool group；边只表达方向，详情点击节点再看。
- Lead/creator 的 coordinate、integrate、review、takeover 和 delivery run 与成员节点使用同一语义。
- UI 不定义新状态，也不要求用户逐节点确认。

## 20. 必须通过的行为场景

1. 单个原子任务不创建多余 Plan 或 Goal。
2. 多步单 Agent 任务建立 Plan，但可在单轮完成而不保留 Goal。
3. 明显跨轮任务从 transient Execution 原子提升为 Goal。
4. compact 前后不重复创建 Plan、Assignment 或 subagent。
5. 两个独立 Work Item 可由不同 Room Agent 并行；同一 parent Agent 也可在同一物理 Room round 内，通过各自唯一的 `tool_use_id` correlation 运行多个 active child subagent。
6. 有依赖的 Work Item 在上游 Accepted 前不能启动。
7. 两个 Work Item 都声明相同 exclusive output scope 时不能并行；未声明 scope 不使 Plan 无效。
8. 显式 review/verify 可以读取被审查 Work Item 的相同结果。
9. subagent stop 只终结精确 child Attempt；父 Agent 整合并显式 Submission，选定 reviewer 验收后才 Accepted。
10. Room Lead 可以拥有自己的 Work Item、在其中使用 subagent，并选择自审或独立 reviewer；它不自动复制另一个 owner 已承担的交付。
11. 一条含两个 `@` 的消息只启动两个 conversation round，不启动 `Researcher → Analyst` 的任何 Assignment；依赖链只由 structured Dispatch 推进。
12. Room member 不能修改 sibling Work Item 或完成 shared Goal。
13. member Submission 被任一选定 reviewer reject 后可以返工并保留历史 Attempt。
14. Agent failure 后 takeover 不产生两个 current Assignment。
15. 迟到的旧 owner Submission 不覆盖新 Assignment。
16. Goal objective revision 变化会拒绝 stale Plan/Attempt mutation。
17. Goal completion 在 required Work Item 未 Accepted 时被拒绝。
18. Goal completion 在 running subagent、pending handoff 或 usage 未结算时被拒绝。
19. 服务重启后 pending Assignment/handoff 可幂等恢复。
20. Plan Mode 可以通过 `prepare_plan_execution` 写入 durable non-authoritative sealed proposal，也可以验证 abandonment；不能 materialize Execution/Plan、开始执行、改变 Goal/Dispatch/Room ledger 或自动续跑。
21. 无论是否存在 managed Execution，裸 `@` 都只做 conversation transport，不创建、不激活 Work Item/Assignment，也不继承 source binding。
22. 同一 command 或重复 Hook delivery 不产生第二个 Assignment、Attempt、Acceptance 或 event sequence。
23. dependency、Assignment、Attempt、Submission 不能跨 Execution、Plan 或 Spec chain 写入。
24. Submission 不可变，reject/accept 只能新增 Acceptance decision；reject 后返工产生新 Submission。
25. Plan 的任何文本、Spec 或 dependency 修改都产生新 revision，历史快照不可改写。
26. Agent 可以根据跨轮、外部等待、Room dependency、恢复成本或任务复杂度自适应提出 Goal；这些是推荐证据，不是后端工作流白名单。
27. Goal promotion 只在用户禁用、Plan Mode、身份/权限冲突、已有冲突 Goal、Execution 非 current/active，或需要新增授权时被后端拒绝。
28. `SessionStart(source=compact)` 恢复同一 SQL snapshot；`PostCompact` 不被当成持久状态注入。
29. 普通首次 Plan 先 seal `operation: create` 的完整 document，再由 `plan_execution(proposal_id, proposal_digest)` 原子创建 Execution、完整 WorkGraph 与 active Plan；失败时不留下 planless Execution。
30. 同一 objective 只改变路线时必须 prepare `operation: replan`；试图用 `operation: replace` 表达相同 boundary 被拒绝，不创建 successor Execution。
31. coordinator seal 不同 transient objective 的 `operation: replace` proposal 后，commit 在同一 SQL transaction 完成旧 Execution supersede 与 successor Execution + active Plan creation；旧 Accepted 结果不进入 successor。
32. `abandon_execution` 只取消当前 transient graph，不创建 successor；重复 command 幂等。
33. Goal-bound Execution 的 replacement/abandonment 返回 `goal_retarget_required`，Room member 与 subagent 调用返回 authority rejection。
34. Plan Mode 的 create/replan/replacement 可以新增 proposal SQL row，但 Execution、Plan、Goal、Dispatch outbox 与 Room ledger 均不变化；abandonment 仍只验证且不写 SQL。
35. replacement/abandonment commit 后的迟到 Room handoff、queued slot、child launch 或 mutation result 被旧 Execution/WorkBinding fence 拒绝，即使 workspace reconciliation 尚未完成。
36. MCP、HTTP 与 app-server 的 managed Goal objective mutation 都进入同一个 retarget coordinator；Room 模型非 Lead 被拒绝，HTTP metadata 不能覆盖 server-owned binding/lead/transition。
37. Goal rebase 在 transition prepare、old graph supersede、Goal commit、successor proposal/materialization 或 binding confirmation 后失败时，使用同一 command/receipt 重试会复用同一 transition 与 successor，并只修复缺失阶段；materialized-but-confirmation-pending 由 reconciler 收口。
38. `prepared | awaiting_plan | binding_reserved` 或 Goal confirmation pending 时不能 complete 或自动 continuation；模型在 `prepared` 获得 `retarget_goal` retry，在 awaiting-plan 阶段先获得 `prepare_plan_execution`，sealed 后再获得 `plan_execution` next action。
39. Goal revision successor 只有在 predecessor 已按同一 transition supersede、typed predecessor/revision/scope 连续且首 Plan 同事务创建时才接受；任意 terminal predecessor 或不同 reserved ID 被拒绝。
40. Goal revision successor 的 Plan Mode preparation 只写 non-authoritative proposal SQL，对 Goal metadata、Execution、Plan、outbox 与 Room ledger 零 mutation；旧 Work/Submission/Acceptance 不自动 carry。
41. Plan active-work supersede、block、takeover、transient replacement、abandonment 与 Goal supersede 在把 Attempt 写为 interrupted/cancelled 前，均在同一事务写入每个逻辑 Attempt 的 exact-target Cancellation dispatch。
42. cancellation consumer 在 delivery 失败或进程崩溃后可回收 lease 并重试；同一 row 不产生第二个逻辑 cancellation。
43. sole Room/DM runtime round 的 provider interrupt 保存 `provider_interrupted`；同 session 已有另一 running round时只取消 exact local context，保存 `local_round_cancelled` 与 limitation，不调用 shared provider/session interrupt。
44. provider interrupt 的唯一性检查与调用之间禁止 successor admission；延迟的 Room/DM cancellation 只命中捕获时的 WorkBinding/runtime round，旧 target 已结束或 stale 时幂等收束，不中断同 session、同 Agent 的 successor。
45. pending Attempt 不调用 runtime 并落为 `not_required/not_started`；running Attempt 缺 exact identity 或 exact local/provider 能力时落为 `unsupported`，不伪报 physical interruption。
46. 同一 Room 同时存在 active Execution 时，无 binding 的用户定向消息、裸 `@` 和 directed message 仍可正常聊天；non-coordinator member 不获得 Execution MCP，直接 mutation 返回 `conversation_only`。
47. Room `review_work` 接受 review-return outbox 注入的 exact ReviewBinding、Assignment 选定 self reviewer 的 exact WorkBinding，或当前 coordinator 的有效 CoordinationBinding；其他成员不能越权验收。
48. input queue/guidance 不改变 target round lane，普通消息链路不会复制 WorkBinding/ReviewBinding；跨 Agent Assignment 与 review lane 只能由各自 durable outbox 创建，自审沿当前 WorkBinding 完成。
49. active Execution 存在时，普通 Room coordinator round 仍从 conversation 开始；读取或 prepare proposal 本身不 mint CoordinationBinding，跳过 `get_execution` / successful proposal materialization 直接 `assign_work`、takeover、promotion 或 abandonment 返回 `conversation_only`。显式转换后同一物理 round 可协调，round 结束 capability 被释放。
50. internal Goal continuation 只有 exact Goal ID/revision/Execution ID 三元组与 SQL Execution/coordinator 全部匹配时直接进入 coordination；任一字段缺失会在 claim 前失败，ambient Goal、旧 revision、错误 Execution 和普通聊天不能获得 Goal mutation authority。
51. 旧 parent round 的迟到 SubagentStart/Stop 不会路由给 successor callbacks；复用 SDK Agent identity 或缺少唯一 correlation 时 fail closed。
52. Work Dispatch 命中 stale/terminal Execution、superseded Plan/Spec 或已释放责任时只取消旧 outbox，绝不 reopen old Work；只有 current active Plan/Spec 下、尚未启动且仍为 `assigned` 的永久 target/root Attempt failure 才同事务 release/cancel 并让当前 Work Item Ready；running Attempt不被 Dispatch consumer 中断，Room/runtime 暂时不可用仍 retry。
53. 带 WorkBinding/ReviewBinding 的 queue item 不能通过 generic queue control 改成 guidance、删除、重排或合并；每条 durable directed message 独立 dispatch。
54. `get_execution` 在没有 current Execution 时返回 unmanaged conversation context，不创建空 Execution；prepare 只 seal proposal，完整 proposal 被 `plan_execution` 成功 materialize 后才进入 coordination。
55. Goal-bound Work/Review round 的 Goal ID/revision 只由后端从 exact Execution binding 解析，消息正文和 binding payload 不能伪造或采用 ambient revision。
56. 父 round结束时 runtime/service/storage 共同强校验并持久化精确 `T+30s` child grace deadline；迟到 Stop仍终结原 child Attempt，终态永久缺失时独立于 Room realtime 的启动/每秒恢复器跨进程将其置为 interrupted，后续 context投影 terminal `subagent_result` 而不自动 Submission/Acceptance。
57. Plan Document 的空输入、`{}`/placeholder、未知或重复 key、multi-document、anchor/alias/tag/merge、资源超限或无效 DAG 在 seal 前被稳定拒绝；不会创建 proposal 或任何 authoritative graph state。
58. sealed proposal 跨 round、compact 与进程重启仍可用同一 ID+digest commit；document、target Execution/version/base Plan、Goal activation、reserved successor 或 predecessor 任一改变都会 exact-fence 拒绝并要求重新 prepare。
59. Goal-free create proposal 在 seal 后若出现 ambient Goal，或 sealed Goal 在 commit 前消失/改变，materialization 被拒绝且不产生 authoritative Execution/Plan 或 Goal binding mutation。
60. 服务在 authoritative Plan transaction commit 后、proposal receipt 更新前崩溃时，reconciler 只有在 stable command 对应的 exact `plan_activated` event 存在时才恢复原 Plan ID；单纯语义相同的 Plan 不能证明归因。
61. materializing proposal 的首次尝试持有有界 lease；前台重放、后台 reconciler 和多实例只有在 deadline 到期后成功完成 expected-version CAS claim 的一方能重入 authoritative command。
62. 已有 planless Execution 可以用 `operation: replan` 和 empty base-Plan fence 原子创建首个 Plan；document 携带任何 `existing_work_item_id` 都被拒绝。
63. Goal-bound Plan materialization 成功但 confirmation 暂时失败或 confirmer 未实现时，proposal 持久化 `materialized + confirmation=pending`；Goal completion/continuation fail closed，reconciler 只重试 confirmation。

## 21. 非目标

本文当前不规定：

- 用 LLM 文本相似度自动猜测所有语义重复；Work Item 必须有 deliverable，output scope 只在 Agent 希望后端保护冲突时声明。
- 用 Execution Orchestration 替代 Room public/private 通信协议。
- 用 Goal 替代 Automation 的定时、重复或日历调度。
- 允许 Goal continuation 获得超出用户原始请求的新权限。
- 将子智能体提升为持久 Room 成员或独立用户关系主体。
