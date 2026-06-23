# Ponytail 结构与边界审计 — 2026-06-22

## 审计口径

本轮用 ponytail-audit 方法对整个 nexus-core 仓库做一次**结构与职责**层面的二次评审，关注四件事：

1. **职责边界**是否清楚——每个文件/包是否只承担一个明确关注点。
2. **冗余/重复**——平行实现、复制粘贴、散落的小工具函数。
3. **结构是否合理 / 是否需要拆分**——god-file、god-package、放错位置的代码。
4. **屎山（过度工程）**——单实现接口、投机泛化、为配置而配置、纯转发的包装层。

ponytail 标签：`delete`（死代码/投机功能）/ `stdlib`（手写标准库已有能力）/ `native`（依赖做了平台已做的事）/ `yagni`（只有一个实现的抽象/没人设的配置/只有一个调用方的层）/ `shrink`（同样逻辑更少行）/ `dup`（应共用一份的复制逻辑）。

> 边界：本轮只看结构与复杂度，不看正确性、安全、性能缺陷——那些走正常 review。

### 与既有审计的关系（重要）

仓库已有两份审计，本轮是**在它们之上的第二次意见**，不重复其工作：

- `engineering-structure-audit-2026-06-16.md`：第一轮全仓结构审计，已落地大量**同包文件拆分**（room/runtime/provider/workspace/automation/IM 适配器等几十处），并记录了若干**明确决定**。
- `ponytail-overengineering-audit-2026-06-17.md`：56 条单点 `delete` 包装器清单（单用 wrapper），属于微观层面，已基本处理。

本轮的新增价值在三处：

- **新证据挑战既有决定**：storage 的双 dialect、room 的 god-package，06-16 都给出了“保持现状”的判断；本轮给出新证据（provider/ 已经用 `bind()` 统一了 dialect，证明合并可行；room 同包拆到 57 个文件后仍难导航）。
- **06-16 没覆盖的盲区**：`runtime/mcp/` 子树（占 runtime 108 个文件里的大头）、`protocol/` 里 appserver JSON-RPC 传输模型的越界、`conversation/` 的整包死代码、前端 `use-agent-conversation.ts` 仍剩 39 个 useCallback。
- **跨包重复的系统性收敛**：`firstNonEmpty`/`normalizeString`/`projectRoot` 等小工具散在 6+ 个包。

下方凡与 06-16 决定冲突处，均标注 **【与 06-16 决定的张力】** 并给出两面理由，交由维护者裁决，不直接下令。

### 置信度标记

- ✅ = 本人直接验证（读了文件 / 跑了 diff 或 `go mod why`）。
- 🔍 = 子审计 agent 报告并附 `path:line`，本人采样核验过。
- ⚠️ = 单一来源，未二次核验，标注“待核验”。

---

## 结论摘要

主干分层（cmd / handler / service / storage / protocol / runtime）**健康**，handler 层薄、chat↔service 边界干净、死代码标记极少（TODO 3 / FIXME 0 / HACK 0 / nolint 0）。06-16 之后又清理了一批大文件（room repo 751→705、use-agent-conversation 1243→1131、i18n 拆成 zh/en）。

**但**仍有三类结构性问题值得动：

| # | 问题 | 类型 | 估算可减 | 置信度 |
|---|------|------|---------|--------|
| S1 | storage 三套 dialect 方案并存，postgres/sqlite 双实现重复 ~1320 行 | dup | ~-1300 | ✅ |
| S2 | `runtime/` 是 5 个子系统挤在一个 package 树（mcp/ 占 6078 LOC） | 结构 | LOC 中性，边界收益大 | 🔍 |
| S3 | `room/` 同包拆到 57 文件后仍是 god-package | 结构 | LOC 中性 | 🔍 |
| S4 | `conversation/` dispatcher/coordinator/model 三文件**整包零调用方**死代码 | delete | ~-165 | 🔍 |
| S5 | `protocol/` 混入 appserver JSON-RPC 传输模型（自定边界违规） | 边界 | 迁移 ~240 LOC | 🔍 |
| S6 | 散落小工具重复 + 双 websocket 库 | dup | ~-100 + 双 WS（中期） | ✅ |

全仓粗估**可减 ~2500–2800 行**（其中 S1 storage 合并占 ~1300，且最有争议），外加若干 LOC 中性但边界收益很大的包级重组。

---

## 删除/合并清单（ponytail 一行格式，按收益排序）

```
dup      postgres/+sqlite/ 五对 repository 文件，仅占位符 $N↔? 与 now()↔CURRENT_TIMESTAMP 等差异，业务逻辑逐字相同。统一到 provider/repository_dialect.go 的 bind() 模式。 [internal/storage/{postgres,sqlite}/repository_*.go]  ✅
delete   conversation/{dispatcher,coordinator,model_conversation}.go 整套 Dispatcher/Coordinator/UnifiedRequest 零外部调用方。 [internal/service/conversation/]  🔍
delete   connectors/catalog.go 中 ~270 行 "coming_soon" 条目 + 95 行中文 feature 文案，非真实集成。 保留已上线条目，文案挪到 docs 侧栏。 [internal/service/connectors/catalog.go]  🔍
dup      gorilla/websocket(channels) 与 coder/websocket(handler) 两套 WS 库。 选一套统一（coder/websocket 更新更勤）。 [internal/service/channels/transport/client.go, internal/handler/websocket/handler.go]  ✅
dup      firstNonEmpty 定义 4 份(skills/provider/agent/automation)、normalizeString/cloneMap 两份(message/permission)、projectRoot() 两份(workspace/skills)。 抽 internal/infra/strx。 🔍
dup      imagegenDefaultResolver 接口 + runtimeImagegenDefaultEnabled 方法在 automation/dm/room 三包逐字重复。 提到 provider 注入。 [automation/service.go, dm/service_runtime_client.go, room/execution_runtime_options.go]  🔍
dup      goal/service_progress.go 三个 reset*ForTransition 函数体完全相同。 合成一个。 [internal/service/goal/service_progress.go:334-353]  🔍
dup      channel adapter 6 个适配器各自重复 ownerUserID/mu/ingress + WithOwner/BaseURL/SetIngress/currentIngress ~15-20 行。 抽 adapters.Base 嵌入结构。 [internal/service/channels/adapters/*.go]  🔍
dup      SendDeliveryMessage 骨架在 5 个适配器重复 ~20 行；ingress 报错分支在 3 处逐字重复。 抽 transport.ChunkedDelivery + adapters.reportIngressError。 [channels/adapters/*_delivery.go]  🔍
dup      goal 错误过滤惯用句 !Is(ErrGoalDisabled)&&!Is(ErrGoalNotFound)&&... 在 goal_runtime*/goal_continuation 出现 6+ 次。 抽 isExpectedGoalError(err)。 [internal/service/room/goal_runtime*.go]  🔍
shrink   runtime/manager_goal_accounting.go Flush/Clear/Activate 三段 Register+Run 同构。 一个 map[kind]callback + 一对 Register/Run。 [internal/runtime/manager_goal_accounting.go]  🔍
shrink   message/processor_system.go 8 个 nil-guard wrapper(firstTaskProgressTaskID...) 每个 if x==nil return ""。 泛型 accessor 或内联。 [internal/message/processor_system.go:139-214]  🔍
shrink   protocol/model_session_key.go NormalizeSessionKeyChannelSegment 与 NormalizeStoredChannelType 同一张 9-case 表写两遍。 一张表 + 方向 flag。 [internal/protocol/model_session_key.go:309-355]  🔍
yagni    runtime Factory 接口只有一个实现 defaultFactory，NewManagerWithFactory 只被 NewManager 自己调用。 直接传 defaultFactory{}。 [internal/runtime/manager_client.go:27-29]  🔍
```

---

## 全局/系统性发现

### S1. storage 的 dialect 问题：三套方案并存（storage 最大的债）

**现状（✅ 本人验证）**：`internal/storage/` 里同一个“postgres `$N` vs sqlite `?` 占位符”问题存在**三种不同解法**：

- **方案 A — 双实现拷贝**（`postgres/`+`sqlite/`）：agent/room/session 各一份，五对文件总计 ~1690 行，diff 后**业务逻辑逐字相同**，差异仅限：
  - 占位符 `$1`↔`?`
  - `now()`↔`CURRENT_TIMESTAMP`
  - `INSERT ... ON CONFLICT DO NOTHING`↔`INSERT OR IGNORE`
  - 一个 helper：`joinPostgresPlaceholders`↔`joinPlaceholders`
  - （`repository_room.go` 705 vs 704，diff 仅 71 行；`repository_agent.go` 428 vs 427，diff 103 行；其余三对同理）
- **方案 B — 单实现 + dialect helper**（`provider/repository_dialect.go` 32 行）：`bind(index)`/`trueValue()`/`currentTimestamp()` 等小函数，**一个仓储文件同时跑两种库**。
- **方案 C — 无 dialect 抽象**（auth/goal/skills/connectors/usage/workspace/jsoncodec）：单包仓储，看不出怎么处理方言（待核验是否只跑单库）。
- 另外 `automation/repository.go:196-209` 又**自己重写**了一遍 `bind/bindList`，没用 provider 的那份。

**净重复估算：方案 A 里 ~1320 行是纯拷贝。**

**【与 06-16 决定的张力】** 06-16 明确写：“不建议马上合并 SQLite/PostgreSQL Room repository；双 dialect 直接合并容易引入 SQL 差异回归。” 本轮尊重该顾虑，但补充三点新证据：

1. **provider/ 已经用方案 B 跑了一年没回归**——证明 `bind()` 合并的风险被高估了，至少对非复杂查询可行。
2. **room/agent 的公共 scanner/aggregate helper 早已收敛到 `roomrepo/scan_room.go`**——已经走在合并路上，只剩 SQL 字符串本体还分两份。
3. 真正的回归风险点是 `ON CONFLICT`↔`INSERT OR IGNORE` 和 `now()`↔`CURRENT_TIMESTAMP` 这类**少数 SQL 语义差异**，这些可以被 dialect helper 精确封装（正是 provider/ 在做的）。

**建议（交裁决，非命令）**：

- **最小动作**：把 `provider/repository_dialect.go` 提为 `internal/storage/dialect.go`，让 automation 复用（去掉 `automation/repository.go:196` 的重复 `bind`）。**零风险、立即可做。**
- **中等动作**：agent/session 用方案 B 重写（查询简单、return 快），保留 room 双实现。**~-400 行。**
- **大动作**：room 也迁到方案 B，删 postgres/+sqlite/。**~-1300 行**，但需逐查询迁移 + 回归测试，正是 06-16 担心的。建议按 room_load → room_delete → room → agent → session 分片推进，每片配 `make check-backend`。
- **文档化**：无论选哪条，把“为什么 agent/room/session 双实现、provider 单实现、automation 自写”写进 `internal/storage/README`，消除新人困惑。

### S2. `runtime/` 是 5 个子系统挤在一个 package 树

**现状（🔍）**：`internal/runtime/` 108 个文件、10827 LOC，实际混了 5 个独立子域：

1. **会话/round 生命周期**（manager*.go、executor_round*.go、guidance、contextual_input、goal_usage）~2500 LOC —— 真正的 "runtime"。
2. **SDK bridge**（manager_client.go、clientopts/、provider/、diagnostics_env、stderr_line、debug_message*.go ~1000 LOC）—— SDK 适配 + 可观测，是 bridge 不是 runtime。
3. **permission**（permission/ 793 LOC）—— 已正确抽出子包，健康。
4. **进程内 MCP 服务**（mcp/ 共 6078 LOC，automation/connectors/goal/imagegen/room 五个域，各有 contract/tool/internal 小树）—— 占了 108 文件的大头，是个自洽子系统。
5. **厂商连接器胶水**（mcp/connectors/tool/feishu_docx_*.go ~620 LOC）—— Feishu docx/bitable/drive/wiki/sheet 工具，塞在 runtime 里。

**【与 06-16 决定的张力】** 06-16 只审了 manager.go + executor_round.go（“第一轮结构风险已解除”），**没有覆盖 mcp/ 子树**——而那才是 108 文件的主体。所以这条不与 06-16 冲突，是**补充盲区**。

**建议**：

- 把 `runtime/mcp/` **提升为顶层 `internal/mcp/`**（mcp/ 的五个域自带 contract/tool/internal，本就是自洽结构）。
- `debug_message*` + `clientopts` + `provider` + `diagnostics_env` 收进 `internal/runtime/bridge/`。
- `mcp/connectors/tool/feishu_docx_*.go` 挪到 `internal/connectors/feishu/mcp/`（与既有 `internal/connectors/feishudocx/` 同侧）。
- 重组后真正的 `internal/runtime/` 收缩到 ~25 文件 / ~2500 LOC，只管“对 session state 编排 SDK round”。

`runtime/permission/request.go:332` 还混了 8 个通用 helper（normalizeString/firstNonEmpty/normalizeBool/cloneMap...），它们和 message/helpers.go 重复——挪进 S6 的共享 `strx`/`valuecoerce`。

### S3. `room/` 是 god-package（即便已拆 57 个文件）

**现状（🔍）**：`internal/service/room/` 57 文件、8345 LOC，至少混 7 个关注点：room/conversation CRUD、实时 round/slot 编排（execution/interrupt/slot_state/execution_slot_status）、公区 mention/wake（public_mentions/directed_message_wake）、定向/私聊（directed_message/private_domain）、goal runtime 集成（goal_continuation/goal_runtime/goal_runtime_usage）、input queue 分发（input_queue*）、memory/attachments/settings。

**【与 06-16 决定的张力】** 06-16 明确：“不建议把 internal/service/room 拆成多个 package；当前 room 是聚合域，同包拆文件更稳。” 本轮的观察是：**同包拆分已经做到极致（57 文件）**，但导航仍困难，因为关注点本质不同而非“一个域的多个切面”。file 级职责大多清楚（execution.go 430 行确是“round 执行”），但 package 级是 7 个域混居。

**建议（弱推荐，交裁决）**：若团队仍偏好同包稳定性，至少在 room/ 内加一个 `README` 标注文件分组（CRUD / realtime / queue / private / goal），让 57 文件可索引。若愿意走包级拆分，最自然的缝是抽 `room/realtime/`（execution + interrupt + slot + goal_runtime*），其余留 room/。**LOC 中性，收益是认知可导航性。**

> 注：06-16 已经把 execution.go、chat.go、input_queue.go、round_state.go 做了同包分片，工作没白做——本条是说“同包已到上限，下一步要么接受要么上提包级”。

### S4. `conversation/` 整包死代码

**现状（🔍，待二次核验）**：`internal/service/conversation/dispatcher.go`、`coordinator.go`、`model_conversation.go` 里的 `Dispatcher`/`RoundCoordinator`/`UnifiedRequest`/`UnifiedInterruptRequest`/`DMHandler`/`RoomHandler` 在整个 `internal/`+`cmd/` **零外部调用方**（grep 验证；channels 有自己的 `DMHandler` 是同名不同类型）。包里真正在用的只剩 `attachment.go` + `titlegen/` 子包。

**建议**：直接删三个文件（**~-165 行，零风险**），删前补一次 `grep -rn 'NewDispatcher\|NewRoundCoordinator\|UnifiedRequest'`。包名也可考虑改成 `attachment/` 或把 `attachment.go` 并进 dm/room（待看是否被双方共用）。

### S5. `protocol/` 混入 appserver 传输模型（自定边界违规）

**现状（🔍）**：`internal/protocol/model_appserver_rpc.go`（89）+ `model_goal_appserver.go`（148）是 Codex app-server 的 JSON-RPC **传输**模型（`AppServerJSONRPCRequest`、自定义 `UnmarshalJSON` 的 `AppServerRequestID`、`ThreadGoal*Params`）。

AGENTS.md 写明：`internal/protocol` “只放跨 HTTP/WebSocket/前端/运行时边界共享的协议模型”。appserver JSON-RPC 线规是**到 SDK bridge 的服务内部传输**，不跨前端/WS，**违反了仓库自己的约定**。

**建议**：迁到 `internal/service/goal/appserver/` 或 `internal/runtime/appserver/`。**LOC 中性，修边界。**

> 06-16 审过 protocol 的 automation 模型并决定留下（合理，automation 确跨边界）；appserver RPC 是新发现，不冲突。

### S6. 散落重复 + 死依赖 + 双 WS 库

- ~~**死依赖 gogo/protobuf**~~（**已更正 2026-06-22，经 Codex 交叉核验**）：原判断错误。`go mod why -m github.com/gogo/protobuf` 实际链路是 `internal/service/channels/adapters → larksuite/oapi-sdk-go/v3/ws → gogo/protobuf/gogoproto`——它是飞书 WS 适配器的**传递依赖**，go.mod 已正确标 `// indirect`，**不可删**。原判断用的是无 `-m` 的 `go mod why`（按包名查，对传递依赖无效）+ grep（只扫本仓源码），方法错。
- **双 websocket 库（✅）**：`gorilla/websocket`（channels 的 transport/client.go、wecom_bot_socket.go）与 `coder/websocket`（handler/websocket/* + shared/sender_websocket.go）。两个子系统各用一套。建议统一到 `coder/websocket`（更活跃），但 channels 侧迁移面较大，列为中期项。
- **小工具重复（🔍）**：`firstNonEmpty` 4 份、`normalizeString`/`cloneMap` 2 份、`projectRoot()` 2 份、`bind()` 2 份（见 S1）。抽 `internal/infra/strx`（字符串）+ `internal/util/anyval`（any 归一化）。这类重复单看很小，但加起来 ~100 行且每天都在被复制新的一份。
- **`any` 用量 2698（✅ 扫描）**：看似吓人，实际高度集中在 MCP tool `schema.go`（JSON schema 本就要 any）和测试文件。**不是系统性坏味道**，不必动。唯一值得看的是 `internal/storage/{postgres,sqlite}/repository_agent.go`（各 25）——与 S1 合并后自然消失。

---

## 分域补充发现

### storage

- `workspace/` 是 storage 最大子包（4587 LOC / 32 文件），06-16 已做 facade/overlay/transcript 拆分，结构债主要剩 dialect 一致性（见 S1）。
- `agentrepo/`（132）+ `roomrepo/`（316）已是 dialect 无关的 model/patch/ID-gen 共享层——**S1 合并的天然落点就是这两个包**。

### service/room + service/channels（🔍）

- room 内部细节见上方清单：`goal_runtime*` 的错误过滤惯用句重复 6+ 次（抽 `isExpectedGoalError` ~-40 行）、三个 trim+dedupe helper 重复（`normalizeRoomDirectedMessageRecipients`/`normalizedPrivateDomainAgents`/`roomDirectedMessageWakeTargetAgentIDs`，抽 `uniqueTrimmed` ~-25 行）、`execution_slot_status.go` 三处构造同形 result message（抽 `buildSlotResultMessage` ~-25 行）。
- channels 包级结构**反而健康**（contract/message/transport/typingloop/deliveryroute/management/adapters 分得清楚）。问题在两处内部：
  - adapters/ 里 6 个适配器的 base 字段 + 5 个 getter 逐字重复（抽 `adapters.Base`，~-120 行）。
  - channels 包根混了 ingress ledger / pairing / login / account / config 等**控制面**服务（`service_ingress_ledger.go` 270、`service_pairing_store.go` 216、`service_login_*` 等），它们不是 channel。建议挪 `channels/control/`（management/、contract/ 已是子包，这步是补齐）。

### service/ 能力服务（🔍）

- **死代码**：见 S4（conversation 整包）。
- **connectors/catalog.go（367）vaporware**：~270 行 `coming_soon` 条目 + 95 行中文 feature 文案，非真实集成。保留已上线、文案挪 docs。~-200 行。
- **provider/catalog_provider.go 名不副实**：内容是静态 LLM preset，不是 runtime catalog，与 connectors/catalog.go 撞名误导。改名 `presets.go`（零行为变更）。
- **provider vs connectors vs channels 边界**：三者**确实是三个不同关注点**（LLM API 配置 / SaaS 集成 / 消息投递），不是重叠。唯一问题是“catalog”撞名——改名解决。
- **tiny packages 逐一裁决**（🔍 全部读过的结论）：

| 包 | 文件/LOC | 裁决 |
|----|---------|------|
| `goalobjective` | 2/317 | 保留（隔离 llm+preferences 依赖），可考虑移到 `goal/objective/` 表达父子关系 |
| `llm` `usage` `loops` `preferences` | 各 ~210-290 | 保留（≥2 调用方、单一关注点） |
| `runtimeselection` `sessionresume` | 各 ~115-150 | 保留（dm+room 共用） |
| `nxsruntime` | 2/80 | 保留（handler/core 用） |
| `toolpolicy` | 2/430 | 保留（dm/room/automation 共用匹配逻辑） |

  **结论：tiny packages 都不是 yagni**，06-16/06-17 也没把它们标成债，不动。

- `skills/service_registry_legacy.go`（158）是一次性迁移码（`TODO(skill-legacy-registry): 存量数据完成迁移后移除`），但每次 `loadExternalRecords` 都跑一遍。迁移完成后整文件删；迁移完成前建议改成一次持久化 flag 而非每次扫盘。

### runtime + message + protocol（🔍）

详见 S2/S5 + 删除清单。补充：

- `message/segment_assistant.go`(398) 和 `processor.go`(383)：**各自单一关注点**，398 行里 stream-slot/index 冲突处理是真边界逻辑，**不是 god-file**，不动。
- `protocol/model_session_key.go`(367)：8 个 channel 常量 ×2 + parse/build/validate，**合理**，只是内部两张 switch 表重复（见清单 shrink 项）。
- `runtime/mcp/automation/tool/` 20 个文件里 9 个只含一个 `func`——over-split，但符合 AGENTS.md 的 `command_*.go` 命名约定，borderline。

### handler / cli / app / chat（🔍）

- **handler 健康且薄**：grep 确认 handler 零直接 import storage，全走服务注入，18 个子包按 resource 分得清楚。唯一结构味：`handler/core/handlers.go`(344) 名为 core 实际塞了 4 个不相关域（health/system、preferences、nxs-runtime、**provider CRUD 占 12/18 个 handler**）。**建议把 provider handler 挪 `handler/provider/`**，core 收缩到 ~80 行。LOC 中性，边界清晰。
- **cli 健康**：29 文件无 god-command，`command_automation.go`(348) 只是子命令树装配。不动。
- **app/server 健康**：`AppServices` 是个扁平 DI struct（23 字段，无反射/无图解析），不是过度工程的容器。可选美化：把 23 个能力服务字段分组（core 已分组，再分一组 capability）。`goal_resume.go`/`goal_interrupt.go` 疑似 domain 逻辑漏进 app 层，待核验是否该回 `service/goal/`。
- **chat ↔ service 边界干净**：`chat/dm`+`chat/room` 是纯 domain（model/mapper/projection），service 单向 import chat，反向零，storage 零。教科书式分层，不动。

### 前端 web/src（🔍）

- **`use-agent-conversation.ts`(1131) 仍是 god-hook**：39 个 useCallback + 15 个 useEffect，混了 5 个关注点（state+ref 镜像、ws 生命周期、runtime machine 调度、历史分页、出站 action 面）。06-16 已从中抽出 11 个 sibling 文件并决定“避免再拆出薄 hook”——但**编排层仍是瓶颈**。若愿意继续拆，缝是 4 个子 hook（`useAgentConversationState/Socket/Runtime/Actions`），编排器降到 ~150 行。**【与 06-16 决定的张力】**：06-16 选择不再拆，本轮给出具体可拆缝，交裁决。
- **dm-chat-panel(388) vs group-chat-panel(500) 共享 ~70% 骨架**（同样的 useAgentConversation/provider/history/scroll/composer 接线）。抽 `<ChatSurface mode="dm"|"group">`，~-250 行。
- **`utils.ts` 的 `group_messages_by_round` 与 `group_room_messages_by_round`** 仅差一个预处理，合并一个参数化 helper，~-60 行。
- 其余大文件（composer-panel 673、provider-settings-panel 671、sidebar-wide-panel 521、editor-panel 517）多为 JSX 表面积而非逻辑密度，**不算 god-file**，与 06-16 判断一致。
- i18n `messages.{zh,en}.ts` 已按 locale 拆开（各 790），key 对齐，**健康**。

---

## YAGNI / 单实现接口清单（🔍 待逐个核验调用方）

仓库共 132 个 interface 声明。下面是高置信度的“单实现/纯转发”候选，删前各跑一次 `grep -rn '<InterfaceName>'`：

| 接口 | 位置 | 理由 |
|------|------|------|
| `Factory` | runtime/manager_client.go:27 | 唯一实现 defaultFactory，唯一调用方是 NewManager 自己 |
| `RoundMapper` pair | room/round_mapper.go + dm/service_round.go | 两个 15 行 adapter 转发同一个 Processor，抽象过薄 |
| `feishuEventClient` + factory | channels/adapters/feishu_channel.go:42 | 仅为测试假对象存在 |
| `ExternalSessionNotifierFunc` | channels/ingress.go:31 | Func adapter 除非有裸函数调用方否则 yagni |
| `replacementAdoptingChannel` | channels/router_registry.go:56 | 单一实现待 grep 确认 |
| `AgentScopedDeliveryChannel` | channels/contract/model.go:119 | 单一 type-check 使用 |

> runtime/executor_round_model.go 的 `RoundExecutionRequest`（14 字段）里 `AfterQuery`/`ObserveIncomingMessage`/`SyncSessionID` 三个可选回调 slot 是否真有调用方接线，待核验，没人接就删。

---

## 屎山预警清单（watchlist，按“若扩张则优先治理”排序）

1. **storage dialect 三方案并存**（S1）——最大的认知债，每个新人都要问“为什么 agent 双实现、provider 单实现”。
2. **runtime/mcp/ 嵌套在 runtime/**（S2）——6078 LOC 的子系统藏在 runtime 命名空间下，误导“runtime 是什么”。
3. **conversation/ 死代码**（S4）——零调用方还占着包名，最易清。
4. **channels 包根的控制面文件**——pairing/login/account/config 不是 channel 却长在 channels/。
5. **handler/core 的 provider CRUD**——core 不 core，误导路由查找。
6. **散落小工具重复**（S6）——每天都在被复制新的一份，治理成本只会升。

不在 watchlist（已健康，勿动）：chat↔service 分层、handler 整体薄度、cli 命令树、app/server DI、tiny packages、i18n、permission 子包、message processor/segment、protocol automation 模型。

---

## 净收益估算与优先级路线图

| 优先级 | 动作 | 可减 LOC | 风险 | 验证 |
|--------|------|---------|------|------|
| P0 | 删 conversation/ 死代码（删前再 `grep -rn 'NewDispatcher\|RoundCoordinator\|UnifiedRequest'` 确认） | ~-165 | 极低 | `make check-backend` |
| P0 | 提 `storage/dialect.go`，automation 复用 bind() | ~-15 | 极低 | `go test ./internal/storage/...` |
| P1 | 抽 `infra/strx` 收敛 firstNonEmpty/normalizeString/projectRoot；goal reset×3 合一；manager_goal_accounting 3→1；processor_system 8 wrapper 内联 | ~-220 | 低 | `make check-backend` |
| P1 | channels 抽 `adapters.Base` + `transport.ChunkedDelivery` + `isExpectedGoalError` | ~-235 | 中（触及多适配器，逐适配器迁） | `go test ./internal/service/channels/...` |
| P1 | connectors/catalog.go 去 coming_soon + 文案挪 docs | ~-200 | 低 | 前端 typecheck |
| P2 | agent/session 仓储迁方案 B（room 暂留双实现） | ~-400 | 中（逐查询迁 + 回归） | `make check-backend` |
| P2 | protocol appserver RPC 模型迁出；handler/provider 从 core 拆出 | LOC 中性 | 低 | `make check-backend` |
| P2 | 前端 `<ChatSurface>` 合并 dm/group；utils.ts 合并 | ~-310 | 中 | `pnpm --dir web lint typecheck` |
| P3 | runtime/mcp/ 提为 internal/mcp/；feishu 工具挪 connectors/ | LOC 中性 | 中（import 路径大改） | 全量 test + 启动冒烟 |
| P3 | room 仓储也迁方案 B（删 postgres/+sqlite/） | ~-900 | 高（06-16 明示顾虑） | 分片迁移，每片 `make check-backend` |
| P3 | use-agent-conversation.ts 拆 4 子 hook（与 06-16 决定冲突，交裁决） | ~-150 | 中 | `pnpm --dir web lint typecheck` + 手动冒烟 |
| P3 | 统一 websocket 库到 coder/websocket | ~0 | 中（channels 迁移面大） | 连接/重连冒烟 |

**合计粗估：可减 ~2500–2800 行**（P0+P1 ~-835 行低风险；P2 ~-710 行中风险；P3 ~-1050 行 + 若干 LOC 中性重组，高风险）。

> storage 合并（P3 的 ~-900 + P2 的 ~-400 = ~-1300）是最大单项收益，也是最需要维护者拍板的——因为它直接挑战 06-16 的明确决定，需要团队重新评估 provider/ 方案 B 成功一年后的风险预期。

---

## 未核验项 / 待办

- ⚠️ conversation/ 死代码的“零调用方”结论基于子 agent grep，提交删除前本人需再跑 `grep -rn 'NewDispatcher\|RoundCoordinator\|UnifiedRequest\|UnifiedInterruptRequest' internal/ cmd/`。
- ⚠️ 方案 C 的 storage 包（auth/goal/skills/connectors/usage/workspace）实际跑单库还是双库未核验——若只跑单库，S1 的不一致性比写出来的小。
- ⚠️ `app/server/goal_resume.go`/`goal_interrupt.go` 是否为漏到 app 层的 domain 逻辑，仅看文件名推断，未读内容。
- ⚠️ YAGNI 接口清单均需 `grep` 调用方二次确认。
- ⚠️ 前端 `features/conversation/operation/` 未读。

---

## 附录：方法论

- **已安装 ponytail skill**（`/ponytail-audit`、`/ponytail-review` 等 6 个，源在 `~/program/ponytail`，symlink 进 `~/.claude/skills/`；下次会话起 `/ponytail-audit` 可直接调用）。
- 量化基线：1046 个 Go 文件；internal/service 49548 LOC/349 文件（最大）、internal/storage 13291、internal/runtime 10827、web/src 80553 LOC/507 文件。
- 分四个并行子审计（channels+room / runtime+message+protocol / 能力服务 / 前端+cross-cutting），每域返回 `tag: what. replacement. [path:line]` 证据。本人直接验证了 storage diff、`go mod why gogo`、`any` 分布、双 WS 库引用。
- 本轮**未改任何代码**，纯评审 + 产出本文档。
- **更正记录**：初稿把 `gogo/protobuf` 列为死依赖（delete 项），经 Codex 交叉核验 + `go mod why -m` 证伪——它是 `larksuite/oapi-sdk-go/v3/ws` 的传递依赖（飞书 WS 适配器需要）。**教训**：判定依赖可删必须用 `go mod why -m`，或删后 `go mod tidy` + `go build ./...`，不能只靠 grep 本仓源码 + 无 `-m` 的 `go mod why`（后者按包路径查，对传递依赖永远报“不需要”）。
