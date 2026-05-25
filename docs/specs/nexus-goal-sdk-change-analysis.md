# Nexus Goal 的 SDK 改造分析

本文分析两个本地拉取的 SDK 仓库：

- `/Users/berhand/program/Work/Nexus/sdk-analysis/nexus-agent-sdk-bridge`
- `/Users/berhand/program/Work/Nexus/sdk-analysis/nexus-agent-sdk-go`

目标是回答：如果 Nexus 要实现对标 Codex `/goal` 的长程任务能力，SDK 侧是否必须改、需要改哪些、怎么改。

## 1. 结论

当前 Nexus 编译使用的是：

```text
github.com/nexus-research-lab/nexus-agent-sdk-bridge v0.1.0
```

不是 `github.com/nexus-research-lab/nexus-agent-sdk-go`。

因此实施顺序应该是：

1. **Goal 主体全部在 Nexus 实现。**
2. **MVP 可以不改 SDK。**
3. 如果要把长程任务做得更干净、更可观测，再优先给 `nexus-agent-sdk-bridge` 补通用 runtime 原语。
4. `nexus-agent-sdk-go` 只作为未来切换到原生 Go agent loop 时的参考和后续改造对象。

SDK 里不应该出现 `Goal`、`GoalStatus`、`session_goals`、heartbeat wake、前端 badge 这类业务概念。SDK 应该只补通用能力：

- typed usage。
- terminal reason 分类。
- per-turn internal context 注入。
- meta/synthetic 出站消息构造。
- 更完整的 SDK-hosted MCP tool call context。

## 2. 本地拉取状态

| 仓库 | 本地路径 | 当前 HEAD | 标签 | 测试结果 |
| --- | --- | --- | --- | --- |
| `nexus-agent-sdk-bridge` | `/Users/berhand/program/Work/Nexus/sdk-analysis/nexus-agent-sdk-bridge` | `fae3a4d` | `v0.1.0` | `go test ./...` 通过 |
| `nexus-agent-sdk-go` | `/Users/berhand/program/Work/Nexus/sdk-analysis/nexus-agent-sdk-go` | `0c4bb87` | 无当前标签 | `go test ./...` 失败 |

`nexus-agent-sdk-go` 当前失败点：

- `internal/provider/anthropic` 的 prompt cache scope / beta header 相关测试失败。
- `internal/services/agentsummary` 的 `TestCommandRunnerFromEnvParsesSummary` 超时。

这个结果不影响 Nexus 当前运行，因为 Nexus 现在不依赖 `nexus-agent-sdk-go`。

## 3. 两个 SDK 的定位

### 3.1 nexus-agent-sdk-bridge

`bridge` 是当前 Nexus 使用的 SDK。它的定位是：给 Go 宿主提供稳定 client API，底层启动或连接 Claude Code / bridge runtime。

关键能力已经存在：

- `client.Query` / `client.Prompt`。
- `client.NewSession` / `client.ResumeSession`。
- `Session.Send` / `Session.SendMessage`。
- `Session.Interrupt`。
- `Session.Control().SetModel`。
- `Session.Control().SetPermissionMode`。
- `Session.Control().SetMaxThinkingTokens`。
- `Session.Control().ContextUsage`。
- `Session.MCP().Status`。
- `Session.MCP().SetServers`。
- `Options.WithSDKMCPServer`。
- `Options.WithCustomTools`。
- hooks，包括 `UserPromptSubmit`、`PreToolUse`、`PostToolUse`、`Stop`、`PreCompact`、`PostCompact`。
- result 消息里已经有 `usage`、`model_usage`、`terminal_reason`、`duration_ms`、`total_cost_usd` 等原始字段。

当前不足：

- usage 只是 `map[string]any`，没有 typed helper。
- terminal reason 是字符串，没有统一枚举和分类 helper。
- 出站消息只有普通 user helper；meta/synthetic 消息要用 raw map。
- system prompt / append system prompt 主要在 session 初始化时设置，没有干净的 per-turn internal context API。
- SDK-hosted custom tool 的 `tools.Context` 目前只有 `ToolUseID`，没有 session/model/round 这类调用上下文。

### 3.2 nexus-agent-sdk-go

`sdk-go` 是更完整的原生 Go agent loop。它不只是 bridge facade，而是包含 provider、query loop、tool execution、compact、session transcript、memory、built-in tools 等运行链路。

它已经有一些对 Goal 有参考价值的能力：

- provider 层 usage 聚合。
- query loop 内部 token budget continuation。
- stop hook continuation。
- auto compact / reactive compact。
- session memory compaction。
- provider API error category，例如 rate limit、max output tokens、billing、invalid request。
- provider request pipeline。

但它不是当前 Nexus 的实际依赖，并且当前 HEAD 全量测试不绿。短期不应该把 Nexus Goal 绑定到它。

## 4. Nexus MVP 是否需要改 SDK

不需要。

Nexus 现有能力已经足够做 Goal MVP：

- Goal 状态机、DB、API、CLI、WebSocket 全在 Nexus。
- Goal tools 可以通过 `WithSDKMCPServer` / `WithCustomTools` 挂到当前 runtime。
- 当前 goal snapshot 可以由 Nexus 在每轮请求前组织成内部上下文。
- round 结束后的自动 continuation 由 Nexus 的 `roundRunner` / runtime manager / input queue 判断。
- budget 和 usage 可以先从 `ResultMessage.Usage`、`ResultMessage.ModelUsage`、`DurationMS`、`TerminalReason` 原始字段归集。
- interrupt、resume、context usage 都已经有 SDK API。

MVP 阶段建议只改 Nexus：

1. `internal/protocol/model_goal.go`
2. `internal/storage/goal`
3. `internal/service/goal`
4. DM/Room round lifecycle hook
5. runtime MCP goal tools
6. frontend goal panel / event

SDK 改动等到 MVP 行为跑通后再做。

## 5. bridge 侧建议改造

以下改造都应保持通用，不引入 Goal 业务语义。

### 5.1 P1：typed usage helper

问题：

`protocol.ResultMessage` 目前有：

```go
Usage      map[string]any
ModelUsage map[string]any
```

Nexus 做 budget 时需要每次自己解析：

- `input_tokens`
- `output_tokens`
- `cache_creation_input_tokens`
- `cache_read_input_tokens`
- `cache_deleted_input_tokens`
- `server_tool_use.web_search_requests`
- `server_tool_use.web_fetch_requests`

建议新增：

```go
type TokenUsage struct {
    InputTokens              int64
    OutputTokens             int64
    CacheCreationInputTokens int64
    CacheReadInputTokens     int64
    CacheDeletedInputTokens  int64
    WebSearchRequests        int64
    WebFetchRequests         int64
    Raw                      map[string]any
}

func DecodeTokenUsage(raw map[string]any) TokenUsage
func (u TokenUsage) TotalTokens() int64
func (u TokenUsage) BillableInputTokens() int64
func (u TokenUsage) Add(other TokenUsage) TokenUsage
func (m ResultMessage) TokenUsage() TokenUsage
func (m ResultMessage) PerModelTokenUsage() map[string]TokenUsage
```

落点：

- `protocol/message.go` 或新增 `protocol/usage.go`。
- `protocol/message_test.go` 增加 decode / total / per-model 测试。

对 Nexus 的收益：

- Goal budget 逻辑不用依赖散落的 `map[string]any` 解析。
- 不同 provider 的字段差异可以在 SDK helper 里归一。
- 仍保留原始 map，兼容已有代码。

### 5.2 P1：terminal reason 分类

问题：

`ResultMessage.TerminalReason` 当前是字符串，`Subtype` 和 `IsError` 也只是原始字段。Goal 需要区分：

- 正常完成。
- 用户中断。
- provider quota / rate limit。
- context too long。
- max output tokens。
- runtime stream abnormal close。
- permission/user input wait。

建议新增：

```go
type TerminalCategory string

const (
    TerminalCompleted       TerminalCategory = "completed"
    TerminalInterrupted     TerminalCategory = "interrupted"
    TerminalUsageLimited    TerminalCategory = "usage_limited"
    TerminalContextTooLong  TerminalCategory = "context_too_long"
    TerminalMaxOutputTokens TerminalCategory = "max_output_tokens"
    TerminalPermissionWait  TerminalCategory = "permission_wait"
    TerminalRuntimeError    TerminalCategory = "runtime_error"
    TerminalUnknown         TerminalCategory = "unknown"
)

func (m ResultMessage) TerminalCategory() TerminalCategory
func (m ResultMessage) IsUsageLimited() bool
func (m ResultMessage) IsInterrupted() bool
func (m ResultMessage) IsSuccessfulTerminal() bool
```

落点：

- `protocol/result_status.go`。
- `protocol/message_test.go` 或 `protocol/result_status_test.go`。

对 Nexus 的收益：

- Goal service 可以直接把 SDK terminal category 映射到 `goal.status`。
- `usage_limited`、`budget_limited`、`blocked` 的判断更稳定。
- 减少 Nexus 自己猜错误字符串。

### 5.3 P1：meta/synthetic 出站消息 helper

问题：

`UserMessage` decode 已经支持：

```go
IsMeta
IsSynthetic
Priority
```

但出站强类型 helper 只有普通 user message：

```go
NewUserTextMessage(...)
NewUserBlocksMessage(...)
```

如果 Nexus 要发 Goal continuation，不应该把内部 prompt 当普通用户消息展示。现在可以用 `NewRawMessage` 兜底，但 raw map 容易写错。

建议扩展：

```go
type UserMessageOptions struct {
    IsMeta      bool
    IsSynthetic bool
    Priority    string
}

func NewMetaTextMessage(text string) UserTextMessage
func NewSyntheticTextMessage(text string) UserTextMessage
func (m UserTextMessage) WithMeta(bool) UserTextMessage
func (m UserTextMessage) WithSynthetic(bool) UserTextMessage
func (m UserTextMessage) WithPriority(priority string) UserTextMessage
func (m UserBlocksMessage) WithMeta(bool) UserBlocksMessage
func (m UserBlocksMessage) WithSynthetic(bool) UserBlocksMessage
func (m UserBlocksMessage) WithPriority(priority string) UserBlocksMessage
```

编码时写入：

```json
{
  "type": "user",
  "is_meta": true,
  "is_synthetic": true,
  "priority": "goal_continuation"
}
```

落点：

- `protocol/message.go`。
- `protocol/message_test.go`。

对 Nexus 的收益：

- Goal continuation 可以通过强类型 SDK 消息发送。
- 前端和历史层更容易区分真实用户消息与内部续跑消息。
- 后续 heartbeat、scheduled task、runtime guidance 也能复用。

### 5.4 P1：per-turn internal context 注入

问题：

Goal 每一轮都需要注入当前状态：

```xml
<nexus_goal_context>...</nexus_goal_context>
```

但这个上下文不应该永久写进 system prompt，也不应该显示成用户消息。当前 bridge 有：

- 初始化期 `SystemPrompt` / `AppendSystemPrompt`。
- hook 的 `AdditionalContext`，主要适合工具调用后追加上下文。
- raw synthetic/meta user message 兜底。

建议新增一个通用 per-turn context API，避免 Nexus 依赖 raw message。

可选设计 A：发送时携带 internal context

```go
type InternalContextBlock struct {
    Name      string
    Content   string
    Transient bool
}

type SendOptions struct {
    InternalContext []InternalContextBlock
}

func (s *Session) SendWithOptions(ctx context.Context, prompt string, options SendOptions) (*Stream, error)
func (s *Session) SendMessageWithOptions(ctx context.Context, message protocol.OutboundMessage, options SendOptions) (*Stream, error)
```

可选设计 B：控制面设置下一轮 transient context

```go
func (c *SessionControl) SetNextTurnContext(ctx context.Context, blocks []protocol.InternalContextBlock) error
```

wire subtype：

```json
{
  "subtype": "set_next_turn_context",
  "payload": {
    "blocks": [
      {"name":"nexus_goal_context","content":"...","transient":true}
    ]
  }
}
```

推荐设计：

- bridge 先做 B，保持兼容当前 runtime/control 形状。
- Nexus 在每轮发送前调用 `SetNextTurnContext`。
- 如果底层 Claude Code bridge 不支持该 control subtype，SDK 返回明确 unsupported，Nexus 可回退到 synthetic/meta user message。

落点：

- `protocol/control.go` 增加通用 payload 类型或强类型 request builder。
- `client/session.go` 增加 control method。
- bridge runtime 侧如果不支持，需要在 bridge 子进程协议里同步处理。

对 Nexus 的收益：

- Goal hidden context 不污染用户消息。
- 目标更新、checkpoint、budget warning、resume context 都可以复用。
- 后续其他产品态也可以注入 transient runtime context。

### 5.5 P2：SDK-hosted MCP tool call context

问题：

`tools.Context` 当前只有：

```go
type Context struct {
    ToolUseID string
}
```

Goal tools 可以通过闭包绑定 `session_key`，所以 MVP 不受阻。但如果同一个 SDK MCP server 被多个 session 复用，tool handler 无法从 SDK context 里知道当前 session。

建议扩展：

```go
type Context struct {
    ToolUseID string
    SessionID string
    ServerName string
    ToolName string
    Raw map[string]any
}
```

前提：

- MCP JSON-RPC `tools/call` 或 bridge control payload 里需要携带这些 metadata。
- 如果底层没有 metadata，则字段为空，保持兼容。

落点：

- `tools/tool.go`
- `internal/mcpserver/server.go`
- 相关 MCP call decode 逻辑。

对 Nexus 的收益：

- Goal tools 可以更安全地验证调用上下文。
- 多 session 共享 MCP server 时不靠闭包猜上下文。

### 5.6 P2：stream abnormal close 的结构化错误

问题：

当前 `Stream.Result` 在 EOF 前没收到 result 时返回 `ErrNoResult`。Nexus 自己在 runtime 层已经包装了 “stream closed before terminal” 的信息，但 SDK 层可以给更稳定的结构。

建议新增：

```go
type StreamClosedBeforeResultError struct {
    LastMessageType string
    LastSessionID   string
    LastMessageID   string
    WaitError       error
}
```

或者至少：

```go
func IsStreamClosedBeforeResult(error) bool
func StreamCloseDiagnostics(error) StreamCloseInfo
```

落点：

- `client/errors.go`
- `client/query.go`
- `client/session.go` 消息追踪。

对 Nexus 的收益：

- Goal 可以把异常终止映射为 `usage_limited`、`runtime_error` 或 `interrupted`，而不是只看到一段字符串。

## 6. sdk-go 侧建议改造

如果 Nexus 未来切到 `nexus-agent-sdk-go`，需要改的是原生 loop 的 public/API 边界。短期不建议作为 Goal MVP 前置条件。

### 6.1 typed usage 与 terminal category 也要补

`sdk-go` 的 `protocol.ResultMessage` 也保留了：

```go
Usage      map[string]any
ModelUsage map[string]any
TerminalReason string
```

应与 bridge 保持同一套 public helper。否则 Nexus 从 bridge 切到 sdk-go 时，Goal budget 和 terminal 映射还要重写。

建议：

- 两个 SDK 仓库同步增加 `protocol.TokenUsage`。
- 两个 SDK 仓库同步增加 `protocol.TerminalCategory`。
- 如果长期会同时维护两个 SDK，最好把测试样例保持一致。

### 6.2 原生 loop result 需要补 terminal_reason

`sdk-go` 的 `client.emitQuerySuccessResult` 当前 result payload 有 `stop_reason`，但没有稳定的 `terminal_reason`。Goal 需要一个统一终态字段。

建议：

- 正常 assistant end turn：`terminal_reason=completed`。
- max output tokens：`terminal_reason=max_output_tokens`。
- context blocking limit：`terminal_reason=context_too_long`。
- provider rate limit / overloaded：`terminal_reason=rate_limit` 或 `usage_limited`。
- context canceled / interrupt：`terminal_reason=interrupted`。
- provider invalid request：`terminal_reason=provider_error` 或更细分。

落点：

- `client/agent_loop.go`
- `internal/query/run.go`
- `internal/provider/provider.go`
- `protocol/message_result.go`

### 6.3 public per-turn context provider

`sdk-go` 内部已经有 `SystemPromptBlocks`，但这是 internal runtime/query 边界，不是 public API。

建议新增 public option：

```go
type RuntimeContextProvider func(context.Context, RuntimeContextRequest) ([]protocol.InternalContextBlock, error)

type RuntimeContextRequest struct {
    SessionID string
    TurnID    string
    Model     string
}

func (o Options) WithRuntimeContextProvider(provider RuntimeContextProvider) Options
```

query loop 每次构建 provider request 前调用，把结果追加为 transient system block，不进入持久 transcript。

落点：

- `client/options.go`
- `client/options_builder.go`
- `client/runtime_config.go`
- `internal/runtime/query.go`
- `internal/query/run.go`

对 Nexus 的收益：

- Goal context、checkpoint、预算提示可以作为真正的 transient system context。
- 不需要伪装成 user/meta message。

### 6.4 不复用 sdk-go 的 token budget continuation 作为 Goal

`sdk-go/internal/query/token_budget.go` 已经有 “未达到输出 token 目标就继续” 的 continuation 逻辑。这个逻辑可以参考，但不能直接当 Nexus Goal。

原因：

- 它按 output token budget 推进，不理解 Nexus goal objective。
- 它没有 Nexus `session_key`、DB 状态机、用户暂停/恢复、blocked/complete 工具语义。
- 它属于 query loop 内部策略，不适合作为产品级 Goal 状态源。

如果未来复用，应抽象为更通用的 continuation hook：

```go
type ContinuationDecision struct {
    Continue bool
    Message  protocol.OutboundMessage
    Reason   string
}

type ContinuationDecider func(context.Context, ContinuationRequest) (ContinuationDecision, error)
```

但 Goal 的最终决策仍应在 Nexus。

### 6.5 当前 sdk-go 测试不绿，不能作为切换前提

在本地执行 `go test ./...` 时失败。切换 Nexus 到 sdk-go 前，至少要先修：

- Anthropic prompt cache scope / beta header 测试。
- agentsummary command timeout 测试。
- 再补一组 bridge compatibility tests，保证当前 Nexus 使用的 session、MCP、hooks、permission、context usage 行为一致。

## 7. 推荐路线

### 7.1 第一阶段：Nexus-only MVP

不改 SDK。

Nexus 侧做：

- Goal DB / service / API。
- Goal MCP tools。
- round lifecycle reconciliation。
- continuation coordinator。
- 通过当前 SDK raw usage 归集预算。
- 通过 raw synthetic/meta message 或 Nexus 内部消息层注入 goal context。

验收：

- active goal 能自动续跑。
- 模型能通过 tool 标记 complete / blocked。
- 用户暂停、恢复、清除生效。
- token budget 能停止续跑。

### 7.2 第二阶段：bridge 小补丁

只改通用 SDK 原语：

1. typed usage。
2. terminal category helper。
3. meta/synthetic outbound helper。
4. per-turn internal context control。

Nexus 侧替换掉 raw map 和字符串判断。

验收：

- Nexus Goal 不再直接解析 `map[string]any` usage。
- continuation prompt 不再显示为普通用户消息。
- SDK 不含任何 Goal 业务类型。

### 7.3 第三阶段：sdk-go 迁移评估

只有当 Nexus 准备从 bridge 切到原生 agent loop 时才做：

- 修绿 sdk-go 全量测试。
- 补 terminal_reason。
- 补 public runtime context provider。
- 对齐 bridge 的 typed usage / terminal category public API。
- 用 Nexus Goal e2e 验证 bridge 和 sdk-go 行为一致。

## 8. 文件级修改清单

### nexus-agent-sdk-bridge

| 优先级 | 文件 | 修改 |
| --- | --- | --- |
| P1 | `protocol/usage.go` | 新增 `TokenUsage`、decode、total、add helper |
| P1 | `protocol/result_status.go` | 新增 `TerminalCategory` 和 result 分类 helper |
| P1 | `protocol/message.go` | 给 outbound user message 增加 meta/synthetic/priority helper |
| P1 | `protocol/message_test.go` | 增加 usage、terminal、synthetic outbound 测试 |
| P1 | `protocol/control.go` | 增加 next-turn internal context control payload |
| P1 | `client/session.go` | 增加 `SessionControl.SetNextTurnContext` 或 `SendWithOptions` |
| P2 | `tools/tool.go` | 扩展 `tools.Context` |
| P2 | `internal/mcpserver/server.go` | 透传 MCP call metadata 到 tool context |
| P2 | `client/errors.go` | 增加 stream closed before result 结构化错误 |
| P2 | `README.md` / `README_zh.md` | 文档同步 public API |

### nexus-agent-sdk-go

| 优先级 | 文件 | 修改 |
| --- | --- | --- |
| P1 | `protocol/usage.go` | 与 bridge 同步 typed usage |
| P1 | `protocol/result_status.go` | 与 bridge 同步 terminal category |
| P1 | `client/agent_loop.go` | emit result 时写入 `terminal_reason` |
| P1 | `internal/query/run.go` | 把 query stop/error/continuation 原因映射成 terminal reason |
| P1 | `internal/provider/provider.go` | provider error category 映射到 terminal category |
| P2 | `client/options.go` | 增加 runtime context provider public 类型 |
| P2 | `client/options_builder.go` | 增加 builder |
| P2 | `client/runtime_config.go` | 把 provider 传入 internal runtime |
| P2 | `internal/runtime/query.go` | 每轮构造 system blocks 时追加 transient context |
| P2 | `internal/query/run.go` | 保证 context 不写入 transcript、只进 provider request |
| P2 | `docs/sdk/hooks-and-runtime.md` | 文档同步 |

## 9. 不建议做的事

- 不在 SDK 里定义 `Goal`。
- 不让 SDK 直接读写 Nexus DB。
- 不让 SDK 决定 session goal 是否 active。
- 不让 SDK 接管 heartbeat / scheduled task。
- 不把 sdk-go 的 token budget continuation 直接当 Nexus Goal。
- 不在当前 Nexus Goal MVP 里切换到 sdk-go。

## 10. 最小可落地判断

如果目标是尽快做出 Nexus Goal：

```text
先只改 Nexus。
```

如果目标是把长期维护成本降下来：

```text
Nexus Goal MVP 跑通后，给 nexus-agent-sdk-bridge 补 4 个通用 helper：
typed usage、terminal category、meta/synthetic outbound、next-turn internal context。
```

如果目标是以后做自研原生 agent loop：

```text
先把 nexus-agent-sdk-go 修到全量测试通过，再把 bridge 的 public helper 同步过去，最后再评估 Nexus 迁移。
```
