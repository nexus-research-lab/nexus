# Codex /goal 实现调研

## 1. 文档目标

本文档分析 OpenAI Codex 开源仓库中 `/goal` 的当前实现方式。

调研对象：

- 仓库：`openai/codex`
- 快照：`464ab40dfa1fd5058ea52512c29f38d2e4f6b204`
- 提交时间：2026-05-22

本文档只描述 Codex 当前实现，不讨论 Nexus 应该如何实现。

## 2. 总体结论

Codex 的 `/goal` 不是一个简单的 slash command，也不是把目标追加进普通对话上下文。

它由六部分组成：

- 一个 thread 级持久目标
- 一个用户可操作的 `/goal` 命令入口
- 一组 app-server JSON-RPC API
- 一组暴露给模型的 goal 工具
- 一个 runtime 续跑调度器
- 与 rollout、compaction、resume 配合的长程上下文恢复机制

核心思路是：

> 把“目标”作为 thread 状态持久化。每轮工作结束后，如果目标仍然 active，runtime 在空闲时自动注入隐藏 continuation prompt，再启动下一轮普通 turn。

所以 Codex 的长程任务不是单次超长模型调用，而是多个普通 turn 的自动串联。

## 3. 功能开关

`/goal` 由 `Feature::Goals` 控制。

源码中对该 feature 的说明是：

- persisted thread goals
- automatic goal continuation

该 feature 当前状态是 stable，并且默认启用。

关键源码：

- `codex-rs/features/src/lib.rs`
  - `Feature::Goals`
  - key 为 `goals`
  - `default_enabled: true`

这说明 `/goal` 在 Codex 里被视为稳定能力，不是纯实验性命令。

## 4. 用户入口

### 4.1 `/goal` 命令形态

TUI 里 `/goal` 支持几种用户动作：

- `/goal`
  - 查看当前 goal
- `/goal <objective>`
  - 设置或替换当前 objective
- `/goal edit`
  - 编辑当前 goal
- `/goal pause`
  - 暂停当前 goal
- `/goal resume`
  - 恢复当前 goal
- `/goal clear`
  - 清除当前 goal

对应逻辑在：

- `codex-rs/tui/src/chatwidget/slash_dispatch.rs`
- `codex-rs/tui/src/app/thread_goal_actions.rs`
- `codex-rs/tui/src/chatwidget/goal_menu.rs`

### 4.2 目标长度限制

objective 有长度限制：

- 最大 4000 字符
- 为空时报错
- 超长时提示用户把更长说明放到文件里，再在 `/goal` 里引用文件

关键代码：

- `MAX_THREAD_GOAL_OBJECTIVE_CHARS = 4_000`
- `validate_thread_goal_objective`

这能避免用户把大量规格内容直接塞进 goal 状态里。Codex 鼓励 goal 保持为目标描述，而不是完整上下文存储。

### 4.3 临时 session 不支持 goal

Goal 依赖持久化 thread。临时 session 或未 materialized 的 thread 不支持 goal。

TUI 对这类错误有专门提示：

- goals need a saved session
- run `codex` to start a saved session
- or `codex resume` / `/resume` to reopen one

这说明 goal 的语义和 thread 持久化绑定，而不是绑定当前进程内存。

## 5. 持久化模型

### 5.1 数据表

Codex 在 SQLite 中维护 `thread_goals` 表。

表结构核心字段：

- `thread_id`
  - 主键
  - 一个 thread 只有一个当前 goal
- `goal_id`
  - 当前逻辑 goal 的唯一 ID
  - 用于防止并发更新打到旧 goal
- `objective`
  - 用户给出的目标文本
- `status`
  - 当前状态
- `token_budget`
  - 可选 token 预算
- `tokens_used`
  - 已计入该 goal 的 token 用量
- `time_used_seconds`
  - 已计入该 goal 的耗时
- `created_at_ms`
- `updated_at_ms`

关键源码：

- `codex-rs/state/migrations/0029_thread_goals.sql`
- `codex-rs/state/migrations/0033_thread_goal_stopped_statuses.sql`
- `codex-rs/state/src/model/thread_goal.rs`

### 5.2 状态枚举

当前 goal 状态包括：

- `active`
  - 正在进行
- `paused`
  - 用户暂停
- `blocked`
  - 模型确认被外部条件阻塞
- `usage_limited`
  - 使用额度或硬限制阻止继续
- `budget_limited`
  - 达到 goal 的 token budget
- `complete`
  - 目标完成

注意这里有两个很重要的边界：

- `blocked` 和 `complete` 是模型可以通过工具设置的终止性状态
- `paused`、`usage_limited`、`budget_limited` 由用户或系统控制

这避免模型自行暂停、恢复或篡改预算状态。

### 5.3 插入、替换和更新

GoalStore 提供几类操作：

- `get_thread_goal`
  - 读取当前 thread 的 goal
- `insert_thread_goal`
  - 只在当前 thread 没有 goal 时插入
  - 如果已有 goal，返回 `None`
- `replace_thread_goal`
  - 替换当前 thread goal
  - 会生成新的 `goal_id`
  - 用量归零
- `update_thread_goal`
  - 更新 objective、status、token budget
  - 支持 `expected_goal_id` 防止并发错写
- `delete_thread_goal`
  - 清除当前 goal
- `account_thread_goal_usage`
  - 累计 token 和耗时
  - 必要时把 status 改成 `budget_limited`

关键源码：

- `codex-rs/state/src/runtime/goals.rs`

### 5.4 budget_limited 的系统判定

当 `account_thread_goal_usage` 累计 token 后，如果：

- 当前状态允许计量
- `token_budget` 不为空
- `tokens_used + token_delta >= token_budget`

则状态会自动变为 `budget_limited`。

这不是模型判断，而是持久化层在写入 usage 时做的系统状态转换。

## 6. App-server 协议层

### 6.1 JSON-RPC 方法

Codex app-server 暴露以下 goal API：

- `thread/goal/set`
  - 创建或更新单个 persisted goal
- `thread/goal/get`
  - 获取当前 goal
- `thread/goal/clear`
  - 清除当前 goal

对应通知：

- `thread/goal/updated`
- `thread/goal/cleared`

关键文档：

- `codex-rs/app-server/README.md`

关键实现：

- `codex-rs/app-server/src/request_processors/thread_goal_processor.rs`

### 6.2 set 流程

`thread_goal_set_inner` 的主要步骤是：

1. 检查 `Feature::Goals` 是否启用
2. 解析并校验 `thread_id`
3. 获取该 thread 对应的 state DB
4. 确认 thread 是 materialized thread
5. 对 rollout 做 reconcile，保证 sqlite 元数据和 rollout 文件一致
6. 如果 thread 正在运行，先通知 runtime 准备 external mutation
7. 校验 objective 和 token budget
8. 根据是否已有 goal 决定 update 还是 replace
9. 必要时用 objective 填充空 thread preview
10. 发送 JSON-RPC response
11. 按顺序发 `thread/goal/updated` 通知
12. 如果 thread 正在运行，把 external set 的 runtime effect 应用回 Session

这个流程说明 app-server 不是只改数据库。对运行中的 thread，它还要同步 runtime 内存状态，避免持久化状态和当前 turn accounting 脱节。

### 6.3 clear 流程

`thread_goal_clear_inner` 的主要步骤是：

1. 检查 feature
2. 找到 materialized thread
3. reconcile rollout
4. 如果 thread 正在运行，先让 runtime 结算当前 goal usage
5. 删除 DB 中的 goal
6. 如果删除成功，清理 runtime goal 状态
7. 发 response
8. 发 `thread/goal/cleared`

清理 goal 前先 accounting，是为了避免用户在运行中清 goal 时丢失已经发生但还没 flush 的用量。

## 7. 模型可见工具

### 7.1 工具列表

Codex 把 goal 能力作为 Responses API function tools 暴露给模型：

- `get_goal`
- `create_goal`
- `update_goal`

关键源码：

- `codex-rs/core/src/tools/handlers/goal_spec.rs`
- `codex-rs/core/src/tools/handlers/goal/create_goal.rs`
- `codex-rs/core/src/tools/handlers/goal/update_goal.rs`
- `codex-rs/core/src/tools/handlers/goal/get_goal.rs`

旧的 extension 版实现也存在：

- `codex-rs/ext/goal/src/spec.rs`
- `codex-rs/ext/goal/src/tool.rs`
- `codex-rs/ext/goal/src/extension.rs`

两套代码形态不同，但语义基本一致。

### 7.2 `create_goal`

`create_goal` 只应在用户或系统明确要求创建 goal 时调用。

工具约束：

- required: `objective`
- optional: `token_budget`
- 如果当前 thread 已经有 goal，调用失败
- 不允许模型从普通任务中自行推断并创建 goal
- token budget 只有用户明确要求时才设置

这意味着普通用户请求不会自动变成持久 goal。Goal 是显式模式，不是默认任务模式。

### 7.3 `get_goal`

`get_goal` 读取当前 thread goal。

返回内容包括：

- objective
- status
- token budget
- tokens used
- elapsed time
- remaining tokens

它给模型一个低成本方式确认当前目标和预算状态。

### 7.4 `update_goal`

`update_goal` 只能设置：

- `complete`
- `blocked`

不能设置：

- `active`
- `paused`
- `usage_limited`
- `budget_limited`

工具说明里明确要求：

- 只有目标真的完成，才能设为 `complete`
- 只有同一个阻塞条件连续多个 goal turn 都无法推进，才能设为 `blocked`
- 不允许因为预算快耗尽或模型准备停下来就标记完成
- 不允许把困难、不确定、慢当成 blocked

这部分是 `/goal` 成功的关键：Codex 不是只靠代码调度续跑，还通过工具说明约束模型的停止判定。

### 7.5 工具返回

Goal 工具返回结构包含：

- `goal`
- `remainingTokens`
- `completionBudgetReport`

当模型把带预算 goal 标记为 complete 后，返回会提示模型在最终回复里报告最终用量。

## 8. Runtime 生命周期

### 8.1 GoalRuntimeState

Session 内部有 `GoalRuntimeState`。

它维护：

- 当前 state DB handle
- budget limit 是否已经提示过
- accounting lock
- 当前 turn 的 accounting snapshot
- wall-clock accounting snapshot
- 当前 continuation turn id
- continuation semaphore

关键源码：

- `codex-rs/core/src/goals.rs`
- `codex-rs/core/src/session/session.rs`

### 8.2 事件分发模型

Codex 把 goal runtime 行为抽象成 `GoalRuntimeEvent`。

主要事件：

- `TurnStarted`
- `ToolCompleted`
- `ToolCompletedGoal`
- `TurnFinished`
- `MaybeContinueIfIdle`
- `TaskAborted`
- `UsageLimitReached`
- `ExternalMutationStarting`
- `ExternalSet`
- `ExternalClear`
- `ThreadResumed`

`Session::goal_runtime_apply` 根据这些事件统一处理：

- turn 开始时建立 accounting baseline
- 工具完成后计入用量
- goal 工具完成时特殊处理，避免重复 budget steering
- turn 结束或 abort 时 flush usage
- 外部修改 goal 前先结算
- resume 后恢复 active goal runtime 状态
- 空闲时尝试启动下一轮 continuation

### 8.3 turn start

turn 开始时：

1. 保存当前 token usage 作为 baseline
2. 如果当前 collaboration mode 是 Plan，忽略 goal
3. 打开 state DB
4. 读取当前 thread goal
5. 如果状态是 `active` 或 `budget_limited`，把该 goal 标记为当前 turn 的 active goal
6. wall clock 从此开始计时

Plan mode 被排除，说明 Codex 不希望计划模式触发自动长程执行。

### 8.4 tool completed

普通工具完成后：

1. 读取当前总 token usage
2. 和 turn baseline 比较，得到 token delta
3. 读取 wall clock delta
4. 调用 `account_thread_goal_usage`
5. 如果达到 token budget，状态变为 `budget_limited`
6. 发 `ThreadGoalUpdated` event
7. 必要时向当前 turn 注入 budget-limit steering prompt

`update_goal` 工具本身会走特殊路径，避免因为它刚把 goal complete/blocked，又触发普通工具完成后的预算提示。

### 8.5 turn finish / abort

turn 正常结束时：

- 再做一次 usage accounting
- 清理 continuation turn 标记
- 清理当前 turn accounting snapshot

turn abort 时：

- 也会尝试 accounting
- 清理 continuation turn 标记
- 清理 turn accounting

这保证中断不会轻易丢掉已经消耗的 goal 时间和 token。

### 8.6 usage limit

如果底层模型调用返回 hard usage limit，runtime 会：

1. 先结算当前 goal progress
2. 把 active goal 转为 `usage_limited`
3. 发送 `ThreadGoalUpdated`
4. 清除 active goal accounting

这和 `budget_limited` 不同：

- `budget_limited` 是用户给 goal 的 token budget 用完
- `usage_limited` 是外部服务或账户使用限制导致不能继续

## 9. 自动续跑机制

### 9.1 触发点

自动续跑入口是：

- `continue_active_goal_if_idle`
- `GoalRuntimeEvent::MaybeContinueIfIdle`
- `maybe_continue_goal_if_idle_runtime`
- `maybe_start_goal_continuation_turn`

app-server 在 resume 后也会调用 continuation：

- 先发 goal snapshot
- 再让 core 尝试继续 active goal

### 9.2 续跑前置条件

Codex 只有满足以下条件才启动 continuation：

- `Feature::Goals` 启用
- 当前不是 Plan mode
- 没有 active turn
- input queue 没有下一轮待处理输入
- trigger-turn mailbox 没有待处理输入
- thread 不是 ephemeral
- state DB 可用
- 当前 thread 有 goal
- goal 状态是 `active`
- 启动前再次读取 DB，确认 goal id 未变化且仍然 active

这些条件解决几个问题：

- 不抢用户输入
- 不和当前 turn 并发
- 不在 Plan mode 擅自执行
- 不对旧 goal 误续跑
- 不对临时 thread 做不可恢复的长程任务

### 9.3 continuation turn 的构造

当 runtime 决定续跑时，会：

1. 构造 hidden goal context
2. 把它塞入 pending input
3. 创建一个新的默认 turn context
4. 标记该 turn 是 goal continuation turn
5. 调用 `start_task` 启动普通任务

从模型角度看，这仍然是一个普通 turn，只是用户输入不是用户手打的，而是 runtime 注入的 goal continuation prompt。

### 9.4 continuation prompt

续跑 prompt 来自：

- `codex-rs/core/templates/goals/continuation.md`

它包含几类内容：

- 当前 objective
- tokens used
- token budget
- remaining tokens
- 继续工作的行为规则
- 从当前 worktree 和外部状态取证
- 必要时使用 plan
- 不要把目标缩小成更容易完成的子集
- 完成前要做 requirement-by-requirement audit
- 只有真正完成才调用 `update_goal complete`
- blocked 需要连续多个 goal turn 满足严格条件

这个 prompt 直接解释了 Codex 长程任务的策略：

- goal 跨 turn 持续存在
- 单轮没做完就继续推进，不要改写成功标准
- 完成是证据驱动的
- blocked 是严格状态，不是普通失败或犹豫

## 10. 隐藏上下文注入

Goal continuation 不是普通用户消息，而是 `GoalContext`。

`GoalContext` 会渲染成：

```xml
<goal_context>
...
</goal_context>
```

它作为 runtime-owned contextual user fragment 注入模型输入。

关键源码：

- `codex-rs/core/src/context/goal_context.rs`

这里有一个安全细节：

- objective 被明确描述为 user-provided data
- prompt 要求模型把 objective 当成任务目标，不当成更高优先级指令
- objective 渲染时会做 XML escape

这样可以降低用户在 objective 里写伪系统指令时的注入风险。

## 11. 预算与用量统计

### 11.1 token delta

Codex 不直接用总 token usage 覆盖 goal usage，而是记录 delta。

每个 turn 有：

- turn start 时的 token baseline
- 当前 token usage
- last accounted token usage

每次 accounting 时计算：

- input tokens delta
- cached input tokens delta
- output tokens delta

goal token delta 的计算会排除 cached input tokens：

```text
input_tokens - cached_input_tokens + output_tokens
```

这说明 Codex 不把缓存命中的输入 token 全额算入 goal budget。

### 11.2 wall-clock delta

耗时使用 wall-clock accounting。

active goal 改变时，wall-clock baseline 重置。

每次成功 account 后，baseline 前移对应秒数，而不是简单设置为 now。

这样可以减少重复计时或遗漏计时。

### 11.3 budget-limit steering

当 goal 达到 token budget 且还在当前 turn 内时，runtime 注入 budget-limit prompt。

该 prompt 要求模型：

- 不要再开始新的实质性工作
- 尽快收尾
- 总结进展
- 说明剩余工作或 blocker
- 除非真的完成，否则不要调用 `update_goal complete`

模板：

- `codex-rs/core/templates/goals/budget_limit.md`

这使 budget limit 不是硬杀进程，而是让模型进行一次受控收尾。

## 12. 外部修改 goal 的 runtime 同步

用户可以在 UI 或 app-server 外部修改 goal。

对运行中 thread，Codex 会做三个动作：

1. `prepare_external_goal_mutation`
   - 修改前尽量结算当前 goal progress
2. 持久化新 goal 状态
3. `apply_external_goal_set`
   - 根据新状态更新 runtime accounting
   - 如果 objective 变化且当前有 active turn，注入 objective-updated prompt
   - 如果新状态是 active，空闲时尝试继续
   - 如果新状态是 paused/blocked/usage_limited/complete，清理 active accounting

这说明 Codex 支持运行中编辑 goal，不需要等当前 turn 结束。

## 13. Resume 后的行为

thread resume 后，Codex 会：

1. 恢复 thread runtime
2. 读取当前 goal
3. 如果 goal 是 active，恢复 wall-clock active 状态
4. 发 goal snapshot 给客户端
5. 在 replay/snapshot 顺序完成后，尝试 `continue_active_goal_if_idle`

如果 goal 是 paused、blocked、usage_limited、budget_limited 或 complete，则不会自动续跑。

TUI 对 paused、blocked、usage_limited 的 goal 会提示用户是否 resume。

这保证：

- active goal 可以跨 Codex 进程或 UI resume 后继续
- 非 active goal 不会被自动启动

## 14. Rollout 与 compaction

### 14.1 rollout

Codex 会把 session rollout 写成 JSONL。

rollout 文件用于：

- inspect
- replay
- resume
- thread metadata reconcile

关键源码：

- `codex-rs/rollout/src/recorder.rs`

Goal 的 DB 状态依赖 thread metadata，所以 app-server 在 set/clear 时会对 rollout 做 reconcile。

### 14.2 compaction

长程任务会自然遇到上下文窗口限制。Codex 的处理是 context checkpoint compaction。

compaction prompt 要求模型生成 handoff summary，包含：

- 当前进展和关键决策
- 重要上下文、约束、用户偏好
- 剩余工作
- 继续所需的关键数据、例子、引用

关键模板：

- `codex-rs/core/templates/compact/prompt.md`

### 14.3 replacement history

compaction 完成后，Codex 不是只保存一段 summary 文本。

它会构造新的 compacted history，并把它作为 `replacement_history` 写入 `CompactedItem`。

resume 时，rollout reconstruction 会从最新 surviving replacement-history checkpoint 开始，再重放后续 rollout items。

关键源码：

- `codex-rs/core/src/compact.rs`
- `codex-rs/core/src/session/rollout_reconstruction.rs`

这使长程 goal 能跨上下文窗口继续工作：

- goal 状态在 DB
- conversation 历史在 rollout
- 历史压缩点在 compaction replacement history
- continuation prompt 在 active goal 空闲时重新注入

## 15. 与普通任务的差异

普通任务：

- 用户发一条消息
- 模型执行一个或多个工具
- 模型回复
- turn 结束

Goal 任务：

- 用户显式设置 goal
- goal 被保存到 thread DB
- 模型每轮工作后不能随意缩小成功条件
- runtime 统计用量和时间
- 如果 goal 仍 active 且系统空闲，自动启动下一 turn
- 直到 goal complete、blocked、paused、usage_limited、budget_limited 或 clear

因此 `/goal` 的本质是 thread-scoped execution objective，不是普通 prompt 装饰。

## 16. 关键设计细节

### 16.1 单 goal 模型

Codex 每个 thread 只有一个当前 goal。

优点：

- 状态简单
- UI 简单
- runtime 判断简单
- 不需要目标调度优先级

代价：

- 不支持一个 thread 同时挂多个长期目标

### 16.2 goal_id 防并发错写

更新时可以带 `expected_goal_id`。

这避免：

- 旧 turn 还在运行
- 用户已经替换了 goal
- 旧 turn 又把状态写回旧目标

### 16.3 Plan mode 不自动续跑

Plan mode 被明确排除。

这体现一个产品边界：

- 计划模式用于澄清和设计
- goal continuation 用于执行推进

### 16.4 不抢用户输入

续跑前会检查 input queue 和 trigger mailbox。

如果用户已经输入了下一条消息，Codex 不会自动插队续跑。

### 16.5 启动前二次确认 DB

runtime 构造 continuation candidate 后，启动 turn 前再次读取 DB。

只有当：

- `goal_id` 仍一致
- status 仍是 active

才真正启动。

这避免 race condition。

### 16.6 budget_limited 可以收尾

达到 budget 后，Codex 不是立刻终止当前 turn。

它会：

- 标记状态
- 注入 budget-limit steering
- 要求模型收尾

这比直接 kill 更适合 agent 工作，因为用户需要看到当前进展和下一步。

### 16.7 complete 是证据声明

continuation prompt 强调 completion audit。

模型需要从当前文件、命令输出、测试、运行状态、PR 状态等证据证明目标完成。

这说明 Codex 把 `complete` 作为强状态声明，而不是“我觉得差不多了”。

## 17. 源码路径索引

上游源码关键路径：

- `codex-rs/features/src/lib.rs`
  - goal feature 开关
- `codex-rs/tui/src/chatwidget/slash_dispatch.rs`
  - `/goal` slash command 解析
- `codex-rs/tui/src/app/thread_goal_actions.rs`
  - TUI goal 操作
- `codex-rs/tui/src/chatwidget/goal_menu.rs`
  - goal 菜单和摘要展示
- `codex-rs/protocol/src/protocol.rs`
  - `ThreadGoal`、`ThreadGoalStatus`、objective 校验
- `codex-rs/state/migrations/0029_thread_goals.sql`
  - thread_goals 初始表
- `codex-rs/state/migrations/0033_thread_goal_stopped_statuses.sql`
  - blocked、usage_limited 等状态扩展
- `codex-rs/state/src/model/thread_goal.rs`
  - state 层 goal 模型
- `codex-rs/state/src/runtime/goals.rs`
  - goal DB 操作和 accounting
- `codex-rs/app-server/src/request_processors/thread_goal_processor.rs`
  - app-server goal API
- `codex-rs/core/src/tools/handlers/goal_spec.rs`
  - 模型工具定义
- `codex-rs/core/src/tools/handlers/goal/`
  - 模型工具实现
- `codex-rs/core/src/goals.rs`
  - runtime 生命周期、accounting、continuation
- `codex-rs/core/src/context/goal_context.rs`
  - 隐藏 goal context 注入
- `codex-rs/core/templates/goals/continuation.md`
  - 自动续跑 prompt
- `codex-rs/core/templates/goals/budget_limit.md`
  - budget limit 收尾 prompt
- `codex-rs/core/templates/goals/objective_updated.md`
  - objective 运行中更新 prompt
- `codex-rs/rollout/src/recorder.rs`
  - rollout JSONL 持久化
- `codex-rs/core/src/compact.rs`
  - compaction 与 replacement history
- `codex-rs/core/src/session/rollout_reconstruction.rs`
  - resume 时重建 history

## 18. 参考链接

- OpenAI Codex `/goal` 文档：<https://developers.openai.com/codex/use-cases/follow-goals>
- OpenAI Codex slash commands：<https://developers.openai.com/codex/cli/slash-commands>
- 上游仓库：<https://github.com/openai/codex>
- 调研快照：<https://github.com/openai/codex/tree/464ab40dfa1fd5058ea52512c29f38d2e4f6b204>

## 19. 一句话总结

Codex 的 `/goal` 是一套 thread 级长程执行机制：目标存在数据库里，模型通过受限工具声明完成或阻塞，runtime 在空闲时自动注入 continuation prompt 并启动下一 turn，rollout 和 compaction 负责跨重启、跨长上下文继续工作。
