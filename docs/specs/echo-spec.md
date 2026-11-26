# Nexus 主动跟进（Echo）模块设计规范

- 状态：**Implemented / 主窗口 opt-in**
- 产品名：中文 **主动跟进**，英文 **Echo**；内部模块名 `echo`
- 产品定义：**Agent 会在合适的时候主动跟进值得继续的对话。**
- 当前范围：只在主智能体主窗口生效（见 [main-agent-spec](main-agent-spec.md) §3）；Echo 是主窗口 inbox 中宿主写入的 `echo` 事件，message-only、延迟提交、用户输入抢占和常规设置全局开关已接通。投递结果直接保留在主窗口，不提供运行记录页面。
- 不支持：其他 Agent 的 DM、Room 群主（仅保留 §15 阶段设计）、外部 IM、事件型 Signal、工具型主动工作。agent 自己要求“稍后回来看”的场景由主窗口跟进（`follow_up`）承担，不属于 Echo。

## 1. 产品语义

Echo 不是提醒器，也不是让 Agent 定时找话说：对话暂停后仍有明确、具体且有价值的未完成语境时，Agent 在不打扰用户的前提下把它接回来。

“值得继续”必须同时满足：

1. 跟进与当前对话直接相关；
2. 存在尚未闭合的问题、承诺、决定或用户明确要求的后续；
3. 现在跟进比保持沉默更有价值；
4. 跟进不只是复述、寒暄、催促或制造参与感；
5. Agent 只靠当前对话就能安全完成这次表达。

默认保持沉默。成功标准不是发送更多消息，而是只在真正需要时出现。

## 2. 目标与非目标

### 2.1 目标

- 主智能体在主窗口中自然延续未完成语境；
- 把“是否值得说”与“具体说什么”分开，降低误触发和人格漂移；
- 提供全局显式开关与内部活跃时段；状态全部落在主窗口 inbox 事件上，不另建尝试表与调度器；
- 用户产生新输入时立即让路；
- 在服务重启、超时和重复调度下保持最多一次可见投递；
- Room 群主阶段复用同一领域模型，但不提前引入多人路由。

### 2.2 非目标

- 不替代 Automation、日历提醒、监控任务或事件触发工作流；
- 不替代 Goal continuation，不负责让未完成 Goal 自动继续执行；
- 不做 Agent 的后台记忆整理或人格自我更新；
- 不为提高活跃度发送泛化问候、情绪陪伴或“还需要帮助吗”；
- v1 不调用工具、不访问 Connector、不产生外部副作用；
- v1 不支持外部 IM、Room、多 Agent 自选发言或随机成员唤醒；
- 不保存或展示模型思维链、内部独白或完整 gate prompt。

## 3. 与现有能力的边界

| 能力 | 触发来源 | 执行上下文 | 主要产物 |
| --- | --- | --- | --- |
| Echo | 对话结束后的安静窗口，由系统判断是否值得继续 | 主窗口的同一 Session | 一条可见跟进消息，或保持沉默 |
| 跟进（follow_up） | 主智能体在轮内用 `nexus.command` 给自己设定的一次性时刻 | 主窗口的同一 Session，正常工具权限 | 一轮带备注的普通工作 |
| Automation | 用户明确配置的时间、周期或监控条件 | 任务绑定或隔离 Session，持有任务权限快照 | 任务结果、通知或经批准的工具行为 |
| Goal continuation | 已存在 Goal 的受控延续条件 | 绑定 Goal authority 的内部 round | Goal/Execution 推进 |

- Echo 不把主动跟进伪装成定时任务；它与跟进、main 目标定时任务共用主窗口 inbox 的未来事件与到点唤醒。
- 周期 Heartbeat 已删除；main 目标定时任务写入主智能体主窗口 inbox。

## 4. 核心不变量

1. **用户明确开启**：全局默认关闭；产品可以在引导时询问，但不得静默开启。
2. **同一身份**：最终消息必须由原 Agent 在原 DM Session 中生成，不能由 gate 模型代写。
3. **同一语境**：不创建合成收件箱、匿名 main session 或新的用户对话。
4. **默认静默**：任一判断不确定、上下文不完整或基础设施异常都不产生可见消息。
5. **用户优先**：任何用户输入进入主窗口 inbox 时，立即把该 inbox 中 pending/delivering 的 `echo` 事件置为 `cancelled`；Echo 从不在安全点注入正在运行的轮次。
6. **没有伪造输入**：隐藏 wake 只作为 synthetic round marker 保存，不作为可见用户消息。
7. **没有工具**：v1 round 使用 message-only 策略；模型要求工具时本次静默。
8. **最多一次**：同一 Session、trigger 和 anchor 最多产生一条可见 Echo 消息。
9. **投递前复核**：生成完成不代表可以发送；最终提交前必须再次验证 inbox 中没有待处理的用户输入。
10. **不存内部思维**：数据库与日志只保存结构化决定，不保存完整 gate 输入、思维链或隐藏 prompt。

## 5. 首期用户故事

### 5.1 应该跟进

- Agent 问了完成当前事情所必需的明确问题，用户暂未回答；
- Agent 明确承诺稍后回来确认，且对话中存在可验证的待确认点；
- 用户明确邀请一次没有指定时间、周期或外部条件的跟进，例如“如果我没回，可以之后再问我一次”；
- 对话内决定卡在清晰选项上，简短跟进能帮助用户继续。

带明确时间、周期、监控条件或外部状态的请求应创建 Automation，不进入 Echo。

### 5.2 应该保持沉默

- 对话已自然结束，最后一句只是致谢、确认或告别；
- Agent 只能重复上一条回复，或发送“还需要帮助吗”；
- 需要联网、调用工具或读取新外部状态才能产生价值；
- 涉及高风险医疗、法律、财务、危机干预或敏感关系建议，且用户没有明确要求后续；
- 用户已开始新 round、关闭 Echo，或主窗口正在处理其他工作；
- 只能根据模糊语气猜测用户希望被联系。

## 6. 领域模型

### 6.1 名词

| 名词 | 含义 |
| --- | --- |
| Settings | 当前用户的 Echo 全局开关 |
| Policy | 系统内部维护的时段策略 |
| Anchor | 产生候选的可见用户 round |
| Echo 事件 | 主窗口 inbox 中 `kind=echo` 的宿主事件，`available_at` 为到点时间 |
| Gate | 只决定是否值得跟进及 focus 的后台小模型调用 |
| Echo round | 主智能体在主窗口收到的隐藏、无工具、延迟提交 round |

### 6.2 整体链路

```text
可见用户轮成功 ──ScheduleAfter──> echo 事件 pending（available_at = 结束时间 + 安静等待，顺延进活跃时段）
                                       │ 任何用户输入 → cancelled
到点且主窗口空闲 ──claim──> delivering ──Decide（锁外：开关、活跃时段、Gate）
        │ 关闭 / skip / 出错 → consumed（静默）
        │ 活跃时段外 → 退回 pending 并改写 available_at
        └ follow_up ──锁内 CAS delivering→consumed──> Echo round（隐藏、无工具、延迟提交）
                                                         └ Admit：无回复标记或有待处理用户输入 → 丢弃
```

### 6.3 事件状态

Echo 复用主窗口 inbox 的状态机（pending → delivering → consumed，另有 cancelled）：

- `pending`：等待 `available_at`；
- `delivering`：到点后被 drain 认领，正在跑 Decide；Decide 是一次模型调用，必须在派发锁外执行；
- `consumed`：已静默收口，或已通过 CAS 起 Echo round；
- `cancelled`：用户输入在 pending 或 delivering 期间到达，或同一主窗口写入了新的 echo 事件。

起轮前在派发锁内做条件迁移 delivering → consumed；迁移失败说明期间用户已开口，本次不起轮。若此时已有其他轮在跑，事件直接 cancelled。

## 7. 持久化设计

### 7.1 用户 Preferences

- 全局开关复用用户级 `.settings/preferences.json` 的 `echo_enabled` 字段，默认 `false`；与其他用户偏好共享同一份带版本的原子持久化，不另建 Echo 设置表。
- 时区、活跃时段与安静等待是产品内部策略，不从用户 API 读取，也不按用户持久化。
- 时区依次使用服务端 `DefaultTimezone` 与 `Asia/Shanghai`；其余默认值见 §17。
- 关闭开关不需要收口：已安排的 echo 事件到点时 Decide 读到关闭即静默 consumed。

### 7.2 Echo 事件

Echo 事件是 `agent_inbox_events` 的一行（见 main-agent-spec §3），不另建表：

| 字段 | 取值 |
| --- | --- |
| `kind` | `echo` |
| `dedupe_key` | `echo:` + anchor round ID，同一 anchor 最多一条 |
| `available_at` | 到点时间；活跃时段外顺延时改写 |
| `payload` | 空：Echo 的上下文在到点时从主窗口历史读取 |

每个主窗口同时最多一条待定 Echo：写入新事件前先取消旧的。不保存对话正文、gate prompt、focus、完整模型响应或思维链。

## 8. 产生候选

主窗口轮次结束后由 dm service 判断并调用 `EchoPolicy.ScheduleAfter`：

- round 来自可见用户输入（`Internal=false`），成功结束且由 assistant 完成；
- 会话是主智能体主窗口；
- 用户已开启 Echo。

内部 Goal、Automation、跟进、定时任务、Echo、recovery round 不会产生 Echo。

- `available_at` = 轮次结束时间 + 安静等待；落在活跃时段外时顺延到下一个活跃窗口开始。
- 不设冷却与每日上限：每条 Echo 都锚定一轮新的用户输入，用户开口即取消，节奏天然有界。

## 9. 到点与预检

- 到点唤醒由 dm service 的主窗口 inbox 定时器承担（[`internal/infra/duework`](../../internal/infra/duework/loop.go)）：启动时退回遗留 delivering 并立即 reconcile，写入未来事件后 `Notify`，之后只按 inbox 最近的 `available_at` 唤醒。
- 主窗口空闲时，drain 发现队首是 echo 事件即认领并在后台运行 Decide；运行中到点的 echo 等本轮结束后再处理，从不进入安全点注入。
- Decide 按固定顺序检查：开关仍开启 → 当前在活跃时段内（否则退回 pending 并顺延）→ 读取主窗口历史 → Gate。

## 10. Gate 合同

### 10.1 输入

Gate 使用当前配置的后台快速模型，不新增 Provider 配置。输入只包含：

- anchor 前最近最多 12 条可见消息；
- 最多 24,000 个 Unicode 字符；
- 受控产品规则与敏感场景边界。

上下文按主窗口历史读取。Gate 不读取附件、其他 Session、Connector 或 workspace。

### 10.2 输出

Gate 只允许严格 JSON：

```json
{
  "decision": "follow_up",
  "reason_code": "awaiting_answer",
  "focus": "询问用户是否已经确定部署窗口"
}
```

`decision` 只有 `follow_up` 和 `skip`。

| 类别 | reason_code |
| --- | --- |
| 正向 | `awaiting_answer`（Agent 已提出完成当前事情所需的明确问题）、`promised_followup`（Agent 作出明确后续承诺）、`unfinished_decision`（清晰决定或选项尚未闭合）、`requested_check_in`（用户明确要求稍后回来跟进） |
| 静默 | `concluded`、`no_new_value`、`would_repeat`、`social_only`、`too_ambiguous`、`sensitive_context`、`needs_tool` |
| 宿主写入 | `invalid_output`（解析失败）、`gate_unavailable`（Gate 不可用）；不是模型可返回值 |

- `requested_check_in` 仅表示用户邀请一次无明确时间条件的跟进。出现具体时间、周期、监控条件或外部状态时，gate 必须返回 `needs_tool`，由正常对话引导用户使用 Automation。
- Gate 依次衡量相关性、信息缺口、预期价值、时机、重复风险和用户负担。只有存在一个具体 focus、且正向价值明显高于打扰成本时才返回 `follow_up`；不确定时必须 `skip`。

### 10.3 故障语义

- 严格解析失败：以 `invalid_output` 静默结束。
- 超时、配额不足或模型不可用：以 `gate_unavailable` 静默结束并记录日志。
- 不自动重放失败 Gate。

## 11. Echo round

### 11.1 派发

Gate 通过且 CAS 成功后，dm service 在主窗口起轮。请求语义：

```go
dm.Request{
    SessionKey:           mainWindowSessionKey,
    Internal:             true,
    ExecutionOrigin:      "echo",
    InputOptions: sdkprotocol.OutboundMessageOptions{
        HiddenFromUser:  true,
        Synthetic:       true,
        Purpose:         "echo_followup",
        ToolAccess:      "none",
        MaxOutputTokens: 400,
        Metadata: map[string]string{
            "echo_event_id": eventID,
        },
    },
}
```

隐藏输入告诉 Agent 这是系统触发的 Echo 候选，不是用户新消息，也不是对上一条消息的即时回答。对措辞的要求：

- 像真正记得前文的人自然重新开口：先用贴合语境的轻量寒暄、回想或过渡承接上文，再带来一个具体新价值（更清楚的判断、可执行的小建议、推动决定的问题或兑现先前承诺）。
- 开头可参考“对了……”“顺着刚才那件事……”“我又想了一下……”的语感，但不得固定套用模板。
- 保持原 Agent 的人物性格和会话语言；通常 2 到 5 句，不用无必要的标题、清单或总结腔。
- 不得说“你还没回复”、责备用户、制造紧迫感、泛泛询问“还需要帮助吗”，不得虚构等待时长、新事实或用户意图。
- Gate 的 `focus` 只是内部方向，不能照抄或当作用户事实。
- 无法自然增加价值、只能生硬重复问题或已不值得发送时，返回控制标记 `<nexus_echo_no_reply/>`。

最终措辞由主智能体 runtime 生成，保留 persona、模型、上下文和正常 Markdown 能力。

### 11.2 Message-only policy

Echo round：

- 不挂载 MCP、Connector、communication、Goal、Execution 或 Automation 工具；
- 不允许权限申请；
- 最多一个 Agent turn；
- 不允许 subagent；
- 不允许写 workspace、记忆或配置；
- 需要任何工具才能产生价值时返回 no-reply。

输出与控制标记：

- 输出预算不超过 400 completion tokens；宿主不截断 Markdown；超出预算或没有完整 final 时按 no-reply 收口。
- `<nexus_echo_no_reply/>` 只有在 trim 后与完整输出精确相等时才是控制标记；正文中偶然出现该字符串不能吞掉正常消息。

实现约束：

- 不得通过复制 Agent 当前 allow/deny 列表实现；由宿主签发不可扩大的专用 message-only policy。
- 普通工具白名单的非 nil 空切片只表示“快照中没有规则”，不等于禁用全部工具，不能充当 deny-all。
- Bridge 与 runtime 通过协商后的 `message_execution_policy_v1` 执行 `tool_access=none` 和 `max_output_tokens`。
- runtime 在构造工具目录前禁用内置工具和 MCP，并跳过 Stop Hook、记忆召回/维护、自动压缩和续写。
- 无法确认支持该能力的 runtime 必须拒绝 Echo round。

### 11.3 延迟发布与最终提交闸门

Echo assistant 输出不能边生成边显示。DM runtime 捕获本 round 的候选 final，直到提交闸门（`DeferredAssistant.Admit`）确认：

- round 成功结束，内容非空且不是 no-reply；
- 主窗口 inbox 中没有待处理的用户输入（Echo round 无工具、没有安全点，期间到达的用户输入只会排在它之后）。

- 闸门成功：宿主一次性持久化并广播 assistant message。
- 闸门失败：丢弃候选输出与本轮 runtime 消息，静默结束；排队的用户输入随后照常开跑。
- 该机制是 DM service 的 host-only deferred assistant admission，不能暴露给 HTTP/WS 调用者。用户不会看到半条 Echo stream。

### 11.4 可见消息

成功消息是普通 assistant message，额外携带受信 metadata：

```json
{
  "metadata": {
    "source": "echo",
    "echo_event_id": "inbox_..."
  }
}
```

- 中文界面只显示轻量“主动跟进”标识，英文界面显示 `Echo`。
- 隐藏 prompt、gate reason、focus 和内部事件状态不进入对话正文。

### 11.5 Transcript 与后续上下文

`HiddenFromUser` 只控制 Nexus 的可见历史投影，该轮并非无状态：

- DM service 先持久化隐藏 round marker；runtime 再把隐藏 user turn 与生成的 assistant turn 写入原 Session transcript。
- 成功投递后真实角色顺序为 `... U-A-U_echo-A_echo-U_next-A_next ...`；界面隐藏 `U_echo`，用户看到 `... U-A-A_echo-U_next-A_next ...`。后续 turn 继续携带这组 runtime 历史。
- DM service 为隐藏输入指定稳定 runtime UUID，并记录该 round 流出的全部消息 UUID。
- 最终准入成功时保留完整 `U_echo-A_echo`；未准入则在派发下一条输入前通过 runtime control 同时删除隐藏输入及其输出。no-reply、生成后被取消和 assistant 产生前中断都不会进入后续模型上下文。
- 不能只删除 Nexus overlay marker，也不能依赖相邻角色归一化掩盖语义污染。

## 12. 幂等与恢复

- 同一 anchor 由 `dedupe_key` 保证最多一条 echo 事件；同一主窗口写入新事件前取消旧事件。
- 进程启动时 delivering 退回 pending，到点后重新 Decide；Decide 无副作用，重跑安全。已 CAS 为 consumed 的事件不会复活，因此不会重复起 Echo round。
- 重复可见投递是最高优先级故障，恢复路径保持 fail closed。

## 13. HTTP 与协议面

| 方法与路径 | 用途 |
| --- | --- |
| `GET /nexus/v1/settings/echo` | 读取用户级全局开关 `{ enabled }` |
| `PUT /nexus/v1/settings/echo` | 更新用户级全局开关 `{ enabled }` |

- 路由只操作当前 owner 的设置。
- 内部时间策略使用 IANA timezone 和 `HH:mm`，跨午夜窗口有明确测试。
- v1 不提供“立即运行 Echo”接口（会混淆主动判断与普通消息发送）。
- 领域与 wire 校验集中在 `internal/echo`；HTTP handler 只做 owner-scoped 适配。
- Echo 事件只服务内部调度，不提供用户读取接口，也不进入待发送列表。

## 14. 前端体验

- 常规设置只提供一个“主动跟进”开关（英文 `Echo`），说明：`Nexus 会在主窗口安静一段时间后，主动跟进值得继续的对话。`
- 开关即时保存；失败时原位回滚，并在同一行显示 Nexus 风格错误，不改变布局高度。
- 关闭开关只取消未来跟进资格，不删除已发送的正常消息。
- 关闭请求无需额外的“停止在途跟进”步骤；结果未知时只需对账开关本身。
- 不提供“最近记录”界面；已投递消息就是主窗口中带“主动跟进”标识的普通 assistant message，用户无需理解事件、gate 或调度。

## 15. Room 群主阶段

Room 不进入 v1。后续阶段复用 Gate、延迟提交与未来事件，并增加硬约束：

1. 只有当前 `RoomRecord.HostAgentID` 可以成为 Echo actor；
2. Room 必须有独立的 `echo_enabled` 明确开关，不能复用 `HostAutoReplyEnabled`；
3. host 变更、Room 配置 revision 变化或任何新公区消息都会取消旧候选；
4. 每个 public anchor 只能有一个 host attempt，不能随机挑成员，也不能 fanout；
5. Echo 消息是普通公区表达，但不得产生 actionable `@mention`、directed message、Goal continuation、Assignment 或 Execution；
6. host Echo 同样 message-only、延迟发布、无 stream；
7. Room 菜单文案为 `允许群主主动参与`，说明仍使用产品定义句；
8. 用户关闭 root conversation 或停止相关工作时，派生 Echo 必须一并取消。

- 第 5 条必须由宿主提交策略保证：Room Echo message 携带受信的“禁止 handoff”语义，mention parser 只做展示、不创建 handoff；不能只靠 prompt 要求 Agent 不写 `@`。
- Room 的最终提交还要校验 host identity、Room authority epoch、public cursor 和 configuration version。
- 只有这些边界在 DM 阶段稳定后才实现 Room。

## 16. 安全、隐私与可观测性

- gate 输入视为当前 Session 数据，只发送给用户已配置的后台模型 Provider；
- 不把其他 Session、Room 私域、Connector 数据或 workspace 文件拼进 gate；
- 日志不记录对话正文、focus、隐藏 prompt 或 Provider 原始响应；
- `focus` 只存数据库，按 owner scope 读取，并受长度限制；
- Echo metadata 由宿主生成，runtime 同名字段必须清除；
- 支持外部 IM 前必须重新设计 active pairing、通知权限和 transport 级 opt-in；
- 审计以 inbox 事件状态与服务日志完成；聚合产品指标在出现 dogfood 校准需求后再增加。

## 17. 默认值

| 配置 | v1 默认值 |
| --- | --- |
| 全局 Echo | 关闭 |
| 安静等待 | 6 小时 |
| 活跃时段 | 09:00–22:00，policy timezone |
| gate 故障 | fail closed，不自动重放 |
| runtime 工具 | 全部禁用 |
| 输出方式 | 最终一次性提交，不流式展示 |

这些值只在内部 dogfood 中校准；公开配置仅有全局开关，不暴露调度参数或模型阈值。

## 18. 实现边界

```text
internal/echo/                         Settings 与时段策略
internal/service/echo/                 开关、ScheduleAfter/Decide（实现 dm.EchoPolicy）、Gate 与提示
internal/service/dm/main_inbox_echo.go 事件写入、取消、到点认领、CAS 起轮与提交闸门
internal/service/dm/main_inbox_timer.go 主窗口未来事件的到点唤醒
internal/handler/echo/                 owner-scoped HTTP
web/src/features/settings/general/sections/settings-general-behavior-section.tsx
```

跨 product / bridge / runtime 的窄合同，三层必须同时上线，不能只在 product 中声明“无工具”：

- product：Echo 判断、主窗口事件与 DM 提交闸门；
- `nexus-agent-sdk-bridge`：传递明确的消息执行策略；
- 默认 Go runtime：在工具目录构造前落实无工具和输出预算。

跨模块契约只有两个：

1. `dm.EchoPolicy`：`ScheduleAfter` 决定何时安排，`Decide` 决定到点时是否开口；Echo service 不持有状态、不起轮、不写库；
2. host-only deferred assistant admission，在可见提交前执行最终闸门。

## 19. 验收标准

### 19.1 功能

- 开启 Echo 后，主窗口每个符合条件的成功用户轮只留一条待定 echo 事件；
- gate `skip`、no-reply 和异常都不出现空消息或用户气泡；
- 投递的消息来自主智能体、主窗口，并显示一个 `Echo` 标识；
- delivered 消息先自然承接原会话再提供一个具体新价值，不表现为催办、客服回访或突兀追问；
- 关闭全局 Echo 后到点即静默；其他 Agent 的 DM 不产生 Echo；
- 活跃时段与跨午夜窗口行为确定；
- Echo 不调用任何工具，也不产生权限请求。

### 19.2 并发与恢复

- 新用户消息在等待、gate 和最终提交三个阶段都能让 Echo 让路；
- Echo 生成期间用户输入不会看到半条 stream；
- 同一 echo 事件只有一次 CAS 能起轮；
- gate 超时、进程重启和消息广播失败均不产生重复可见消息。

### 19.3 安全与隔离

- owner A 无法读取或修改 owner B 的设置和事件；
- runtime 无法覆盖宿主写入的 Echo source 或事件 metadata；
- gate 输入不包含其他 Session 或 Room 私域内容；
- 数据库与常规日志没有隐藏 prompt、对话副本和思维链；
- 外部 IM 和 Room 在 v1 请求中 fail closed。

## 20. 明确拒绝的方案

| 方案 | 拒绝原因 |
| --- | --- |
| 复用 Automation 定时任务事件 | 后台语义不同（用户配置的时刻 vs 对话结束后的判断），无法保证原对话身份 |
| 固定周期扫描所有 DM | 成本高、难去重，没有清晰 anchor |
| 让小模型直接写最终消息 | 绕过原 Agent persona、runtime 与真实历史 |
| classifier 出错就唤醒主 Agent | 主动打扰不是关键任务，应保持沉默 |
| 一开始开放工具 | 把消息型跟进升级成权限和副作用系统 |
| 流式展示 Echo | 用户新输入无法安全抢占，容易出现半条主动消息 |
| 保存内部 thoughts | 增加隐私负担，且不是 v1 产品价值所必需 |
| Room 随机选 Agent | 身份不可预测、容易重复发言；只能由当前群主承担 |
| 用 prompt 约束代替 durable fence | 去重、取消和权限必须由宿主与存储保证 |
| 独立 attempt 表与调度器 | 与主窗口 inbox 各管一套状态，取消与到点要跨两处对账；改为 inbox 的一种宿主事件 |
