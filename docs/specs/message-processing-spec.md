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
- assistant 正文真相源只来自 `cc transcript`

### 2.3 result message

- 一轮执行的终态结果
- 包含结果文本、执行终态与 runtime 摘要
- result 真相源只来自 Nexus overlay
- 对外 API / WebSocket 不再直接暴露 standalone `result`
- 最终展示统一收口为 `assistant.result_summary`

### 2.4 round

- 一次用户输入触发的一轮业务对话
- 当前历史分页、状态收口都按 round 处理

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

前端只做两类处理：

- stream：增量展示过程
- durable message：写入最终消息列表

同一个 `message_id` 的 durable snapshot 必须更新同一条消息投影，不能按 snapshot 数量追加多条气泡。`round_status`、`agent_round_status`、input queue 和 handoff 事件只更新状态投影，不直接变成正文消息。

round 结束只由 terminal `round_status` 定义，前端不再自己猜测。

## 4. 当前历史真相源

### 4.1 DM / 私有 session

当前真相源是：

- `cc transcript`
- `overlay.jsonl`

其中：

- transcript 保存 agent 私有正文历史
- overlay 只保存 Nexus 自己补的语义
- transcript 与 overlay 的职责必须严格分开，禁止混用

### 4.2 overlay 里保存什么

DM / 私有 session 主要保存：

- `round_marker`
- `result`
- transcript 本身没有的补充消息

硬规则：

- `assistant` 只能来自 transcript
- `result` 只能来自 overlay
- transcript 里的 `MessageTypeResult` 不参与历史投影

### 4.3 cc transcript 的终态规则

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

当前历史分页已经统一按 round，不按消息条数。

### 5.1 首屏

- 默认加载最近一页 round

### 5.2 向上翻页

- 上滚到顶部时再请求更早 round
- 保持视口位置不跳

### 5.3 重同步

- 只刷新最近一页
- 不再整段全量重拉

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
- Agent public handoff 的 `detected/queued/running` 不生成“系统发言”气泡。源消息中的 `@Agent` chip 是唯一的交接正文，目标卡片只显示排队/运行状态；目标 final reply 到达后才显示完整回复。
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
