# Nexus Goal 跨项目完整改造计划

本文面向两个当前真正相关的项目：

- `nexus-core`：Nexus 产品侧、Go 后端、前端、数据库、运行时编排。
- `nexus-agent-sdk-bridge`：Nexus 当前依赖的 Agent SDK facade，负责把 Nexus 运行时请求桥接到 Claude Code CLI。

`nexus-agent-sdk-go` 不在当前主链路里。它可以作为运行时设计参考，但不应该成为本轮 Goal 功能的直接改造目标，除非后续 Nexus 明确切换运行时 SDK。

## 目标

完整复现 Codex `/goal` 的产品能力，而不是只做一个前端标签或普通任务列表：

1. 每个会话最多有一个当前 Goal。
2. Goal 有明确状态机：创建、更新、暂停、恢复、完成、阻塞、清除。
3. Goal 状态持久化，服务重启、页面刷新、会话恢复后仍可继续。
4. 模型每轮都能看到当前 Goal 的隐藏上下文、预算和进度。
5. 模型可以通过受控工具把 Goal 标记为完成或阻塞。
6. 用户输入优先；没有用户输入且 Goal 仍 active 时，系统可自动续跑。
7. 续跑可被中断、可恢复、可观察，不影响普通 DM、Room、Automation。
8. 长程运行需要预算、checkpoint、摘要和失败退避，不能无限空转。

## 总体结论

Goal 的产品语义必须放在 `nexus-core`。Bridge 只应该补通用运行时原语，不应该出现 `Goal`、`goal_id`、`goal_status` 这类产品概念。

原因很直接：

- Goal 依赖 Nexus 的 session、room、message、storage、frontend、automation 和权限模型。
- Bridge 不拥有 Nexus 的数据库，也不应该决定一个会话是否还有未完成目标。
- Bridge 只知道如何向底层 Agent 发送输入、接收消息、注册工具、处理中断。
- 如果把 Goal 状态机塞进 Bridge，后续换运行时或接入其他 Agent 时会被产品语义绑死。

因此改造边界是：

- `nexus-core`：实现 Goal 的全部领域模型、状态机、API、DB、UI、续跑调度、预算和 checkpoint。
- `nexus-agent-sdk-bridge`：补齐 Goal 所需要的通用能力，例如 typed usage、terminal 分类、meta/synthetic outbound message、内部上下文注入、工具上下文和结构化 stream 错误。

## 跨项目依赖顺序

建议按下面顺序推进，避免一次性把产品语义和运行时协议搅在一起。

### Phase 0：开关和边界

只在 `nexus-core` 加配置开关，默认关闭或只对开发环境开启：

- `NEXUS_GOAL_ENABLED`
- `NEXUS_GOAL_AUTO_CONTINUE_ENABLED`
- `NEXUS_GOAL_MAX_CONTINUATIONS_PER_RUN`
- `NEXUS_GOAL_DEFAULT_TOKEN_BUDGET`

本阶段不改变普通对话行为。

### Phase 1：Bridge 通用能力

先发一个小版本，例如 `nexus-agent-sdk-bridge v0.1.1`。

Bridge 改动必须是纯 additive，现有 `Send`、`Receive`、`Interrupt`、`ProcessBackend` 行为不变。

需要补的不是 Goal，而是这些通用能力：

1. typed usage extraction
2. terminal result classification
3. meta/synthetic outbound send options
4. optional next-turn internal context API
5. tool execution context metadata
6. structured stream close error
7. runtime capability detection

如果 Claude Code CLI 当前不支持真正的 hidden context，Bridge 也不要伪装支持。应该返回 capability unsupported，让 `nexus-core` 退化为发送 synthetic/meta user message，并在 Nexus 侧控制该消息不进入可见用户 transcript。

### Phase 2：nexus-core Goal 领域层

在 `nexus-core` 中实现 DB、repository、service、state machine、API 和 websocket events。

此阶段先不自动续跑，只保证 Goal 能创建、展示、更新、暂停、恢复、完成、阻塞、清除，并可持久化。

### Phase 3：运行时注入和模型工具

在 DM 和 Room 的 runtime 启动路径中注入 Goal 上下文和 Goal MCP tools。

模型工具只允许：

- 读取当前 Goal 快照。
- 标记完成。
- 标记阻塞。
- 追加进度 note。

模型不允许：

- 修改 token budget。
- 清除 Goal。
- 替用户创建新 Goal。
- 越权操作其他 session 的 Goal。

### Phase 4：自动续跑和预算

在每轮结束后由 `nexus-core` 判断是否继续。

续跑条件必须全部满足：

- Goal status 为 `active`。
- 当前 session 没有 running round。
- 用户输入队列为空。
- 没有 pending permission。
- 没有未投递完的高优先级系统事件。
- token budget 未耗尽。
- 连续空转次数未超过上限。

### Phase 5：checkpoint、恢复和 heartbeat

补长期任务需要的稳定性：

- 每 N 次 continuation 生成 checkpoint。
- 接近上下文限制前生成 Goal summary。
- 服务重启后从 DB 恢复 active/paused/blocked Goal。
- heartbeat 可以唤醒当前 thread/session，但不能和 scheduled task 混成一个领域模型。

### Phase 6：前端和观测

前端提供 Goal 面板、状态徽标、预算、进度、暂停/恢复/清除操作和 Goal events。

日志、metric、event 必须能回答：

- 当前 Goal 是谁创建的。
- 为什么继续或为什么不继续。
- 哪一轮把 Goal 标记成完成或阻塞。
- token 预算消耗到了哪里。
- 最近一次失败是什么。

## Bridge 需要改哪些

Bridge 是本轮最小但关键的改造点。它不实现 Goal，只补运行时原语。

### 1. `protocol/usage.go`

新增 typed usage 模型，把当前 result message 里的 raw usage 解析成稳定结构。

建议结构：

```go
type TokenUsage struct {
    InputTokens              int64
    OutputTokens             int64
    CacheCreationInputTokens int64
    CacheReadInputTokens     int64
    ReasoningTokens          int64
    TotalTokens              int64
    Raw                      map[string]any
}
```

新增方法：

- `ParseTokenUsage(raw any) (TokenUsage, bool)`
- `TokenUsage.Add(other TokenUsage) TokenUsage`
- `TokenUsage.IsZero() bool`

用途：

- `nexus-core` 可以做 Goal budget，而不是解析 provider-specific raw JSON。
- 后续接不同 runtime 时仍能保留 raw 字段。

兼容性：

- 不移除现有 raw usage 字段。
- 如果解析失败，返回 zero usage + `false`，不能让普通对话失败。

测试：

- Anthropic/Codex 常见 usage JSON。
- 字段缺失。
- 字段类型为 `float64`、`int`、`json.Number`。
- unknown fields 保留在 Raw。

### 2. `protocol/result_status.go`

新增 terminal 分类，把底层 result subtype / terminal reason 归一到产品可用分类。

建议结构：

```go
type TerminalCategory string

const (
    TerminalCategorySuccess     TerminalCategory = "success"
    TerminalCategoryInterrupted TerminalCategory = "interrupted"
    TerminalCategoryLimit       TerminalCategory = "limit"
    TerminalCategoryError       TerminalCategory = "error"
    TerminalCategoryCancelled   TerminalCategory = "cancelled"
    TerminalCategoryUnknown     TerminalCategory = "unknown"
)
```

新增方法：

- `ClassifyTerminal(subtype string, terminalReason string) TerminalCategory`
- `TerminalCategory.IsRetryable() bool`
- `TerminalCategory.IsUserInterrupted() bool`

用途：

- Goal continuation 需要区分正常完成、用户中断、上下文限制、预算限制和 runtime error。
- `nexus-core` 不应该散落一堆字符串判断。

兼容性：

- `ResultMessage.ResultSubtype`、`TerminalReason` 保持原样。
- 分类只是辅助字段或 helper，不改变 stream 协议。

### 3. `protocol/message.go`

新增 outbound message options，支持 meta/synthetic 标记。

建议结构：

```go
type OutboundMessageOptions struct {
    Meta           bool
    Synthetic      bool
    HiddenFromUser bool
    Purpose        string
    Metadata       map[string]string
}
```

需要注意：

- `HiddenFromUser` 不是保证底层模型不可见，它表示这个消息不应该作为真实用户输入展示在 Nexus UI。
- 真正的 model hidden context 要依赖底层 runtime capability。
- 如果底层不支持隐藏上下文，Nexus 可以把 Goal context 作为 synthetic/meta user message 发给模型，但不展示成用户消息。

新增 helper：

- `NewTextMessageWithOptions(text string, options OutboundMessageOptions)`
- `Message.IsSynthetic() bool`
- `Message.IsMeta() bool`

兼容性：

- 现有 `NewTextMessage`、`Send(ctx, prompt)` 行为完全不变。
- 新字段序列化时使用 `omitempty`。

### 4. `client/session.go`

新增发送 API，但保留现有 API。

建议新增：

```go
func (s *Session) SendWithOptions(ctx context.Context, prompt string, options protocol.OutboundMessageOptions) error
func (s *Session) SendMessageWithOptions(ctx context.Context, msg protocol.Message, options protocol.OutboundMessageOptions) error
```

用途：

- `nexus-core` 的 Goal continuation 可以用 synthetic/meta 标记发送下一轮内部输入。
- 普通用户输入仍走现有 `Send`。

兼容性：

- `Send` 内部可以调用 `SendWithOptions(ctx, prompt, protocol.OutboundMessageOptions{})`。
- 不改变 public interface 的已有方法签名。

### 5. `client/control.go` 或 `client/session.go`

新增可选的 next-turn internal context API。

建议接口：

```go
type InternalContextBlock struct {
    Name     string
    Content  string
    Priority int
    Metadata map[string]string
}

func (s *Session) SetNextTurnContext(ctx context.Context, blocks []InternalContextBlock) error
```

能力约束：

- 如果当前 process backend 无法把 context 注入为 hidden/system context，返回 `ErrUnsupportedCapability`。
- 不能偷偷降级成普通 user message，因为调用方需要知道上下文是否真 hidden。
- 降级策略由 `nexus-core` 决定。

用途：

- 完整复现 Codex `/goal` 时，每轮模型都需要看到 Goal 摘要、预算、状态和操作规则。
- 这是最接近 Codex hidden context 的 bridge 原语。

### 6. `client/capability.go`

新增 runtime capability 查询。

建议：

```go
type Capability string

const (
    CapabilitySendOptions        Capability = "send_options"
    CapabilityInternalContext    Capability = "internal_context"
    CapabilityTypedUsage         Capability = "typed_usage"
    CapabilityTerminalCategory   Capability = "terminal_category"
)

func (s *Session) Supports(cap Capability) bool
```

用途：

- `nexus-core` 可以在运行时选择 hidden context 或 synthetic fallback。
- 不需要通过版本号猜能力。

### 7. `tools/tool.go`

扩展 tool context，但保持已有工具签名兼容。

建议给工具执行上下文增加可选字段：

```go
type ExecutionContext struct {
    SessionID string
    RoundID   string
    Source    string
    Metadata  map[string]string
}
```

如果已有 `tools.Context`，就在其上 additive 扩展。

用途：

- Nexus 注入 Goal MCP tool 时，可以可靠知道调用来自哪个 session/round。
- 如果当前 tool server 已经通过闭包绑定 session，则这个字段不是必需，但对日志和审计有价值。

### 8. `client/errors.go`

新增结构化错误，特别是 stream 提前关闭。

建议：

```go
type StreamClosedBeforeTerminalError struct {
    LastMessageID string
    LastMessageType string
    SessionID string
    Cause error
}
```

用途：

- Goal 长程续跑必须知道是可重试 transport 问题，还是模型正常结束。
- 现有外层错误字符串不适合做自动恢复判断。

兼容性：

- 可以用 `errors.As` 识别。
- 原有错误文本可保留。

### 9. README 和 changelog

更新：

- `README.md`
- `README_zh.md`
- `CHANGELOG.md` 或 release notes

重点说明：

- Bridge 不提供 Goal。
- Bridge 提供的是 generic continuation/context/usage primitives。
- `SetNextTurnContext` 可能因为 backend 不支持而返回 capability error。

### Bridge 验收

Bridge 必须满足：

- `go test ./...` 通过。
- 老代码只用 `Send`、`ReceiveMessages`、`Interrupt` 时行为不变。
- 新 API 对 unsupported backend 返回明确错误。
- typed usage 对未知字段容忍。
- terminal category 对未知 subtype 返回 `unknown`，不能 panic。

## nexus-core 需要改哪些

`nexus-core` 是主体工作区。Goal 的状态、策略和 UI 全在这里。

### 1. 协议模型

新增：

- `internal/protocol/model_goal.go`
- `internal/protocol/model_goal_event.go` 或并入 `model_event.go`

建议类型：

```go
type GoalStatus string

const (
    GoalStatusActive   GoalStatus = "active"
    GoalStatusPaused   GoalStatus = "paused"
    GoalStatusComplete GoalStatus = "complete"
    GoalStatusBlocked  GoalStatus = "blocked"
    GoalStatusCleared  GoalStatus = "cleared"
)

type Goal struct {
    ID          string
    SessionKey  string
    Objective   string
    Status      GoalStatus
    TokenBudget *int64
    Usage       GoalUsage
    Version     int64
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CompletedAt *time.Time
    BlockedAt   *time.Time
}
```

配套：

- `CreateGoalRequest`
- `UpdateGoalRequest`
- `PauseGoalRequest`
- `ResumeGoalRequest`
- `ClearGoalRequest`
- `GoalSnapshot`
- `GoalProgressNote`
- `GoalUsage`
- `GoalCheckpoint`

WebSocket event：

- `goal.created`
- `goal.updated`
- `goal.status_changed`
- `goal.progress_added`
- `goal.continuation_scheduled`
- `goal.continuation_skipped`
- `goal.checkpoint_created`
- `goal.cleared`

### 2. 数据库迁移

新增 sqlite/postgres migration。实际编号在实现时以当前 migration head 为准，避免和已有未合入 migration 冲突。

建议表：

#### `session_goals`

字段：

- `id`
- `session_key`
- `objective`
- `status`
- `token_budget`
- `token_used_input`
- `token_used_output`
- `token_used_cache_creation`
- `token_used_cache_read`
- `continuation_count`
- `empty_progress_count`
- `version`
- `created_by`
- `created_at`
- `updated_at`
- `completed_at`
- `blocked_at`
- `cleared_at`
- `last_error`
- `metadata_json`

索引：

- `idx_session_goals_session_key`
- `idx_session_goals_status`
- partial unique index：同一个 `session_key` 只能有一个 `active`/`paused`/`blocked` Goal。

#### `goal_events`

字段：

- `id`
- `goal_id`
- `session_key`
- `event_type`
- `source`
- `round_id`
- `payload_json`
- `created_at`

用途：

- 审计状态变化。
- 调试为什么自动续跑或停止。

#### `goal_checkpoints`

字段：

- `id`
- `goal_id`
- `session_key`
- `summary`
- `continuation_count`
- `usage_json`
- `created_at`

用途：

- 长程任务压缩上下文。
- 服务重启后恢复 Goal 进展。

### 3. Storage repository

新增包：

- `internal/storage/goal/repository.go`
- `internal/storage/goal/model_goal.go`
- `internal/storage/goal/sqlite_repository.go`
- `internal/storage/goal/postgres_repository.go`
- `internal/storage/goal/repository_test.go`

Repository 接口建议：

```go
type Repository interface {
    Create(ctx context.Context, goal *GoalRecord) error
    GetCurrentBySessionKey(ctx context.Context, sessionKey string) (*GoalRecord, error)
    GetByID(ctx context.Context, id string) (*GoalRecord, error)
    UpdateWithVersion(ctx context.Context, goal *GoalRecord, expectedVersion int64) error
    AppendEvent(ctx context.Context, event *GoalEventRecord) error
    ListEvents(ctx context.Context, goalID string, limit int) ([]GoalEventRecord, error)
    CreateCheckpoint(ctx context.Context, checkpoint *GoalCheckpointRecord) error
    GetLatestCheckpoint(ctx context.Context, goalID string) (*GoalCheckpointRecord, error)
}
```

重点：

- 使用 optimistic version 防止模型工具、用户操作、自动续跑同时改状态。
- `GetCurrentBySessionKey` 只返回 `active`/`paused`/`blocked` 的当前 Goal。
- `complete`、`cleared` 作为历史保留，不再参与当前 Goal 唯一约束。

### 4. Goal service

新增包：

- `internal/service/goal/service.go`
- `internal/service/goal/state_machine.go`
- `internal/service/goal/service_usage.go`
- `internal/service/goal/service_checkpoint.go`
- `internal/service/goal/service_continuation.go`
- `internal/service/goal/service_tool.go`
- `internal/service/goal/service_test.go`

核心职责：

- 创建当前 Goal。
- 替换或清除旧 Goal。
- 状态转移校验。
- 用户操作和模型工具操作权限分离。
- 记录 usage。
- 计算是否应该 continuation。
- 生成注入给模型的 Goal context。
- 生成 checkpoint。

状态转移建议：

| From | To | Source | 说明 |
| --- | --- | --- | --- |
| none | active | user/api | 创建 Goal |
| active | active | user/api | 更新 objective/budget |
| active | paused | user/api/system | 暂停 |
| paused | active | user/api | 恢复 |
| blocked | active | user/api | 用户确认后恢复 |
| active | complete | model/tool/user | 完成 |
| active | blocked | model/tool/system | 阻塞 |
| active/paused/blocked | cleared | user/api | 清除当前 Goal |

限制：

- 模型只能从 `active` 标记到 `complete` 或 `blocked`。
- 模型不能从 `blocked` 恢复，也不能清除 Goal。
- 系统只能因为预算、错误退避、上下文限制进入 `paused` 或 `blocked`，不能替用户宣布完成。

### 5. HTTP handlers

新增：

- `internal/handler/goal/handlers.go`
- `internal/handler/goal/routes.go`
- `internal/handler/goal/handlers_test.go`

API 建议：

- `GET /api/goals/current?session_key=...`
- `POST /api/goals`
- `PATCH /api/goals/{goal_id}`
- `POST /api/goals/{goal_id}/pause`
- `POST /api/goals/{goal_id}/resume`
- `POST /api/goals/{goal_id}/clear`
- `GET /api/goals/{goal_id}/events`

路由装配：

- `cmd/nexus-server/app` 或当前统一 routes 装配处。

注意：

- `session_key` 必须走现有 session 权限校验。
- 不允许前端用任意 session_key 操作其他会话。

### 6. DM runtime 集成

需要改：

- `internal/service/dm/service.go`
- `internal/service/dm/service_request.go`
- `internal/service/dm/service_round.go`
- `internal/service/dm/service_input_queue.go`

关键接入点：

1. `HandleChat` 收到用户输入后，读取当前 Goal snapshot。
2. 如果 Goal active，把 Goal context 注入 runtime。
3. 创建 runtime client 时注入 Goal MCP tools。
4. `executeRound` 结束后记录 usage。
5. 让 Goal service 决定是否需要 continuation。
6. 如果用户输入队列非空，先处理用户输入，不续跑 Goal。
7. 如果需要续跑，发送 synthetic/meta continuation input，不展示成普通用户消息。

续跑 prompt 应由 `goal.Service` 生成，例如：

```text
Continue working on the active goal using the latest transcript and checkpoint.
Do not ask the user for input unless blocked.
Call mark_goal_complete when the goal is genuinely done.
Call mark_goal_blocked only when you cannot make meaningful progress without user input or external state.
```

实际文本可以中文化，但语义要稳定，并由后端集中生成，不能散落在 DM/Room 中。

### 7. Room runtime 集成

需要改：

- `internal/service/room/runtime_env.go`
- `internal/service/room/service_realtime.go`
- `internal/service/room/service_runtime.go` 或当前 round 执行文件
- `internal/service/room/input_queue.go`

建议第一版策略：

- Goal 绑定到具体 `session_key` 或具体 room member runtime。
- 不做 room-global shared goal。
- Room 内每个 agent/member 如果有独立 runtime session，就只能看到自己的 Goal。

原因：

- shared goal 会牵涉多 agent 协作、抢占、谁能 complete/block 的问题。
- 第一版先保证单 runtime 行为正确。

### 8. Runtime adapter 集成 Bridge 新 API

需要改：

- `internal/runtime/clientopts/options_agent_client.go`
- `internal/runtime/clientopts/options_claude_command.go`
- `internal/runtime/manager.go`
- `internal/runtime/executor_round.go`
- 可新增 `internal/runtime/goal_context.go`

工作：

- 升级 `go.mod` 中 `github.com/nexus-research-lab/nexus-agent-sdk-bridge` 到包含新 helper 的版本。
- 在 runtime adapter 中暴露 `SendWithOptions` 能力。
- 如果 bridge 支持 `SetNextTurnContext`，优先使用 hidden/internal context。
- 如果不支持，fallback 到 synthetic/meta user message。
- `RoundExecutionResult` 保留 typed usage 和 terminal category。

注意：

- 不能让普通 `Query(ctx, prompt)` 自动带 Goal。
- Goal context 必须由 DM/Room service 显式传入。

### 9. Goal MCP tools

新增：

- `internal/runtime/mcp/goal/server.go`
- `internal/runtime/mcp/goal/tool_get_goal.go`
- `internal/runtime/mcp/goal/tool_mark_complete.go`
- `internal/runtime/mcp/goal/tool_mark_blocked.go`
- `internal/runtime/mcp/goal/tool_add_progress.go`

工具：

#### `get_current_goal`

返回：

- objective
- status
- budget
- usage
- latest checkpoint
- allowed actions

#### `mark_goal_complete`

参数：

- `summary`
- `evidence`

行为：

- 只允许 active goal。
- 写入 goal event。
- websocket 广播状态变化。

#### `mark_goal_blocked`

参数：

- `reason`
- `needed_input`
- `attempts_made`

行为：

- 只允许 active goal。
- 如果 blocked policy 要求连续阻塞多轮，可以先写 progress event，达到阈值后再转 blocked。

#### `add_goal_progress`

参数：

- `note`
- `next_step`

行为：

- 不改变 status。
- 用于 checkpoint 和 UI 进度。

安全：

- 工具 server 由 Nexus 注入，绑定当前 session/goal。
- 模型不能传 goal_id 去操作其他 Goal。
- 所有工具操作都走 `goal.Service` 状态机。

### 10. Continuation coordinator

建议放在：

- `internal/service/goal/service_continuation.go`

由 Goal service 返回一个纯决策对象：

```go
type ContinuationDecision struct {
    ShouldContinue bool
    Reason         string
    Prompt         string
    Delay          time.Duration
    MaxAttempts    int
}
```

DM/Room 负责实际执行：

- 检查是否有 running round。
- 检查 input queue。
- 调用 runtime `SendWithOptions` 或现有 send fallback。
- 记录 round metadata。

不要把 DM/Room 的 runtime mutex 和 queue 逻辑放进 Goal service，否则会产生循环依赖。

### 11. Checkpoint 和上下文压缩

新增逻辑：

- 每 `N` 次 continuation 生成 checkpoint。
- usage 接近预算或上下文限制时生成 checkpoint。
- blocked/paused 前生成 checkpoint。
- resume 时把 latest checkpoint 放入 Goal context。

可以复用现有 summary/memory 思路，但 Goal checkpoint 应该是自己的领域数据，不要塞进普通消息表后再反查。

### 12. Automation / heartbeat 集成

不要把 Goal 做成 scheduled task。

可以共享的是唤醒能力：

- heartbeat 可以提醒当前 thread/session 继续检查 Goal。
- scheduled task 可以创建用户输入或系统输入，但不应该成为 Goal 状态源。

建议后置改：

- `internal/service/automation/service_heartbeat.go`：heartbeat prompt 中可包含 active Goal snapshot。
- `internal/service/automation/service_scheduler.go`：只在明确允许 unattended continuation 时调用 Goal continuation dispatcher。

边界：

- Automation 失败不能把 Goal 直接标成失败。
- Goal continuation 失败要记录 goal event 和 retry state。

### 13. Frontend

需要改：

- `web/src/types/conversation/goal.ts`
- `web/src/types/system/websocket.ts`
- `web/src/lib/api/goal-api.ts`
- `web/src/hooks/agent/use-session-goal.ts`
- `web/src/features/conversation/room/dm/dm-conversation-header.tsx`
- `web/src/features/conversation/room/dm/dm-chat-panel.tsx`
- `web/src/features/conversation/shared/message/item/*`

第一版 UI：

- DM header 显示 Goal badge。
- 点击打开 Goal panel。
- 支持 create/update/pause/resume/clear。
- 展示 status、usage、latest checkpoint、progress events。
- `goal.status_changed` websocket 到达后实时更新。

重要 UX：

- synthetic continuation 不能显示成用户发言。
- Goal event 可以作为轻量 system event 显示，例如“Goal 已继续执行 / 已暂停 / 已完成”。
- 如果模型标记 blocked，需要清楚展示需要用户提供什么。

Room UI 第一版可以只显示当前 selected member/session 的 Goal，避免一开始做多 agent shared goal。

## 非破坏性约束

### Bridge

- 只新增 API，不改已有 API 签名。
- raw message 字段继续保留。
- unsupported capability 用结构化错误返回，不 panic。
- process backend 默认行为不变。

### nexus-core

- 功能开关关闭时，普通 DM、Room、Automation 行为完全不变。
- 没有 active Goal 时，不注入任何 Goal context，不注册 Goal tools。
- 用户输入永远优先于 Goal continuation。
- Goal continuation 不进入普通用户消息 transcript。
- 模型工具只能操作当前 session 的当前 Goal。
- DB 迁移 additive，不删除旧表字段。

## Codex `/goal` 对齐验收矩阵

| Codex 能力 | Nexus 实现点 | 验收 |
| --- | --- | --- |
| 创建当前 goal | Goal API + service + DB | 同 session 只能有一个 current goal |
| 模型看到 goal | runtime context injection | 每轮 prompt/context 中有 goal snapshot |
| 模型标记完成 | Goal MCP tool | 状态变 complete，停止 continuation |
| 模型标记阻塞 | Goal MCP tool | 状态变 blocked，UI 显示 needed input |
| 自动续跑 | continuation coordinator | 无用户输入时继续，有用户输入时停止 |
| token 预算 | bridge typed usage + goal usage | 预算耗尽后暂停或阻塞 |
| interrupt | DM/Room runtime interrupt | interrupt 后不自动继续，除非用户恢复 |
| restart resume | DB + checkpoint | 服务重启后 current goal 可恢复 |
| checkpoint | goal_checkpoints | 长程运行不会只依赖完整 transcript |
| observability | goal_events + websocket | 可追踪每次状态变化和续跑原因 |

## 测试计划

### Bridge

执行：

```bash
go test ./...
```

重点测试：

- `ParseTokenUsage`
- `ClassifyTerminal`
- `SendWithOptions` 不改变 `Send`
- unsupported `SetNextTurnContext`
- `errors.As` 识别 structured stream error

### nexus-core 后端

执行：

```bash
go test ./internal/protocol/...
go test ./internal/storage/goal/...
go test ./internal/service/goal/...
go test ./internal/handler/goal/...
go test ./internal/service/dm/...
go test ./internal/service/room/...
```

最终执行：

```bash
make check-backend
make check
```

重点测试：

- 状态机合法/非法转移。
- optimistic version 并发更新。
- 同 session current goal 唯一约束。
- 用户输入队列优先于 continuation。
- feature flag off 时现有 DM 流程不变。
- 模型 tool 不能越权操作 goal。
- budget 耗尽不会继续空转。
- service restart 后可恢复 latest checkpoint。

### 前端

执行：

```bash
pnpm lint
pnpm typecheck
```

如有本地 e2e：

```bash
pnpm test:e2e
```

重点测试：

- Goal panel create/update/pause/resume/clear。
- websocket event 实时刷新。
- synthetic continuation 不显示成用户消息。
- blocked 状态展示 needed input。

## 推荐落地里程碑

### M1：Bridge v0.1.1

输出：

- typed usage
- terminal category
- send options
- capability detection
- unsupported internal context API
- structured stream close error

验收：

- Bridge `go test ./...` 通过。
- nexus-core 升级依赖后无行为变化。

### M2：nexus-core Goal 基础层

输出：

- protocol
- migrations
- repository
- service state machine
- handlers
- websocket events

验收：

- 可以通过 API 创建/暂停/恢复/清除 Goal。
- 页面刷新后 Goal 仍在。
- 无 runtime 自动续跑。

### M3：模型上下文和工具

输出：

- runtime Goal context injection
- Goal MCP tools
- usage recording

验收：

- 模型能看到 Goal。
- 模型能 mark complete/block。
- 状态变化可广播到前端。

### M4：自动续跑

输出：

- continuation decision
- synthetic/meta runtime input
- user queue priority
- budget stop
- retry/backoff

验收：

- 无用户输入时可持续推进。
- 有用户输入时不抢占。
- interrupt 后不会偷偷续跑。

### M5：checkpoint 和恢复

输出：

- goal checkpoint
- summary context
- restart resume
- heartbeat wake integration

验收：

- 长程任务不依赖完整 transcript。
- 重启后能继续或显示可恢复状态。

### M6：前端完整体验

输出：

- Goal panel
- status badge
- progress timeline
- budget display
- blocked input callout

验收：

- 用户可以完整使用 `/goal` 等价能力。
- 普通对话、Room、Automation 回归正常。

## 最小可交付和完整可交付的区别

最小可交付可以只改 `nexus-core`：

- DB + service + API。
- 每轮 prompt 前插入 synthetic Goal context。
- 用 Nexus 自己的 MCP tool 标记 complete/block。

但完整、健壮、优雅的实现应该改 Bridge：

- 不让 Nexus 解析 bridge raw usage。
- 不让 Nexus 到处判断 terminal string。
- 不让 synthetic/meta message 变成临时约定。
- 不让 hidden context 支持与否靠猜。
- 不让 stream close error 只能读字符串。

因此本计划建议先做 Bridge 的通用能力小版本，再做 Nexus 的 Goal 产品层。这样不会把 Goal 概念塞进 SDK，也能让 Nexus 的长程任务实现更稳。
