# Execution Graph 协议

## 1. 文档目标

本文定义 Nexus 如何把单 Agent Loop、SDK Task、子智能体、工具调用、Room Agent 通信以及 Execution WorkGraph 组合成一张可执行、可恢复、可观察的分层工作图。

它回答：

- 单 Agent 为什么也可以拥有工作图。
- WorkGraph 与 Agent 内部 Loop 分别负责什么。
- Agent、Subagent、Tool、Gate 哪些应成为主图节点。
- Hooks 如何把真实 runtime 事件绑定为 Node Run 与 Edge Run。
- 模型如何只读取当前工作所需的图状态，而不把回复浪费在流程播报上。
- 图开始运行后，后端如何保证继续调度、检查点、失败恢复和明确阻塞。
- 前端如何用头像、图标、状态环和方向箭头展示最重要的信息。

本文不替代 [Execution Orchestration 协议](./execution-orchestration-spec.md)。后者定义 Goal、Plan、Work Item、Assignment、Attempt、Submission 与 Acceptance 的责任状态机；本文定义这些责任在实际运行时如何展开为 Agent、Subagent、Tool、Gate 和消息流。

## 2. 稳定产品规则

面向用户和模型的稳定规则是：

> Goal 绑定跨执行边界持续存在的目标；WorkGraph 组织共享责任和依赖；Agent 在节点内部自主 Loop；Task 管理节点内局部步骤；Subagent 提供独立子上下文；Tool 和 Gate 提供确定性动作与检查；Hooks 把真实运行投影成 Graph Run；后端调度器保证图持续运行；UI 只展示最重要的节点、方向和状态。

更短的产品表述是：

> Agent 决定怎么做，Graph 负责把它稳稳地跑完。

Prompt 只能解释这些能力的适用边界，不能把模型编排成一条固定脚本。模型选择直接执行、Task、Subagent、Goal、Plan 或 Graph 扩展；后端只执行身份、权限、状态、显式依赖、检查点和恢复等确定性规则。

硬边界与自主选择必须分开：

| 后端硬约束 | Agent 自主选择 |
| --- | --- |
| 身份、Room membership、用户授权和副作用审批 | 是否建图、建 Goal 或直接做 |
| exact binding、幂等、CAS、不可变历史 | 用 Task、Subagent、Room member 或自己执行 |
| 已声明 dependency 的 readiness、已声明 scope 的冲突 | 是否声明 dependency/scope、是否并行 |
| event correlation、stale run fence、状态一致性 | 是否自审、独立 review、加 Gate 或再跑一轮 |
| 系统预算、取消和基础设施容量 | 采用何种 Loop 策略以及何时收口 |

Graph 是 Agent 实际决策的可恢复记录，不是要求 Agent 服从的预制流程模板。

## 3. 一张图，两层语义

Nexus 对外只展示一张 Execution Graph，但后端必须区分两个层次。

### 3.1 外层：责任与路由 Graph

外层描述：

- 哪些工作具有独立交付边界。
- 哪些节点可以并行。
- 哪些输出必须经过验证或审批。
- 哪个 Agent 对哪个 Work Item 负责。
- 状态从哪个节点流向哪个节点。
- 失败后从哪里恢复。

Execution WorkGraph 仍是共享责任的 DAG。Work Item dependency 只表达真实的交付依赖，不用于记录 Tool Result 返回 Agent、Subagent 返回父 Agent 或验证失败后的重试回边。

### 3.2 内层：Agent Loop

每个 Agent 或 Subagent 节点内部都可以自主运行：

```text
observe -> reason -> act/tool -> observe -> adjust -> finish/block
```

Tool Result 回到 Agent、Subagent Result 回到父 Agent、校验失败后的再执行属于 runtime control edge。它们可以形成有界回边，但不能被写进 WorkGraph dependency DAG。

### 3.3 分层关系

```text
Execution Graph
├── Work Item group
│   └── Agent node
│       ├── Task list
│       ├── Tool runs
│       └── Subagent subgraph
├── promoted Tool / Gate node
└── Agent-to-Agent message and handoff edges
```

没有 Plan 的单 Agent 对话仍可产生 runtime Graph：根 Agent 是第一个节点，Tool、Task 与 Subagent 运行附着其下。存在 Plan 时，同一套 runtime Graph 再按 Work Item 分组，并叠加共享依赖和验收状态。

## 4. 可执行图模型

Nexus 使用四部分描述图：

```text
G = (V nodes, E edges, S state, P policy)
```

### 4.1 V：节点

节点是拥有明确输入、输出和生命周期的执行单元。节点定义与某次实际运行必须分离：

- `GraphNode` 是稳定逻辑节点。
- `NodeRun` 是一次不可变执行实例。
- 同一节点可以产生多次 Node Run。
- 重试、Loop iteration 和恢复不得覆盖旧 Run。

### 4.2 E：边

边描述可执行路由和数据方向。Graph definition 中可以存在直通、条件、扇出、扇入和有界回边；每次实际穿越形成不可变 `EdgeRun`。

UI 默认只画方向箭头，不在边上显示文字。edge kind、条件、消息引用和 state mapping 只在点击详情时出现。

### 4.3 S：状态

Graph State 是节点之间传递的 typed state，不是完整聊天历史。它至少可以引用：

- objective 与 completion boundary；
- current Work Item、Assignment 与 Attempt；
- accepted upstream outputs；
- Artifact、消息、测试、外部事实和其他 evidence refs；
- Task 与 child run 摘要；
- budget、deadline、retry 和 loop counter；
- blocker、approval 与 checkpoint；
- ready、active、pending delivery 和 terminal nodes。

大内容保留在 Room/DM history、workspace Artifact、runtime transcript 或领域数据库中，Graph State 只保存稳定引用和有界摘要。

### 4.4 P：策略

Policy 描述：

- 谁能创建、扩展或替换节点；
- 哪个 Agent 可以执行当前节点；
- 当前节点可以挂载哪些工具能力；
- 哪些工具存在副作用或需要审批；
- 哪些边由代码自动穿越，哪些边需要模型判断或用户确认；
- retry、timeout、budget、loop bound 和 failure escalation；
- 哪些状态可以进入下一个节点。

Policy 中的硬部分只来自后端事实和用户配置，不按模型名称建立白名单。Agent 选择策略可以由 Skill 推荐，但不得变成隐藏准入门禁。Provider adapter 只负责把各 runtime 的事件规范化为同一 Graph 协议。

## 5. 节点分类与视觉层级

### 5.1 AgentNode

表示一个持久 Agent 在当前 Execution 中承担的一次独立工作责任。

- 使用 Agent 头像作为主视觉。
- node identity 不等于 Agent ID；同一个 Agent 可以在不同 Work Item 或不同运行阶段出现多个节点。
- 点击后展示当前工作、Task、Tool Runs、Artifacts、消息和历史 Attempts。
- Agent 的自然语言输出应是实际工作内容，不是状态播报。
- Lead/creator 也必须有自己的 Agent Node Run，用于 coordinate、integrate、review、takeover 或 delivery；不能只把它画成图外的分配者。

### 5.2 SubagentNode

表示父 Agent 在自己的责任内启动的独立子上下文。

- 使用子智能体头像和轻量分支标记。
- 使用 child task/session、SDK Agent 或 Attempt 的稳定 identity 生成自己的头像，不借用父 Agent 头像。
- 必须绑定 exact parent Node Run 和可用的 child/tool identity；存在 managed Assignment 时再绑定父 Work Item/Attempt，没有时保留为 runtime-only 子图。
- 可以展开自己的 Agent Loop 与 Tool Runs。
- 子智能体完成只返回父 Agent，不自动完成 Work Item、Submission、Acceptance 或 Goal。

### 5.3 ToolNode

所有 Tool Call 都必须被记录为 Tool Run，但默认不全部成为主图节点。

只有具备下列至少一种明确语义的工具步骤才提升为可见 ToolNode：

1. 产生多个节点消费的 canonical Artifact 或 evidence。
2. 决定后续路由、解锁条件或 completion。
3. 产生外部副作用，例如发布、写生产数据或发送外部消息。
4. 需要独立 retry、timeout、approval、checkpoint 或恢复。
5. 是确定性验证步骤，例如测试、schema 校验、预算计算或权限检查。
6. 在 Graph definition 中被 Agent 或用户显式声明为独立步骤。

普通搜索、网页读取、局部文件读取和一次性查询保留在 Agent 详情中。相同 logical tool 的连续调用可以折叠为一个图标和次数徽标，但每次 Tool Run 仍有独立身份与输入输出引用。

### 5.4 GateNode

Gate 是可供 Agent 判断路线的独立检查或人工停点。只有 active、blocking 或被显式选中的 Gate 才出现在默认主图：

- deterministic gate：测试、格式、权限、预算、状态 invariant；
- semantic verifier：独立上下文中的验收或 objective alignment；
- human gate：用户审批、授权或补充输入。

非阻塞的内部检查只作为 Tool Run，不额外占据主图。

自审通常折叠在同一个 Agent 节点中，以 Submission → Acceptance 状态变化表示；只有 Agent 明确选择独立 reviewer、人工审批，或检查结果会改变路由时，才额外显示 Gate/Reviewer 节点。后端不要求每份结果都拉起另一个 reviewer。

### 5.5 WorkItemGroup

Work Item 是节点分组和共享交付边界，不是 runtime node kind。

- 它可以包含一个 Agent，也可以包含 Agent、Subagent、Tool 与 Gate。
- `parent_work_item_id` 只表达包含关系，不能形成 readiness edge。
- 只有 `dependency_ids` 形成跨 group 的硬依赖箭头。

### 5.6 非节点对象

- Goal：Graph 的长期 objective envelope。
- Plan：当前 WorkGraph definition revision。
- Task：Agent/Subagent 节点内部的局部步骤。
- Message：节点之间的数据流记录。
- Submission/Acceptance：节点或 Work Item 的交付与验收状态。
- Artifact：节点输出引用。
- Loop：一组 Node Run 与有界回边，不是单独节点。

## 6. 边与数据流

后端保留 edge kind，前端默认只显示方向：

| kind | 语义 |
| --- | --- |
| `dependency` | 下游 Work Item 不能在上游 Accepted 前开始 |
| `dispatch` | Assignment/Graph scheduler 启动目标 Agent Node |
| `tool_call` | Agent/Subagent 调用 Tool |
| `tool_result` | Tool observation 返回调用者 |
| `spawn` | Agent/Subagent 创建 child Subagent |
| `child_result` | child result 返回 exact parent run |
| `message` | Agent 之间通过 Room/DM 协议传递内容 |
| `handoff` | 消息或产物被绑定为正式工作交接 |
| `condition_true` / `condition_false` | Gate 选择后续路径 |
| `loop_back` | 受 guard、次数、预算和 deadline 约束的回边 |
| `fan_out` / `fan_in` | 并行分支的启动和汇合 |

`@`、Room directed message、parent-child message、tool result 都是 transport 或 source metadata，不改变 Graph edge 的基本方向语义。普通聊天消息可以形成可观察 message edge，但不自动升级为 dependency、handoff、Submission 或 Acceptance。

## 7. Graph Definition、Run 与 View

### 7.1 Graph Definition

Graph Definition 保存稳定拓扑和策略引用：

```text
GraphNode {
  id
  kind
  work_item_id?
  actor_id?
  parent_node_id?
  input_contract
  output_contract
  policy_ref
  visibility
}

GraphEdge {
  id
  from_node_id
  to_node_id
  kind
  condition_ref?
  state_mapping?
  policy_ref?
}
```

`visibility` 使用稳定三档：

- `primary`：默认显示在主图。
- `nested`：展开父节点或 Work Item 时显示。
- `detail`：只在节点详情和运行时间线显示。

模型不能通过自然语言把一次普通工具调用伪装成 primary node。显式 Graph mutation 或后端确定性 promotion rule 才能改变 visibility。

### 7.2 Graph Run

每次执行创建：

```text
NodeRun {
  id
  node_id
  parent_run_id?
  work_attempt_id?
  agent_round_id?
  tool_use_id?
  child_session_id?
  iteration
  status
  input_state_revision
  output_state_revision?
  started_at
  finished_at?
}

EdgeRun {
  id
  edge_id
  from_run_id
  to_run_id?
  state_revision
  message_ref?
  artifact_refs?
  status
}
```

现有 WorkAttempt 是 Agent/Subagent responsibility run 的领域真相；Tool/Task/Message runtime 事件不得另造第二套责任状态。Graph Run 通过稳定 identity 引用现有对象。

### 7.3 ExecutionGraphView

前端读取的 `ExecutionGraphView` 是确定性投影：

```text
WorkGraph responsibility
+ current/historical WorkAttempts
+ normalized Tool/Task/Subagent hooks
+ Room/DM message routes
+ artifacts and gates
= one hierarchical Execution Graph view
```

View 不是写入格式。前端不得从节点位置、DOM 顺序、头像或消息文本反推业务状态。

## 8. Hook 事件协议

Hooks 的职责是观察、绑定、校验和更新 Graph Run，不负责替 Agent 发明工作拆解。

| canonical event | identity requirements | Graph effect |
| --- | --- | --- |
| `agent_run_started` | session + agent round + actor | 创建/恢复 Agent Node Run |
| `pre_tool_use` | parent run + tool use id + tool name | 创建 Tool Run 与 call edge，执行 Policy admission |
| `post_tool_use` | exact tool use id | 关闭 Tool Run，写 output refs，返回 Agent |
| `post_tool_use_failure` | exact tool use id | 记录失败和可恢复事实，把路线选择留给 Agent 或显式 policy |
| `task_created/updated/completed` | task id + owning run | 更新节点内部 Task，不创建主图节点 |
| `subagent_started` | parent run + tool use id + child identity | 创建 nested Subagent Node Run 与 spawn edge |
| `subagent_message` | child/parent run + message id | 记录 parent-child message edge |
| `subagent_stopped` | exact child identity | 关闭 child run，记录 child result edge |
| `agent_message_delivered` | source/target agent run + message id | 记录 Agent-to-Agent message edge |
| `attempt_terminal` | exact WorkAttempt | 关闭 responsibility run，不隐式 Acceptance |
| `checkpoint_committed` | graph/state revision | 建立 durable recovery boundary |
| `graph_blocked/resumed` | blocker/gate identity | 更新 liveness 状态和恢复动作 |

### 8.1 Provider-neutral normalization

- SDK、Bridge、Claude、GLM 和其他 Provider 必须进入同一 canonical event adapter。
- 不按模型名开启或关闭 Graph 能力。
- 精确 identity 可用时记录完整 Tool/Subagent 图。
- 某 runtime 缺少细粒度 Hook 时，可以从 assistant/tool result stream 恢复 detail-level Tool Run；不能伪造不存在的 identity。
- detail 缺失不能破坏 WorkGraph、Assignment、Attempt 和调度正确性。

### 8.2 Hook 失败边界

- 可观察性 Hook 失败不能让已经成功的业务 Tool 被重复执行。
- Policy/admission Hook 失败必须 fail closed，并返回明确 reason code。
- terminal Hook 必须可幂等重放。
- parent round 退出后，已创建的 child binding 仍保留到 child terminal 或 reconciliation deadline。

## 9. 模型 Graph Context

模型不读取前端图，也不读取完整 Graph Snapshot。后端为当前 Node Run 生成有界 `graph_context`：

```text
objective
current_node
current_work_item_and_assignment
input_contract
output_contract
accepted_upstream_outputs
current_tasks
active_children
recent_tool_failures
real_blockers
budget_and_deadline
allowed_capabilities
recommended_next_primitives
```

### 9.1 上下文分层

- coordinator：获得完整 topology digest、ready/active/blocked groups 和关键 state refs。
- Agent worker：只获得当前节点、必要上下游和自己的 child runs。
- Subagent：只获得父节点委托、必要输入和 child-local tools。
- reviewer/gate：只获得被审结果、标准和独立 evidence，不继承生产者的长推理历史。

### 9.2 输出纪律

Graph status 通过 Execution Event 更新 UI。模型 final output 必须优先交付：

- 实际发现、分析、代码或决策；
- Artifact 与 evidence；
- 无法继续时的真实 blocker 和所需输入。

“已经分配”“正在等待”“下一步将由某 Agent 执行”等状态不得要求模型反复写入聊天正文。系统可以显示轻量状态 chip，但不能把状态播报伪装成 Agent 工作结果。

## 10. Tool capability 与 ToolNodeBinding

基础 Tool schema 在所有 Provider 间保持稳定。Graph 不动态重写嵌套参数结构，而是在当前节点上附加 `ToolNodeBinding`：

```text
ToolNodeBinding {
  tool_name
  purpose
  input_state_mapping
  output_state_mapping
  side_effect_class
  retry_policy
  approval_policy
  visibility
}
```

当前 Node 可以收到相关工具的优先提示：

- Researcher 优先获得检索、网页、文档和引用工具。
- Coder 优先获得文件、终端、测试和诊断工具。
- Reviewer 优先考虑读取、测试、校验工具；写入仍服从用户授权和副作用审批。
- Coordinator 获得 Goal、Plan、Assignment、消息和 Graph 调整能力。

推荐集合不是模型白名单。除用户权限、运行环境或副作用审批明确禁止外，Agent 仍可调用完整可用能力；系统只通过排序、Skill 和上下文摘要减少工具过载。不同 Provider 获得相同 schema 与权限语义。

## 11. Goal、Task、Subagent 与 Graph 选择边界

模型静态指导只解释适用条件，不强制每个任务创建结构。

### 11.0 触发不是场景匹配，而是正交决策信号

系统不维护“复杂任务用 Plan”“多人用 Room”“步骤多用 Goal”这类用例白名单。Agent 从直接执行开始，只在某一种结构的表达价值高于它的协调成本时增加该结构。七个选择维度彼此独立：

| 决策信号 | 它回答的问题 | 通常增加的结构 | 不能据此自动触发 |
| --- | --- | --- | --- |
| local-state pressure | 当前 Agent 是否容易遗漏局部步骤，或用户是否需要看到节点内进度？ | Task/Todo | 步骤数量本身 |
| context-fork value | 隔离上下文、专业注意力或局部并行的收益是否高于合并成本？ | Subagent | 想让图更丰富 |
| responsibility boundary | 是否出现了可独立交付、需要明确 owner/验收/接管的责任？ | Work Item + Assignment | Room 中有人被 `@` |
| topology value | 显式依赖、并行分支、恢复点或整合顺序是否值得持久化？ | Plan/WorkGraph | 任务被描述为复杂 |
| persistent identity | 是否需要另一持久 Agent 跨轮拥有责任并可见交接？ | Room Assignment；消息仍用 `@`/directed message | 讨论、投票、一次性建议 |
| evidence branch value | 一次检查的结论是否会实质改变交付、返工、等待或升级路线？ | Gate；需要重复时形成 Loop | 每一步都做形式化审核 |
| continuity risk | objective 是否必须跨 round、上下文、外部等待、中断、预算或高恢复成本继续存在？ | Goal | Plan、Room、Subagent 或参与人数本身 |

这些信号不是互斥分类：单 Agent 可以使用 WorkGraph；一个 Work Item 内可以同时有 Task 和多个 Subagent；Room 可以纯聊天而没有 Plan；简单任务也可能因外部等待需要 Goal；复杂任务若能在当前 boundary 内稳定完成则不需要 Goal。

模型侧的选择原则是 `minimum sufficient structure`：只增加当前任务真正需要的语义。后端可以投影 observed boundary、Ready Work、外部等待、冲突或恢复成本等建议事实，但不能把建议事实写进 `allowed_actions` 伪装成流程权限，也不能自动创建 Goal、Plan、Assignment、Gate 或下一轮 Loop。运行事件随后把 Agent 的真实选择投影进图。

### 11.1 直接 Agent Loop

适用于：一个连贯目标、一个主要上下文、明确停止条件、不需要独立交接或审查。普通对话和原子任务默认从这里开始。

### 11.2 Task

适用于：当前 Agent 需要记录局部步骤、检查遗漏或展示节点内部进度。Task 不创建新上下文、所有权、Work Item 或 Graph dependency。

### 11.3 Subagent

适用于至少一种情况：

- 大量旁支信息需要隔离，保护父上下文；
- 子任务可独立并行；
- 需要不同专业角色、模型注意力或工具集合。

父 Agent 仍拥有当前 Work Item 和最终整合责任。

### 11.4 WorkGraph / Execution Graph

适用于责任或拓扑需要成为可恢复事实：独立交付、真实依赖、并行分支、交接、Gate、独立恢复或跨 Agent 可见协作。WorkGraph 不是 Room 专属；单 Agent 在拓扑价值足够时也可以使用。参与人数本身不触发 Graph；普通 Room 聊天也不触发 Plan。

### 11.5 Goal

适用于 objective 需要跨当前 round、上下文、预算、外部等待、中断或恢复边界继续存在。复杂度、Room dependency 和恢复成本都可以成为 Agent 的判断依据；后端可以给出建议，但只按权限、用户配置和当前状态决定工具是否可调用。

### 11.6 Room

Room 连接多个持久 Agent 节点。`@` 与 directed message 是 Agent-to-Agent 通信协议，可以形成 message edge；只有可信 Assignment/Handoff binding 才形成共享责任交接。

## 12. 调度、检查点与持续运行

### 12.1 Liveness invariant

一个 `running` Execution 必须始终满足至少一种状态：

1. 存在 active Node Run；
2. 存在 durable pending dispatch/retry；
3. 存在明确 blocker、Gate 或等待中的外部事件；
4. 正在提交 terminal transition。

若四者皆无，Execution 是 orphaned running state。reconciler 必须记录原因并可靠唤醒当前责任 Agent 或 coordinator 作决定，不能静默等待用户发送“继续”；除非用户或显式 policy 已经授权，reconciler 不得替 Agent 猜测下一节点或把 objective 判成失败。

### 12.2 Super-step

一次 super-step 包含当前可并行运行的一组 Ready 节点。结束时：

- 成功输出先持久化为 pending writes；
- 失败节点不会抹掉同 super-step 已成功节点的输出；
- Graph State revision 与 checkpoint 原子推进；
- scheduler 只兑现 Agent 已选择或确定性 policy 已声明的边，不从自然语言猜测语义路线；
- Room Agent、DM Agent 或 child runtime 通过 durable queue/outbox 被唤醒。

### 12.3 Loop safety

每条实际回边必须绑定明确 guard 和新的 Node Run/state revision。Agent 还应根据风险选择：

- 明确 guard；
- iteration counter；
- token/time/cost budget；
- max attempts 或 deadline；
- exit、blocked 和 escalation path；
- 每轮独立 Node Run 与 state revision。

系统级 token/time/cost budget 与取消能力始终有效，但后端不要求每个短 loop 都填写同一套策略字段。模型不能靠重复自然语言把同一个 WorkAttempt 伪装成新的 loop iteration。

### 12.4 Recovery

- 从最后成功 checkpoint 恢复，不从用户原始消息重跑整张图。
- Tool/Agent/Subagent terminal event 可幂等重放。
- 已成功 pending writes 不因 sibling failure 被重跑。
- 迟到事件只允许命中 exact Node Run/Attempt/Graph revision。
- replacement、abandonment 与 Goal retarget 后，旧事件不能进入 successor graph。

## 13. 前端投影

### 13.1 默认主图

默认主图采用克制、图标优先的视觉：

- Agent：头像；
- Subagent：较小头像和分支关系；
- promoted Tool：工具图标；
- active/blocking Gate：菱形或审批图标；
- current status：边框、状态环或角标；
- repeated Tool/Loop runs：数字徽标；
- data flow：只有箭头和连线，不显示边文字。

主图不展示 objective、deliverable、criteria、Tool input/output 或消息正文。

### 13.2 节点详情

点击节点后再显示：

- 当前任务与 Work Item；
- 输入/输出 contract；
- Task 列表；
- Tool Run 时间线；
- Subagent 子图；
- Artifact、消息和 evidence；
- retry、failure、blocker 与 Gate 结果；
- Node Run 历史和 loop iterations。

### 13.3 分层展开

- 顶层：Work Item groups 与 primary nodes。
- 点击 Agent：展开其 Tool/Subagent Loop。
- 点击 Subagent：展开 child Loop。
- 点击次数徽标：展开重复 Tool Runs 或 loop iterations。
- 点击边：展示 route kind、message/artifact refs 和 state mapping。

### 13.4 单 Agent 与 Room

DM/单 Agent 和 Room 使用同一组件：

- 单 Agent 无 Plan 时显示 root Agent 与其 Tool/Subagent Loop。
- 单 Agent 有 Plan 时按 Work Item 分组。
- Room 显示多个 Agent 节点，通过 message/handoff edges 连接。
- 托管 WorkGraph 中，带 Coordination binding 的创建者/Lead 通过 `dispatch` 边进入首个责任节点；带 Work/Review binding 的 runtime Agent 合并回对应 Work Item/Gate，不重复生成平行头像。
- 无 Work/Review/Coordination binding 的普通 Room reply root 不混入托管 WorkGraph；它仍属于 Conversation activity，不能为了视觉连续而伪造依赖边。
- casual Room chat 可以显示轻量 message activity，但不能被误投影成 managed WorkGraph progress。

## 14. 示例

### 14.1 单 Agent 调研

```text
[Agent avatar] -> [search icon] -> [Agent avatar]
       |                               |
       v                               v
[Subagent avatar] -> [tool icon] ------+
```

主图可折叠为一个 Agent 头像、一个搜索次数徽标和一个 Subagent 头像；全部 Tool Runs 在详情中保留。

### 14.2 Room fan-out / fan-in

```text
                   -> [Researcher avatar] -
[Lead avatar] ----> [Analyst avatar] ------> [Lead avatar]
                   -> [Writer avatar] -----
```

Agent 应只对真实独立分支 fan-out；后端严格执行已声明 dependency 和 scope，但不从自然语言猜测隐含依赖。Lead 不必等待无关分支，结果通过 Room message/handoff 与 typed state refs 汇合。

### 14.3 Tool/Gate promotion

```text
[Coder avatar] -> [test icon] -> [Verifier avatar] -> [deliver]
       ^                                |
       +--------------------------------+
```

测试作为确定性 ToolNode，因为它决定路由并产生 canonical evidence。普通文件读取仍留在 Coder 详情。Verifier 失败时形成有界回边，每次返回新的 Node Run。

## 15. 与现有实现的迁移边界

### 15.1 保留

- Execution/Plan/Work Item/Assignment/Attempt/Submission/Acceptance 状态机。
- WorkGraph hard dependency readiness。
- Room/DM message、handoff、input queue 与 runtime session 真相源。
- SDK Task/Todo 的局部进度语义。
- Subagent child Attempt 与 exact parent binding。

### 15.2 修正

- `parent_work_item_id` 从 readiness/layout edge 中移除，只作为 group containment。
- Execution Graph UI 不再把每个 Work Item 渲染成一张文字密集卡片。
- Agent/Subagent Attempts 成为可见运行节点。
- Task 保留在节点详情，不冒充 WorkGraph 节点。
- 状态消息从 Agent final output 移到 Execution Event/UI projection。
- Agent runtime 在 Tool Result/child result 后继续自己的 Loop；WorkGraph scheduler 在 Acceptance、dispatch 和显式 graph mutation 后推进 Ready 工作，不依赖用户逐步确认。

### 15.3 新增

- canonical Graph Run event model；
- Tool Run 与 promoted ToolNode projection；
- Gate、checkpoint、super-step 与 liveness reconciliation；
- actor-specific `graph_context`；
- 图标优先的分层 Execution Graph View。

## 16. 验收不变量

1. 没有 Plan 的单 Agent 使用 Tool/Subagent 时也能形成 runtime Graph。
2. WorkGraph dependency 保持 DAG；Tool/child/loop back edge 不进入 readiness evaluator。
3. `parent_work_item_id` 不增加节点深度或等待关系。
4. 所有 Tool Call 可追踪，但普通 Tool 默认不占主图节点。
5. ToolNode/GateNode promotion 由显式 definition 或确定性规则决定，不解析模型自述。
6. 同一 Agent 在不同 Node Run 中拥有不同 run identity。
7. Subagent 节点绑定 exact parent/tool/task/session，不按时间或 Agent 名猜测。
8. Task 只附着 exact owning Agent round。
9. `@` 形成 message edge，但不自动形成 Assignment、Submission 或 Acceptance。
10. Graph 状态更新不要求 Agent 在聊天正文复述流程。
11. running Execution 不允许无 active/pending/blocker/terminal 的静默状态。
12. Tool Result、child result 会恢复 exact parent Agent Loop；Acceptance 后新的 WorkGraph Ready 状态可被持续调度，不要求用户发送“继续”。
13. checkpoint 后恢复不重跑同 super-step 已成功节点。
14. loop 每轮都有独立 Node Run、guard 与退出原因；可选 loop policy 与系统预算共同限制风险。
15. Claude、GLM、OpenAI 和其他 Provider 通过 capability/event normalization 使用同一协议，不按模型白名单分叉。
16. UI 默认只显示头像、重要工具/Gate 图标、状态环和方向；详情按点击展开。

## 17. 非目标

- 不把所有对话和 Tool Call 强制升级为 managed Execution。
- 不要求简单任务创建 Goal、Plan 或多 Agent Graph。
- 不把 Mermaid/DOT 文本作为写入协议或状态真相源。
- 不用前端布局反推 dependency、parent、Attempt 或消息身份。
- 不让 Hook 替模型决定语义拆分。
- 不把子智能体提升为持久 Room 成员。
- 不用更多 Agent 数量衡量 Graph 质量。
- 不让模型通过状态播报维持调度器活性。
