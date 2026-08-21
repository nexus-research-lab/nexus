# 消息处理规范

## 1. 文档目标

本文档定义当前消息链路的三件事：

- 实时消息怎么流动
- 历史消息怎么落盘和读取
- 前端为什么按 round 展示和分页

本文同时定义消息进入时间线后的稳定展示顺序、并发 Agent 的快慢处理、guide/queue 控制消息的投影，以及消息正文与状态信息的密度边界。它不定义 Room 业务流程；Room handoff 的通信语义见 [Room 协作协议](./room-collaboration-spec.md)。

## 2. 核心对象

### 2.1 stream event

- 运行时实时增量
- 负责过程态，不是历史真相源

### 2.2 assistant message

- 某个 assistant turn 的 durable 消息
- 可能包含 thinking、tool、text 等内容
- assistant 正文真相源只来自 runtime transcript

### 2.3 result message

- 一轮执行的终态结果
- 包含结果文本、执行终态与 runtime 摘要
- 有效错误由 `is_error` 或 `error_*` subtype 共同决定；`success + is_error: true` 仍是失败
- result 真相源只来自 Nexus overlay
- 对外 API / WebSocket 不再直接暴露 standalone `result`
- 最终展示统一收口为 `assistant.result_summary`

### 2.4 round

- 一次用户输入触发的一轮业务对话
- 当前历史分页、状态收口都按 round 处理

### 2.5 身份边界

| 字段 | 生成方 | 语义 |
| --- | --- | --- |
| `client_request_id` | Web | 一次发送尝试的 ACK / timeout 关联，不是业务主键 |
| `client_message_id` | Web | optimistic 用户消息与幂等重试身份 |
| `round_id` | Nexus | 一次用户输入的根业务轮次，由后端生成 |
| `message_id` | Nexus / runtime | durable 消息身份，不能复用为 round |
| `agent_round_id` | Nexus | Room 内单个 Agent slot 的执行身份，与 root round 独立 |

前端不得发送或拼接 canonical `round_id`，也不得从 `message_id`、前缀或 Agent 名称
反推 root round。`agent_round_id` 必须使用显式字段，不能编码进 `round_id` 后缀。

### 2.6 投影归属

- bridge 只传递 runtime message、stream、session 与控制事件，不持有 UI turn 模型。
- Nexus 后端负责 transcript、overlay、共享消息和实时状态的产品级归一化。
- Web 只负责折叠、虚拟列表、搜索和 viewport 加载，不从多个旁路状态猜生命周期。

## 3. 实时链路

### 3.1 入口

- 前端通过 WebSocket `chat` 发起一轮执行
- 后端创建 / 复用 runtime client
- runtime 返回 stream / durable message / round status
- `chat_ack` 与用户 `input_queue enqueue` 的受理 ACK 共用 10 秒上限（常量 `protocol.RequestAckTimeoutMS`）
- `client_request_id` 标识一次传输尝试；`client_message_id` 标识同一条逻辑输入，ACK 未知后重试必须复用后者
- `input_queue` 快照只表达共享队列当前状态，不能充当请求回执；后端完成持久化后必须向请求连接单播 `input_queue_ack`
- ACK 超时表示“后端受理状态未知”，前端必须保留输入并允许用同一 `client_message_id` 重试，不能把超时当作已确认失败后直接清空草稿

### 3.2 前端展示

前端按四种投影边界处理实时内容：

- stream：增量展示过程
- durable message：写入最终消息列表，可恢复并可产生未读
- ephemeral message：只展示 round 内过程，终态到达后移除
- transient message：保留在当前打开的时间线中，但不进入历史、后台缓存或未读

同一个 `message_id` 的 durable snapshot 必须更新同一条消息投影，不能按 snapshot 数量追加多条气泡。`round_status`、`agent_round_status`、input queue 和 handoff 事件只更新状态投影，不直接变成正文消息。

Goal 完成收据由宿主在成功的 `update_goal(complete)` 后生成，并以内部 `goal_id + round_id` 精确绑定到该轮最终 assistant 的同一 `message_id` durable snapshot；`goal_id` 优先取成功工具结果返回的权威 identity，旧 Provider 未返回 identity 时才回退到该物理 round 的固定 Goal binding，两者冲突必须 fail closed。这两个绑定 ID 进入历史但不进入用户文案。收据始终可显示“Goal 已完成”，只在 Goal 聚合报告存在正耗时时附加耗时，只在 `usage_finalized=true` 时附加 actual token。结算仍在进行、Provider usage 永久不可得或查询失败时，未知 token 项必须完全省略，不得显示“结算中”“不可用”，也不得把未知值当成 0。后续结算成功或兼容修复推进了同一 Goal 聚合真相时，历史读取必须按隐藏的 `goal_id` 用当前 finalized report 静默刷新收据，而不是永久保留旧 snapshot 的错误数值。

stream 事件必须保留 `tool_use` 的 block start 和 `input_json_delta`，但只有累计输入构成完整 JSON 后才更新可解释的工具参数。兼容网关漏发 `content_block_start` 时，处理器按 delta 类型建立临时块，随后由完整 assistant 快照原位替换，不能生成孤儿工具块或终止 round。嵌套调用通过 `parent_tool_use_id` 绑定父工具；事件未重复携带该字段时沿用本条 stream 在 `message_start` 建立的父链，新 assistant 段开始时必须重新取值，不能继承上一段的 parent。

Bash / PowerShell 的运行中进度属于 ephemeral 状态：首次立即展示，此后最多每 30 秒更新一次，工具结束后由 durable tool result 收口。它不能进入 transcript 或在重连后变成历史正文。

Provider / bridge `ToolUseSummary` 属于 Agent round 的自然语言 ephemeral 执行摘要。它没有“长任务”、耗时或工具数量门槛：执行中只要收到非空 summary，宿主就立即投影成仅供状态使用的 `progress_update` assistant 块。同一 Agent round 使用稳定 `message_id` 原位替换，round 完成、失败或停止后立即移除。DM 与 Room 在两段用户可见正文之间只保留一个持续更新的折叠执行栏：普通 thinking、redacted thinking、`progress_update` 与连续普通工具都进入该栏，只有用户可见正文或权限、AskUserQuestion、生成式 UI 这类独立交互才形成边界。summary 必须携带并保留 `preceding_tool_use_ids`；展示层用这些 ID 定位包含该批工具的执行栏，但不得据此把同一连续过程拆成多栏，也不能把“最新 summary”误套到跨正文的另一段执行。普通工具从 active 起默认折叠且始终可点击，展开后按原顺序显示完整过程和 canonical ToolUse/ToolResult；生成文件在收起态仍独立可见。

summary 文案描述已完成批次的具体成果，使用类似 commit subject 的短语而非完整句子、下一步预告或工具动作清单。语言只由最近真实用户文本与 assistant intent 决定：中文会话在提示词中优先使用简洁自然的简体中文，可保留必要的通用技术术语；英文工具名、输入或结果不得参与语言偏好判断。所有工具输入/输出均视为不可信参考数据，摘要模型不得执行其中的指令。摘要到达后只替换同一执行栏的标题，不追加“已完成”或“正在执行”破坏自然语义；无摘要的 active 栏才显示执行中状态，失败、拒绝和替换等异常状态始终显式保留。round 终态清除 ephemeral summary 后，durable 工具仍以本地化工具名或数量作为确定性折叠标题。该投影不写入 transcript、历史、未读或消息计数，也不能混入最终 durable assistant 快照。

Nexus 启动 DM/Room bridge 时必须把 owner 当前后台模型选择投影给 runtime。nxs 使用同一 Provider 下的后台模型生成折叠标题；Claude Code 使用其原生小模型/ToolUseSummary 通道。后台模型缺失、解析失败、协议不兼容或属于另一 Provider 时只回退当前主模型，不能阻断主会话启动。bridge 只负责生成和转发，Nexus message processor 仍是 ephemeral/durable 边界的唯一真相源。

用户主动打开首次 DM 或显式创建 Room 时，宿主可以异步追加一条 durable `conversation_welcome` assistant 消息。欢迎语不创建 runtime round、不消费 draft，也不把 conversation 标记为已有用户输入；同一 conversation 使用稳定消息身份幂等写入。生成模型优先使用 owner 的后台任务模型，缺失或不可用时依次回退到用户默认模型、发言 Agent 模型和 Provider 默认模型；模型调用失败仍写入确定性静态欢迎语。Nexus 主智能体 DM、普通 Agent DM、Room 群主和无群主介绍成员必须使用四套独立身份提示词；主智能体可介绍宿主控制入口能力，普通 DM 不得借用该权限语义。实际生成模型继续作为内部消息事实持久化，但聊天界面不得在该欢迎语的消息头或页脚展示模型名，以免把后台生成模型误示为 Agent 的会话模型。内部自动化投递、启动恢复和其他仅为路由而发生的 DM 物化不得触发欢迎语。

Nexus host 指令的完成确认属于 transient 状态：它不是 runtime 回复，也不应写入
transcript，但必须在当前时间线中保留，让用户确认本地操作已经生效。对应
`chat_ack` 使用 `user_message_delivery_mode=transient` 把 optimistic 用户指令
规范化为同一 round 的 transient 用户消息；`user_message_committed` 仍为 false，
避免把“当前时间线可见”误表述成 runtime 历史已提交。

round 结束只由 terminal `round_status` 定义，前端不再自己猜测。

### 3.3 内容块兼容

- 已知内容块按协议类型显式解码，不靠全局字段改名。
- Claude Code 的 `server_tool_use` / `web_search_tool_result` 等块保留原始 `source_type`，同时投影到 Nexus 现有工具渲染模型。
- 新版本 runtime 发来未知或字段不完整的内容块时，前端保留原始类型和 payload，并安全隐藏；单个未知块不能让整条消息解析失败或让会话停止。
- 空数组、单个空白 text、`(no content)` 和工具中断占位文本不构成可见 assistant；多块或非文本块仍是有效消息。

## 4. 当前历史真相源

### 4.1 DM / 私有 session

当前真相源是：

- runtime transcript
- `overlay.jsonl`

其中：

- transcript 保存 agent 私有正文历史
- overlay 只保存 Nexus 自己补的语义；允许用同 `message_id` 保存不改写正文的 assistant 补充快照
- Goal 完成收据作为 Nexus 补充语义写入同 `message_id` 的 overlay assistant 快照，并在 compact 后合并回 transcript assistant
- transcript 与 overlay 的职责必须严格分开，禁止混用

### 4.2 overlay 里保存什么

DM / 私有 session 主要保存：

- `round_marker`
- `result`
- transcript 本身没有的补充消息

硬规则：

- assistant 正文只能来自 transcript；overlay assistant 只能携带同 `message_id` 的 Nexus 补充字段，并在 compact 时合并回原正文
- `result` 只能来自 overlay
- transcript 里的 `MessageTypeResult` 不参与历史投影

### 4.3 runtime transcript 的终态规则

对 transcript assistant 来说，终态只认 `message.stop_reason`：

- `message.stop_reason` 有值
  - 这条 assistant 快照就是终态 assistant
  - 不要求再存在独立 `result` 消息
- `message.stop_reason` 为空
  - 这条 assistant 仍然视为未完成快照

也就是说：

- `result` 不是 assistant 完成的必要条件
- 历史读取不能因为“没有 result”就把 transcript assistant 直接判成 interrupted
- synthetic interrupted 只允许出现在真正缺少终态且 round 已结束的场景

兼容性说明：

- assistant 的 `is_complete` 字段在持久化层继续维护，以兼容旧 transcript / 历史回放数据
- 终态判定入口只看 `stop_reason`

补充约束：

- assistant 的 `usage` 允许直接来自 transcript
- `duration_ms / duration_api_ms / num_turns / total_cost_usd / result / subtype / is_error` 只允许来自 overlay result
- `model_usage / structured_output / fast_mode_state / runtime_subtype` 也只从 overlay result 投影到 `assistant.result_summary`
- 不允许从 transcript assistant 反推一个“差不多的 result”

### 4.4 Room shared 历史

Room shared 不再保存完整正文副本，而是：

- inline overlay
- transcript_ref

也就是：

- 共享层只保存用户消息、result/synthetic 消息和对 transcript assistant 的引用
- 真正正文按需从成员 transcript 投影恢复
- `transcript_ref` 只允许引用 assistant，不允许引用 result

## 5. 分页机制

当前历史分页已经统一按 round，不按消息条数。runtime transcript、DM overlay 与
Room ledger/private transcript 继续是唯一 canonical 真相源；分页索引只是可删除、
可校验、可从这些真相源重建的派生数据，不得反向改写历史或成为第二套权威存储。

### 5.1 首屏

- 默认加载最近一页 round

### 5.2 向上翻页

- 上滚到顶部时再请求更早 round
- 保持视口位置不跳

### 5.3 重同步

- 只刷新最近一页
- 不再整段全量重拉

### 5.4 派生索引与故障恢复

- DM 与 Room 的 runtime transcript、overlay 和 Room ledger/private transcript 始终是唯一
  canonical 真相。旧用户数据不改写、不搬迁；首次读取只从 canonical 生成宿主
  `app/cache/history-read-model.v1.sqlite` 派生读模型。
- 派生库只保留当前 schema，不保留数据迁移链。schema 变化、初始化中断或损坏时
  直接丢弃并从 canonical 回建；不得反向改写 canonical 历史。
- DM 与 Room 先完成第 6 节定义的完整规范化，再在单个 SQLite 事务中发布新
  generation。每个 physical round 分开保存 B-Tree 游标元数据、完整 payload 与摘要；
  不调用模型压缩、不丢弃消息块。
- 热读先验证全部 canonical source 快照，再通过 B-Tree 读取有界的游标元数据窗口和
  本页命中的 round payload。单组摘要、scope 元数据或数据库损坏时一律放弃
  派生结果并安全回建，不得返回未经校验的历史。
- Session Round Navigator 的标题、状态、Agent 和时间元数据与消息页属于同一
  generation；热开不得为导航再扫描完整 overlay，冷开必须复用同一 rebuild future
  和 `indexing` 短轮询协议。
- 冷建或 source 变化时，同一个会话只允许一个后台 rebuild；全局只允许固定数量的
  active rebuild，不保留 detached 等待队列。HTTP 客户端设置 `defer_index=true` 时，
  后台槽已满或前景等待超过短预算就返回 `indexing=true` 和 `retry_after_ms`；
  前端保持 loading 并用短请求重试，禁止把该空 `items` 提交为真实空历史。原请求
  超时或切页不取消已受理的有界 rebuild，重试必须 attach 同 scope future，不得从零重扫。
- generation 在单个数据库事务内切换。会话在 build/persist 期间被删除时必须放弃写入，
  禁止由派生读模型重新创建 canonical session/Room 容器。超过保留期的 scope 可直接淘汰。
- 派生层必须限制 group 数、单 group、generation 和单页 payload 字节。detached build
  还必须在读取/规范化前限制 canonical source 总字节：DM 先检查全部
  overlay/transcript 快照；Room 先检查 ledger，再从有界 ledger 收集 dependency 快照，
  并在 resolve private transcript 前再次检查。source 超限的 detached build 只能生成
  disabled marker；ledger 自身已超限的 Room marker 只绑定 ledger snapshot，不能为了
  计算全依赖 digest 反过来读取 oversized ledger。完整规范化必须回到可由 HTTP
  context 取消的 request-bound 路径。超限不能改写 canonical 或向客户端返回截断历史，
  而要持久化绑定当前 source digest
  的 disabled marker，并走 request-bound canonical 精确分页。同 source 不得反复启动
  detached rebuild；source 变化后才允许在有空闲 admission 时尝试恢复索引。该降级
  保留完整功能，但会回到完整规范化的原有成本；它不伪装成 append-only 增量方案。
- 当前任一 canonical source 变化都会触发一次完整 generation 重建。duplicate UUID、
  parent 主链、marker 对齐和 Room transcript_ref 都可能反向改变旧 round，因此在没有
  等价性证明前不得用 append-only 增量更新替代完整规范化。
- Launcher 与侧栏目录只读取 Session metadata/Room catalog，禁止为了标题、预览或排序
  扫描 transcript/history；单个超长或损坏会话不能阻塞整个目录首屏。

### 5.5 客户端窗口与大型内容

- 首屏、向前分页、around 定位和 `indexing` 重试都必须携带请求级取消信号；切换或
  清空 Session 时取消旧请求。会话代次仍作为拒绝迟到响应的第二道栅栏，不能替代
  transport cancellation。
- 浏览器只保留受 root round 数与估算驻留字节双预算约束的消息窗口。淘汰以完整
  root round 为单位；around 请求必须保留目标锚点，实时、乐观与最新 round 优先于
  可再次分页取得的历史。单个不可拆 round 可以独自超过字节预算，但不得因此继续
  保留更早的大 round。
- canonical 历史仍保存完整 Tool result 与内联图片。派生 SQLite generation 对单个
  `>=256 KiB` 的 Tool result 或图片保存内容摘要、大小和 opaque detail ref，消息页只
  返回有界预览/引用；Tool 展开或图片实际渲染时才读取完整 detail。detail 必须同时
  绑定 owner、Session scope、source generation 和 payload digest；source 变化、scope
  不匹配、派生数据损坏或淘汰后一律返回 unavailable，不能回退猜测旧内容。
- 图片 detail 只允许以受限 raster MIME 和 `nosniff` 响应；桌面客户端必须通过带
  会话令牌的 fetch 生成临时 Blob URL，不能把认证 URL 直接交给 `<img>`。
- 旧 `ConversationTurn`、`TurnPage` 与 turn-index HTTP 投影已经删除。Feed 和定位只
  使用消息 round 页与同 generation 的 `SessionRoundIndex`，禁止恢复先全量读取再切片
  的第二套历史 API。

## 6. 规范化规则

历史读取时会统一做：

1. transcript / overlay 合并
2. transcript user 与 round marker 尾部对齐
3. snapshot 压缩
4. 未完成 round 物化
5. round 归一化
6. round 分页

这意味着：

- API 返回的是“可展示历史”
- 不是原始文件逐行回放
- runtime 的 `is_meta` user、Skill 完整正文、连续的 Execution / Goal
  `<internal_context>` carrier 和其他内部 carrier 必须在可见 round 投影前过滤；
  旧版 `<internal_context source="explicit_skill">` 包装只用于兼容读取，不能重新显示或进入模型历史
- marker 对齐按 transcript user 槽位逐个消费；空槽位不能跳过后借用下一轮 marker，
  否则刷新后旧的 unknown/内部消息会窃取新 Slash 的 round 身份
- runtime command metadata 统一还原为原始 `/name args`；它与 overlay marker 相同
  时只展示一份用户输入

同一 round 的稳定顺序必须是：

1. user
2. assistant / system / task_progress

说明：

- `result` 在文件侧仍然存在于 overlay
- 但对外投影时，优先挂到 assistant 的 `result_summary`
- 只有内部存储层保留 `result` 语义，不再把它当成前端可见主消息类型
- 未完成 round 的物化产物直接是 `assistant + stop_reason: cancelled + result_summary.subtype: interrupted`
- 不再经过 `role: result` 的中间态

### 6.1 公区时间线顺序

时间线同时满足“事实顺序”和“因果顺序”：

1. 同一 root round 的 primary user message 在最前，只展示一份；被定向到某个执行槽的 guide user message 属于该槽的附着输入，按 6.3 的规则紧贴目标卡片，不在顶部重复一份。
2. 已发布的 Agent final reply 按服务端公区发布时间升序展示；同一时间使用 root round 内由后端在 slot 创建时分配的稳定 `display_order`，再以 `agent_round_id` 兜底。不得按客户端收到事件的先后重排历史。
3. source message 必须先于它触发的 handoff 状态和 target reply；handoff child 可以在 sibling slot 仍运行时出现，但不能插入 source 之前。
4. 仍处于 pending、streaming、等待权限或等待 guide ACK 的 slot 不是公区事实，统一放在已完成回复之后，按 slot 启动顺序排列。
5. 同一 slot 的流式更新只更新该 slot 的状态卡；slot 进入终态后再替换为最终回复，不重复追加一张卡。

回复顺序不由 Agent 名称、`@` 书写顺序或 Skill 自己的预期决定。并行 slot 谁先完成谁先进入已发布回复区；慢 slot 只保留一个紧凑的活动状态。

示例：A 先启动但较慢，B 后启动且先完成：

```text
用户消息
├─ B 的最终回复
└─ A：执行中（紧凑状态）

A 完成后：
用户消息
├─ B 的最终回复
└─ A 的最终回复
```

活动卡从“活动区”进入“已完成区”是唯一允许的结构变化；已经发布的回复之间不因后续 stream 或 guide 而互换位置。

### 6.2 实时与历史的一致性

- 实时订阅使用 `room_seq` 做事件重放和缺口检测；它是传输序号，不是历史排序真相源。
- 历史使用持久化的公区发布时间、稳定 display order 和因果关联归一化；公区发布时间由服务端在消息进入 shared overlay 时确定，不使用 runtime 开始时间，也不能直接重放 WebSocket 到达顺序。
- 如果多个并发消息落在同一时间粒度，持久化层必须提供稳定 tie-breaker；恢复后不能因为进程重启改变已有回复的相对顺序。
- 目标 Agent 的状态事件可以先于它的 final reply 展示，但不能先于 source public message。

### 6.3 guide、queue 与 handoff 的展示

- `delivery_policy=guide` 是投递策略，不是新的 assistant 消息。
- 单目标 guide：用户消息在时间线上只保留一份，以紧凑的“补充要求”样式紧贴目标 Agent 卡片之前；不在全局位置和目标卡片各渲染一份正文。
- 多目标或无法安全归组的 guide：用户消息保留在原始公区位置，旁边只显示目标 Agent 头像/名称摘要，不为每个目标复制正文。
- guide 的 ACK、fallback、`guided_input` 等控制事件合并为目标卡片的一行轻量状态；详细过程放入 Thread，不生成独立大气泡。
- 尚未消费的用户 queue item 只出现在 composer 的待发送队列；消费后才进入时间线，且只进入一次。
- 用户入队请求只有在收到 `input_queue_ack` 后才能清空 composer；队列项即使在 ACK 前后被立即派发，重试也必须由持久化幂等记录返回原 `item_id`，不得创建第二轮。
- Agent public handoff 的 `detected/queued/running` 不生成“系统发言”气泡。源消息中的 `@Agent` chip 是唯一的交接正文，目标卡片只显示排队/运行状态；目标 final reply 到达后才显示完整回复，并可用宿主 `handoff_reply` 注解在消息头展示非动作的“回应 `@成员`”因果。这里的 `@` 只是宿主投影的成员来源标识，不得进入正文、`agent_mentions`、mention URI、handoff detector 或 wake。
- no-reply、空 assistant、纯 result 和重复 wake 不占用独立时间线行。

### 6.4 消息渲染的密度边界

- 一份正文只渲染一次；状态、路由和耗时附着在消息头、轻量状态行或 Thread 中。
- 用户消息和 Agent final reply 是主内容；thinking、tool、permission、guide 过程默认折叠或摘要化。
- 连续的状态事件合并为最新状态，不逐条堆叠“已发送/已排队/已启动/等待中”。需要审计时在 Thread 查看完整事件。
- Agent 头像只出现在消息头、Agent mention chip 和必要的状态卡，不为每条控制事件重复放大头像。
- `@Agent` 渲染为小头像 + 可点击名称；点击打开 Agent 资料，不触发第二次 handoff。头像 URL 不写入消息，历史按当前成员目录解析。
- 主 Feed 显示事实和一行状态摘要，Thread 显示过程细节；两者使用同一消息注解和同一 Agent 身份映射。

## 7. API 约束

Room / DM 历史读取统一走 room conversation 语义：

```text
GET /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/messages
```

旧的 `/nexus/v1/sessions/{session_key}/messages` 已移除。

## 8. 已删除的旧链路

以下链路已经不再是运行时主链：

- 私有 `messages.jsonl` 完整正文副本
- room shared 完整正文副本
- `cost/summary` 旧 HTTP 链
- `telemetry_cost.jsonl` / `telemetry_cost_summary.json`

## 9. 当前前端展示规则

- 历史时间线按 round 组织；同一 root round 内按本节定义的事实/因果顺序展示。
- 同一 Agent 的 active slot 使用 `agent_round_id` 归组；慢 Agent 不阻塞已完成回复，也不制造空白占位消息。
- guide 只保留一份用户正文，单目标时紧贴目标卡片，多目标时保留在全局位置并显示目标摘要。
- 中间过程默认折叠；工具、thinking、AskUserQuestion 和 guide 控制事件属于过程层，不与 final reply 平铺竞争。
- 用户消息和 final reply 都走 Markdown 渲染链；`agent_mentions` 由共享渲染器转成可点击 Agent chip。
- 主 Feed 不显示独立 wake/queue 系统气泡；状态使用轻量行或 badge，详细事件进入 Thread。

## 10. 一句话总结

当前消息系统是：

- 实时态：WebSocket 增量
- 历史态：transcript / overlay 归一化结果
- 分页单位：round
- 对外终态：统一为 `assistant + result_summary`
- 展示顺序：user → 已发布 final reply → 活动 slot 状态
- 控制消息：正文只出现一次，状态合并到目标卡片或 Thread
