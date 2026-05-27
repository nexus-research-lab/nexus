# Nexus Goal Runtime Parity Audit

## Scope

本文记录 `codex/goal-runtime-parity` 当前 Goal 功能与 OpenAI Codex 开源实现的对齐状态。

对照基线：

- Codex upstream：`openai/codex` `origin/main`
- upstream commit：`1de8c43467258a395b43f4f973b198cc6c1db256`
- 重点源码：
  - `codex-rs/ext/goal/src/spec.rs`
  - `codex-rs/ext/goal/src/tool.rs`
  - `codex-rs/state/src/runtime/goals.rs`
  - `codex-rs/app-server-protocol/src/protocol/v2/thread.rs`
  - `codex-rs/app-server/src/request_processors/thread_goal_processor.rs`
  - `codex-rs/core/templates/goals/*.md`

本审计只评价 Goal 语义 parity。Nexus 的 DM、Room、Web 面板和 HTTP native API 属于产品承载差异，不要求逐行复刻 Codex TUI。

## Current Assessment

当前实现已经覆盖 Codex Goal 的核心语义，距离 upstream 主要剩下少量宿主系统差异：

- 模型可见工具：已对齐。
- app-server goal 协议：已对齐核心 wire shape。
- 预算、usage、耗时、budget_limited、usage_limited：已对齐核心语义。
- hidden continuation / steering prompt：模板与 upstream 当前文件一致。
- checkpoint：当前 Codex Goal 没有独立 `GoalCheckpoint` 协议或工具，本分支已移除 Nexus 早期私有 checkpoint 分支。
- 文件结构：已拆分过大的 Goal 测试和服务主文件，当前没有必须立即拆分的超大 Go 文件。

结论：核心 runtime parity 已接近完成。剩余差异主要是 Nexus session/room 宿主模型与 Codex thread/rollout 宿主模型不同。

## Parity Matrix

| Area | Codex behavior | Nexus current state | Status |
| --- | --- | --- | --- |
| Model tools | 仅暴露 `get_goal`、`create_goal`、`update_goal` | `internal/runtime/mcp/goal/tool` 只注册这三件套 | Aligned |
| `create_goal` | 只在显式要求时创建；已有 goal 时失败；可选正整数 `token_budget` | 工具描述、schema、冲突错误和预算校验已对齐 | Aligned |
| `update_goal` | 只能设置 `complete` 或 `blocked`；不能 pause/resume/budget-limit | schema enum、错误文案和模型可见描述已对齐 upstream | Aligned |
| Tool result | camelCase：`goal`、`remainingTokens`、`completionBudgetReport` | 结构化结果和文本 JSON payload 均保留这些字段，无值为 `null` | Aligned |
| Goal projection | `threadId/objective/status/tokenBudget/tokensUsed/timeUsedSeconds/createdAt/updatedAt` | `ThreadGoal` app-server 投影一致，`tokenBudget` 显式 nullable | Aligned |
| Notifications | `thread/goal/updated` 包含 nullable `turnId` | `ThreadGoalUpdatedNotification.turnId` 显式 nullable | Aligned |
| App-server methods | `thread/goal/set|get|clear` | WebSocket JSON-RPC 已支持；HTTP 兼容入口也存在 | Aligned plus Nexus extension |
| Set ordering | set response 后发 updated，再触发 active goal continuation | WebSocket 入口先 response/notification，再 dispatch continuation | Aligned |
| Budget accounting | 非缓存 input + output 计入 token budget | `GoalUsage.BudgetTokens()` 使用同一口径 | Aligned |
| Budget stop | active goal 计量达到预算后转 `budget_limited` | `RecordUsage*` 和 app-server budget handling 已覆盖 | Aligned |
| Usage limit | runtime hard usage limit 转 `usage_limited` | DM/Room runtime 都会标记 usage_limited | Aligned |
| Plan mode | Plan 模式不注入 Goal context、不记账、不续跑 | `ShouldIgnoreRuntimeForPermissionMode` 覆盖 DM/Room | Aligned |
| Active turn context | 普通 active turn 只记账，不额外注入 goal hidden context | `RuntimeContext` 对 active turn 返回 usage target，context 为空 | Aligned |
| Hidden continuation | 自动续跑时注入 `<goal_context>` 和完整 completion audit | 三个模板与 upstream 文件 diff 为空 | Aligned |
| Checkpoint | Codex Goal 不提供独立 checkpoint tool/schema/table | Nexus 私有 checkpoint 代码、事件、前端类型和迁移表已移除 | Aligned |
| File size | upstream Goal 拆成 spec/tool/runtime/accounting/steering | Nexus service/test 已拆分为 lifecycle/appserver/continuation/runtime/usage 等文件 | Aligned enough |

## Remaining Differences

### 1. Session identity and materialization

Codex Goal 绑定 `ThreadId`，app-server 会定位 materialized rollout；找不到 thread 或 ephemeral thread 时拒绝 goal 操作。

Nexus Goal 绑定结构化 `session_key`。当前服务层校验 key 结构，但不会在 Goal service 内统一查询 Session service 来证明该 session 已 materialized。原因是 Nexus 同时支持：

- DM agent session
- Room member session
- Room shared session
- Web/HTTP native Goal 操作

这属于宿主模型差异，不影响 Goal runtime 的核心语义。若要继续逼近 Codex app-server，可在外层 app-server handler 增加可选的 session materialization guard，但要避免破坏 Room shared Goal。

### 2. Persistence shape

Codex 使用单表 `thread_goals`，一个 thread 一行；Nexus 使用 `session_goals` 加 `goal_events`。

字段语义已对齐，但 Nexus 保留事件表用于 Web 面板、审计和内部状态广播。这是可接受的实现差异，不需要为了 parity 删除事件表。

### 3. Product surface

Codex TUI 的 `/goal` 菜单和状态栏是 Rust TUI；Nexus 是 Web 面板、斜杠命令提示、HTTP native API 和 Room runtime。

当前操作语义已收口：

- 用户可 create/edit/pause/resume/clear。
- 用户侧不再直接 complete/block。
- 模型通过 `update_goal(status=complete|blocked)` 完成或阻塞。

因此 UI 不需要逐像素复刻 Codex TUI。

## File Size Review

当前 Goal 相关文件规模可维护：

- `internal/service/goal/service.go`：约 230 行，只保留服务结构和公开生命周期入口。
- `internal/service/goal/service_transition.go`：状态迁移和持久化。
- `internal/service/goal/service_helpers.go`：校验、预算、ID 和通用 helper。
- `internal/service/goal/service_continuation_test.go`：约 560 行，是当前最大 Goal 测试文件，仍按 continuation 场景聚合，暂不需要继续拆。
- `internal/runtime/mcp/goal/tool/registry_test.go`：约 380 行，覆盖三件模型工具的注册和行为，规模可接受。

后续拆分阈值建议：

- 单个业务 Go 文件超过 450 行且出现两个以上职责时拆。
- 单个测试文件超过 650 行或 setup/helper 开始主导阅读时拆。
- 不为追求小文件而拆散同一状态机场景。

## Next Actions

1. 保持 upstream Codex 参照点刷新：如果 `spec.rs`、`tool.rs`、`thread_goal_processor.rs` 或模板变化，优先同步模型可见契约。
2. 若要继续靠近 app-server，优先评估 session materialization guard，而不是改 runtime 计费。
3. 保持旧规划文档中的 checkpoint 内容只作为历史方案，不再按独立 Goal checkpoint 方向实现，除非 upstream Codex 重新引入对应能力。
4. 每次改动后至少运行：
   - `go test ./internal/protocol ./internal/runtime/mcp/goal/tool ./internal/service/goal ./internal/storage/goal ./internal/handler/websocket`
   - `pnpm run typecheck`
   - `pnpm run lint`
   - `make check-backend`
