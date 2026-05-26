# Nexus Goal 长程任务改造计划

本文基于 `docs/specs/codex-goal-implementation-analysis.md`，给出 Nexus 如果要对标 Codex `/goal` 能力的逐步改造计划。

目标不是照搬 Codex 的实现细节，而是在 Nexus 现有 Go 后端、session runtime、DM/Room、automation heartbeat 基础上，补齐同等产品能力：用户可以声明一个长程目标，模型可以在目标内持续推进、更新状态、自动续跑、受预算约束，并在中断、重启、恢复后保持可解释的进度。

## 1. 对标范围

需要覆盖的 Codex `/goal` 能力如下：

| 能力 | Codex 语义 | Nexus 目标语义 |
| --- | --- | --- |
| 显式目标入口 | 用户用 `/goal` 创建或更新当前 thread 的目标 | 用户可在 DM/Room session 上创建、查看、编辑、暂停、恢复、清除目标 |
| 单会话活动目标 | 一个 thread 同时只有一个 active goal | 初期一个 `session_key` 同时只有一个 active goal |
| 持久化目标状态 | goal 会随 thread 持久保存 | goal 按 `session_key` 持久化，服务重启后可恢复 |
| 模型可读目标 | 每轮请求注入当前 goal hidden context | runtime 每轮请求前注入 `<nexus_goal_context>` |
| 模型可改状态 | 模型通过工具标记完成或受阻 | 内置 tool 允许模型 `complete` 或 `blocked` 当前 goal |
| 用户可改状态 | 用户可改目标、清除目标或停止 | API/CLI/前端可暂停、恢复、编辑、清除目标 |
| 自动续跑 | 当前轮结束后，若 goal 未完成则继续唤起下一轮 | round 结束后，goal coordinator 判断是否自动发起 synthetic continuation |
| 预算限制 | 目标有 token/time 等预算，耗尽后停止 | 按 token budget 和运行时间限制进入 `budget_limited` |
| 使用量追踪 | goal 记录 token/time usage | 从 runtime 回执和轮次生命周期归集 usage |
| 中断与恢复 | 中断后 goal 仍保留，可恢复推进 | interrupt 只终止当前 round，不删除 goal |
| 长上下文处理 | 长任务跨 compaction/summary 延续 | 增加 goal checkpoint/summary，恢复时注入关键状态 |
| 外部状态同步 | 用户或系统改 goal 时触发 runtime 侧更新 | goal service 变更广播事件，running round 用 guidance 注入变更 |
| 可观测性 | goal status、token、自动续跑原因可追踪 | 增加事件、日志、指标和 debug endpoint |

非目标：

- 不重新引入旧 Python 运行链路。
- 不把 goal 设计成 scheduled task 的同义词。
- 不要求第一阶段就改完整前端；先把后端协议、状态和 runtime 行为做稳定。

## 2. Nexus 当前基线

当前代码里已经有几块可复用基础：

- `session_key` 已经是 DM/Room runtime 路由和持久会话的核心标识，适合做 goal 的作用域主键。
- `internal/runtime.Manager` 已经按 session 维护 client、running round、cancel、guided input，并提供 `StartRound`、`MarkRoundFinished`、`InterruptSession`、`SendContentToRunningRound` 等生命周期能力。
- DM 的 `HandleChat` 和 `roundRunner.run` 已经形成完整的请求、排队、runtime 执行、持久化、事件广播、下一条 input queue 分发流程。
- `runtime/guidance.go` 已经提供 PostToolUse guidance 注入能力，可作为 “running round 中 goal 被外部修改后注入上下文” 的直接参考。
- `internal/protocol/model_automation.go` 已有 `SessionTarget`、heartbeat、delivery mode、run status、wake mode 等模型，可以支撑后续 unattended goal 的唤醒，但不应该替代 goal 本身。
- `internal/service/automation` 已经有 heartbeat、scheduled task、execution sink、runtime state，这些适合做长程任务的后台调度层。

因此，Goal 应该被建模成 session 级执行目标；Automation 是触发器和后台唤醒机制。两者有关系，但不能混成一个概念。

## 3. 目标模型

### 3.1 核心定义

Goal 是绑定到一个 Nexus session 的长程执行目标：

- `goal_id`：全局唯一。
- `session_key`：Goal 所属 session。
- `objective`：用户声明的目标文本。
- `status`：当前状态。
- `token_budget`：可选 token 预算。
- `tokens_used`：累计 token 使用量。
- `elapsed_seconds`：累计运行时间。
- `last_round_id`：最近一次推进 goal 的 round。
- `blocked_count`：模型连续声明 blocked 的计数。
- `created_by` / `updated_by`：区分 user、model、system。
- `created_at` / `updated_at` / `completed_at`。

第一阶段约束：

- 一个 `session_key` 同时最多一个非终态 goal。
- goal 只能绑定到已经 materialized 的 session。
- Room 场景先绑定到具体 agent/member runtime session，不做全房间共享 goal。

### 3.2 状态机

| 状态 | 含义 | 允许来源 |
| --- | --- | --- |
| `active` | 目标正在推进或等待续跑 | user/system |
| `paused` | 用户暂停，系统不自动续跑 | user/system |
| `blocked` | 模型认为没有外部输入无法继续 | model/system |
| `budget_limited` | 预算耗尽，系统停止续跑 | system |
| `usage_limited` | 上游额度或 runtime usage 限制 | system |
| `complete` | 目标完成 | model/user/system |
| `cleared` | 用户清除目标 | user/system |

终态：`complete`、`cleared`。  
可恢复但不自动续跑：`paused`、`blocked`、`budget_limited`、`usage_limited`。  
只有 `active` 允许自动 continuation。

### 3.3 权限边界

- 用户/API 可以创建、编辑、暂停、恢复、清除 goal。
- 模型工具只能读取 goal，并把当前 goal 标记为 `complete` 或 `blocked`。
- 系统可以因为预算、usage、runtime 错误改变状态。
- 模型不能提升预算、恢复 paused goal、清除 goal 或修改所属 session。

## 4. 分阶段改造计划

### 阶段 0：边界确认

交付物：

- 明确 goal 只作用于 `session_key`，不直接作用于 automation job。
- 明确第一版只支持一个 active goal。
- 增加 feature flag，例如 `features.goals`，默认关闭或灰度开启。
- 确认 DM 和 Room 的 session 解析规则，保证 goal 不跨 agent/runtime 串线。

验收：

- 能用一页 ADR 说明 Goal、Scheduled Task、Heartbeat、Input Queue 的边界。
- 不触碰现有 scheduled task 行为。

### 阶段 1：协议模型

新增或调整：

- `internal/protocol/model_goal.go`
  - `Goal`
  - `GoalStatus`
  - `GoalCreateRequest`
  - `GoalUpdateRequest`
  - `GoalResponse`
  - `GoalEvent`
- `internal/protocol/event_*`
  - `goal_created`
  - `goal_updated`
  - `goal_status_changed`
  - `goal_cleared`
  - `goal_continuation_scheduled`

设计要点：

- `objective` 做长度限制，例如 4000 字符。
- `token_budget` 必须为正数，可为空。
- `session_key` 使用现有协议类型，不新增自由字符串解析规则。
- event payload 必须带 `goal_id`、`session_key`、`status`、`updated_by`、`reason`。

验收：

- `go test ./internal/protocol/...`
- 协议 JSON round-trip 测试覆盖状态和 request validation。

### 阶段 2：持久化

新增数据库表：

`session_goals`

| 字段 | 说明 |
| --- | --- |
| `id` | 内部主键 |
| `goal_id` | 外部 ID |
| `session_key` | session 作用域 |
| `objective` | 目标正文 |
| `status` | 状态 |
| `token_budget` | 可选预算 |
| `tokens_used` | 累计 token |
| `elapsed_seconds` | 累计运行秒数 |
| `last_round_id` | 最近 round |
| `blocked_count` | 连续 blocked 次数 |
| `created_by` | user/model/system |
| `updated_by` | user/model/system |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |
| `completed_at` | 完成时间 |

约束：

- `goal_id` 唯一。
- `session_key + non_terminal_status` 只能存在一个活动 goal。SQLite 可用应用层事务兜底。
- update 必须支持 optimistic concurrency，至少用 `updated_at` 或 `version`。

新增包建议：

- `internal/storage/goal`
  - `repository.go`
  - `model_goal.go`
  - `codec_goal.go`

验收：

- migration up/down 测试。
- 并发创建同一 session active goal 时，只有一个成功。

### 阶段 3：Goal Service

新增服务：

- `internal/service/goal`
  - `CreateGoal`
  - `GetGoal`
  - `UpdateGoal`
  - `ClearGoal`
  - `PauseGoal`
  - `ResumeGoal`
  - `MarkGoalComplete`
  - `MarkGoalBlocked`
  - `RecordUsage`
  - `ReconcileAfterRound`

职责：

- 校验 session 是否存在且归属正确。
- 统一状态机。
- 统一 usage 归集。
- 统一事件广播。
- 给 runtime 提供可渲染的 `GoalRuntimeSnapshot`。

与现有服务关系：

- 不把逻辑塞进 DM service 或 Room service。
- DM/Room 只在 round 生命周期里调用 goal service。
- automation 可依赖 goal service 查询待唤醒 goal，但不直接改 goal 内部字段。

验收：

- service 单测覆盖所有状态迁移。
- 模型工具尝试越权修改预算时被拒绝。
- 用户清除 goal 后，后续 round 不再注入 goal context。

### 阶段 4：HTTP、WebSocket 和 CLI 入口

建议 API：

- `POST /nexus/v1/goals`
- `GET /nexus/v1/goals/current?session_key=...`
- `PATCH /nexus/v1/goals/{goal_id}`
- `POST /nexus/v1/goals/{goal_id}/pause`
- `POST /nexus/v1/goals/{goal_id}/resume`
- `POST /nexus/v1/goals/{goal_id}/clear`

请求体里传结构化 `session_key`，避免在 URL 里重复实现复杂 session key 编解码。

CLI：

- `go run ./cmd/nexusctl goal create --session-key ... --objective ...`
- `go run ./cmd/nexusctl goal show --session-key ...`
- `go run ./cmd/nexusctl goal pause --goal-id ...`
- `go run ./cmd/nexusctl goal resume --goal-id ...`
- `go run ./cmd/nexusctl goal clear --goal-id ...`

WebSocket：

- 复用现有 session/channel event 通道，新增 goal event。
- 前端不需要轮询当前 goal。

验收：

- API e2e 覆盖 create/show/update/pause/resume/clear。
- WebSocket 能收到 goal 状态变化。
- CLI 输出可用于人工调试。

### 阶段 5：模型工具

需要给 runtime 暴露内置 goal tools：

- `nexus_goal.get_current_goal`
- `nexus_goal.mark_complete`
- `nexus_goal.mark_blocked`

工具权限：

- `get_current_goal`：只返回当前 session 的 goal snapshot。
- `mark_complete`：只能把当前 active goal 置为 `complete`。
- `mark_blocked`：只能把当前 active goal 置为 `blocked`，必须带原因。

接入方式：

- 优先复用现有 MCP/server builder 机制，让每个 runtime session 自动获得 goal tools。
- goal tool handler 通过 runtime context 中的 `session_key` 和 `round_id` 定位目标。
- tool 调用结果要进入 durable message 或 system event，方便审计。

验收：

- 模型在一轮里调用 `mark_complete` 后，round 结束不再自动续跑。
- 模型调用 `mark_blocked` 后，goal 状态可见且不会自动续跑。
- 模型不能操作其他 session 的 goal。

### 阶段 6：Runtime 生命周期接入

接入点：

- `HandleChat` 创建 round 前：读取 current goal snapshot。
- `runtime.ExecuteRound` 请求前：注入 goal hidden context。
- durable message 处理：记录 goal 相关 tool result 和 usage。
- `roundRunner.run` 结束后：调用 `ReconcileAfterRound`。
- `MarkRoundFinished` 后：由 goal coordinator 决定是否自动 continuation。
- interrupt/error 路径：刷新 usage 和最后状态，但不删除 goal。

需要新增：

- `GoalRuntimeSnapshot`
- `GoalContextRenderer`
- `GoalRoundObserver`
- `GoalContinuationCoordinator`

注意事项：

- continuation 必须在 input queue 之后调度，避免用户消息被 synthetic goal prompt 插队。
- 如果 round 结束时存在 pending permission / guided input / human input，不自动续跑。
- 如果用户在 round 运行中修改 goal，用现有 guidance hook 注入变更，而不是取消当前 round。
- 每次 continuation 发起前必须重新读取 goal，防止状态已被用户清除。

验收：

- active goal + assistant 未完成时，round 结束后自动发起下一轮。
- active goal + 用户新消息排队时，优先处理用户消息。
- running round 中用户暂停 goal 后，下一轮不再续跑。

### 阶段 7：Hidden Context 和 Continuation Prompt

每轮请求前注入：

```xml
<nexus_goal_context>
  <goal_id>...</goal_id>
  <status>active</status>
  <objective>...</objective>
  <tokens_used>...</tokens_used>
  <token_budget>...</token_budget>
  <instruction>
    Continue working toward this goal. If it is finished, call nexus_goal.mark_complete.
    If you cannot proceed without external input, call nexus_goal.mark_blocked with a concise reason.
  </instruction>
</nexus_goal_context>
```

Continuation prompt 类型：

- `continue_active_goal`：默认续跑。
- `goal_updated_while_running`：目标被用户更新后下一轮同步。
- `budget_near_limit`：预算接近耗尽时要求收敛。
- `resume_after_restart`：服务重启后恢复 goal。
- `blocked_resume`：用户恢复 blocked goal 后继续推进。

UI 层不要展示 synthetic prompt 为普通用户消息；应展示成 “Goal continuation” 或只更新状态。

验收：

- 日志和 transcript 可区分真实用户消息与 synthetic continuation。
- 前端不会把内部 prompt 错显示成用户发言。
- prompt 变更有 snapshot 测试。

### 阶段 8：预算、Usage 和限制状态

预算维度：

- token budget：第一优先级。
- elapsed time：第二阶段加入。
- round count：作为安全阀。
- continuation count：防止死循环。

usage 来源：

- runtime message/result 的 token usage。
- SDK 回调里的 usage metadata。
- 若某些 provider 没有 usage，则记录 unknown，并启用 round count 安全阀。

状态规则：

- `tokens_used >= token_budget`：进入 `budget_limited`。
- provider quota/rate limit：进入 `usage_limited`。
- 连续 N 次空结果或无进展：进入 `blocked` 或 system-blocked。
- 用户手动恢复后，状态回到 `active`，但不重置 usage。

验收：

- 小 token budget 下能稳定停止。
- quota error 不会无限 retry。
- budget 状态能被 API、CLI、WebSocket 看见。

### 阶段 9：Resume 和服务重启

短期：

- goal 持久化后，用户重新打开同一 session 时恢复可见状态。
- 如果 goal 是 `active`，用户发送任意消息后继续注入 goal context。
- 不在服务启动时自动唤醒所有 active goal。

中期：

- 引入 `goal_runtime_state`，记录 last active round、last heartbeat、next eligible continuation。
- 服务启动后扫描 active goal，但只对满足 unattended 条件的 goal 发起 heartbeat wake。
- 复用 automation 的 wake request / execution sink，避免新增一套后台运行器。

长期：

- 支持 “keep working in this session later” 类能力，goal 可以登记下一次 wake 时间。
- heartbeat 只负责唤醒和派发，goal service 负责判断是否应该继续。

验收：

- 服务重启后，goal 状态不丢。
- 重启期间正在跑的 round 能被标记为 interrupted/unknown，不会误判 complete。
- active goal 不会在启动瞬间批量打爆 provider。

### 阶段 10：Checkpoint 和长上下文处理

第一版可以先依赖 SDK session resume 和现有 memory/context 机制。要完整对标长程任务，需要增加 goal checkpoint：

新增表：

- `goal_checkpoints`
  - `checkpoint_id`
  - `goal_id`
  - `round_id`
  - `summary`
  - `open_items`
  - `completed_items`
  - `last_files_touched`
  - `created_at`

触发条件：

- 每 N 个 continuation。
- token usage 接近预算阈值。
- runtime 报 context too long。
- 用户手动请求 “总结当前 goal 进展”。

注入方式：

- 下一轮 hidden context 包含最近 checkpoint。
- checkpoint 不替代完整 transcript，但给模型一个稳定恢复锚点。

验收：

- 长 goal 经过多轮后，仍能准确说明已完成、未完成、阻塞点。
- context too long 后，不会直接丢 goal，而是生成 checkpoint 再恢复。

### 阶段 11：Automation 和 Heartbeat 集成

边界：

- Scheduled Task：按时间或事件触发一次任务。
- Heartbeat：为主智能体提供周期性或外部事件唤醒。
- Goal：约束一个 session 内长期目标的状态和推进策略。

集成路径：

1. 第一版 goal continuation 只在 session 已经活跃时自动续跑。
2. 第二版允许 goal 注册 wake request，由 heartbeat 在合适时间唤醒。
3. 第三版 scheduled task 可以创建或更新 goal，但不直接承担 goal 状态机。

示例：

- 用户说 “接下来持续把这个 issue 修完”：创建 session goal。
- 用户说 “明天早上继续检查”：创建 heartbeat wake，并保留同一个 goal。
- 用户说 “每天 9 点跑一次报告”：创建 scheduled task，不一定创建 goal。

验收：

- scheduled task 失败不会污染 session goal 状态。
- heartbeat 唤醒 active goal 后，依然通过 goal service 判断是否续跑。
- goal clear 后，相关未执行 wake request 会被取消或标记失效。

### 阶段 12：前端体验

最小界面：

- session header 显示 goal 状态 badge。
- goal panel 支持创建、查看、编辑、暂停、恢复、清除。
- transcript 中显示 goal status event。
- synthetic continuation 不显示成普通用户消息。

完整界面：

- 展示 token budget 和 usage。
- 展示最近 checkpoint。
- 展示 “为什么没有继续跑”：paused、blocked、budget_limited、usage_limited、waiting_user_input。
- Room 里明确显示 goal 属于哪个 agent/member session。

验收：

- 用户不需要看日志就知道 goal 为什么停了。
- 用户可以一键恢复 blocked/paused goal。
- 前端刷新后 goal 状态仍一致。

### 阶段 13：可观测性和审计

日志字段：

- `goal_id`
- `session_key`
- `round_id`
- `status_before`
- `status_after`
- `updated_by`
- `continuation_reason`
- `tokens_used`
- `token_budget`

指标：

- active goals 数量。
- continuation 成功/失败次数。
- goal 完成率。
- blocked/budget_limited/usage_limited 数量。
- 平均 rounds per goal。

审计：

- user/system/model 对 goal 的每次状态修改都写事件。
- tool 调用带 round id 和 caller。
- clear goal 不物理删除，避免丢审计链路。

验收：

- 线上排查时能回答 “这个 goal 为什么停在这里”。
- 能按 `goal_id` 串起 API、runtime、automation 日志。

## 5. 里程碑

| 里程碑 | 范围 | 结果 |
| --- | --- | --- |
| M1 | 协议、存储、service、API/CLI | 用户能创建、查看、编辑、清除 goal |
| M2 | 模型 tools、hidden context | 模型能读取 goal，并标记 complete/blocked |
| M3 | round lifecycle、usage 归集 | goal 能随 round 更新 usage 和状态 |
| M4 | 自动 continuation | active goal 能在同 session 内自动续跑 |
| M5 | budget、interrupt、resume | 预算耗尽、中断、重启后行为可控 |
| M6 | checkpoint、heartbeat 集成 | 支持更长周期和 unattended 长程任务 |
| M7 | 前端完整体验和观测 | 用户能理解、控制、排查 goal |

建议先做到 M4 再开放给内部使用。M5 之后才适合默认打开。

## 6. 验收矩阵

| Codex 能力 | Nexus 验收方式 | 所属里程碑 |
| --- | --- | --- |
| `/goal` 创建目标 | API/CLI 创建后 session 可查询 | M1 |
| 当前目标持久化 | 服务重启后 goal 仍存在 | M1/M5 |
| 目标注入模型上下文 | runtime 请求 snapshot 包含 `<nexus_goal_context>` | M2 |
| 模型标记完成 | tool 调用后状态为 `complete`，不再续跑 | M2 |
| 模型标记受阻 | tool 调用后状态为 `blocked`，等待用户恢复 | M2 |
| 自动继续工作 | active goal round 结束后发起 continuation | M4 |
| 用户消息优先 | input queue 中用户消息先于 continuation | M4 |
| 预算停止 | token budget 耗尽进入 `budget_limited` | M5 |
| 中断不丢目标 | interrupt 后 goal 仍可恢复 | M5 |
| 重启可恢复 | active goal 状态恢复，running round 有明确终态 | M5 |
| 长上下文延续 | checkpoint 生成并注入下一轮 | M6 |
| 定时/后台唤醒 | heartbeat 可唤醒 active goal | M6 |
| 可观测 | 日志和事件可解释 goal 停止原因 | M7 |

## 7. 风险和处理

### 7.1 自动续跑失控

风险：模型不主动 complete，导致无限 continuation。  
处理：round count、continuation count、token budget、empty-progress detection 四层安全阀。

### 7.2 用户消息被内部续跑抢占

风险：goal continuation 比真实用户消息先执行。  
处理：continuation 调度必须在 input queue drain 后，且发起前重新检查 queue。

### 7.3 Room 语义混乱

风险：Room 里多个 agent 或 member 共用 goal，导致目标归属不清。  
处理：第一版只绑定具体 runtime session；共享 room goal 单独设计。

### 7.4 Goal 和 Scheduled Task 概念混淆

风险：把 goal 做成定时任务，导致状态机和触发器互相污染。  
处理：Goal 管状态和目标；Scheduled Task/Heartbeat 只负责触发和唤醒。

### 7.5 Usage 不完整

风险：部分 provider 不返回准确 token usage。  
处理：usage 可标记 unknown，并用 round count/time limit 做保底预算。

### 7.6 Hidden Context 泄漏到用户界面

风险：synthetic prompt 或 goal XML 被显示为用户消息。  
处理：消息模型区分 user-visible message、internal instruction、system event。

## 8. 推荐实施顺序

1. 先写 `Goal` 协议和 service 状态机，不接 runtime。
2. 再接 API/CLI，保证人工可操作和可调试。
3. 接入模型 tools，但只允许 complete/blocked。
4. 接入 hidden context，让普通 round 能感知 goal。
5. 接入 round 结束后的 reconciliation 和 usage。
6. 打开同 session 自动 continuation。
7. 增加 budget、interrupt、resume 保护。
8. 最后接 heartbeat、checkpoint 和完整前端。

这个顺序的核心是先把 goal 做成可验证的 session 状态，再让 runtime 自动推进。只要状态机稳定，后面的 continuation、heartbeat、前端展示都可以渐进增强。
