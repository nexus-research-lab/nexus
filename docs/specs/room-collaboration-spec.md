# Room 协作协议

## 1. 定位

本文是 Group Room 的当前通信协议。它只定义“谁能看到什么、谁何时运行、回复投到哪里”，不定义任何具体业务流程。

Room 模块的对象边界见 [Room 模块规范](./room-spec.md)。本文不重复 Session Key、历史归一化和前端时间线规范；时间线的展示顺序与消息密度见 [消息处理规范](./message-processing-spec.md)。

## 2. 负责与不负责

平台负责：

- 公区消息的可见性和成员唤醒。
- directed message 的私域投递、延迟唤醒和回复路由。
- Room 成员上下文的增量构造、cursor 和 checkpoint。
- round/slot 的串行交接、持久化队列、恢复和运行时护栏。
- 可供 UI 和诊断使用的通信事件。

平台不负责：

- 业务 Skill 的阶段、顺序、投票、胜负、任务分配或完成判定。
- 自动决定谁汇总、何时交还主持人或何时结束讨论。
- 把普通通信动作提升为额外业务原语或状态机。
- 将私域正文自动公开。

## 3. 核心概念

### 3.1 Public feed

Public feed 是 Group Room 的公共事实层，只包含：

- 用户公开消息。
- 已完成或已收口的 Agent final reply（错误/中断也以终态投影表示）。
- publish_public_message 明确发布的公区事实。

stream、thinking、tool_use、未完成/取消/失败的中间输出、独立 runtime result 和 no-reply 标记都不是公区事实。实时状态事件可以广播给订阅者，但不作为 Agent 的公区历史上下文。

### 3.2 Private context

Private context 是某个 Agent 可见的定向上下文，来源包括：

- 发给它的 directed message。
- 多人 recipients 消息中它可见的部分。
- directed reply route 投影给它的结果。
- 它自己的私域记录。

Private context 不进入 public feed；checkpoint 只控制消费边界，不作为正文注入；其余权限边界不由本协议定义。

### 3.3 Wake

Wake 是让目标 Agent 获得一次运行机会的调度动作。Wake 不是业务消息，也不改变已经写入的 public/private 事实。

### 3.4 Reply route

Reply route 规定被唤醒 Agent 的单次 final reply 如何投影：

- public：写入 public feed。
- private：写入指定 Agent 的 private context，可选择是否立即唤醒下一跳。
- none：不向其他成员投影。

### 3.5 Correlation ID

correlation_id 是可选的不透明关联值，只用于日志、诊断和 UI 分组。它不表示阶段、请求状态、完成状态，也不驱动唤醒。

## 4. 公区输入与 Agent handoff

### 4.1 用户输入的目标解析

用户向 Room 发送消息时，后端按以下优先级解析目标：

1. 请求中的显式 target_agent_ids。
2. 正文中可解析的 Agent @ 别名。
3. 单成员 Room 的默认成员。
4. 开启 host 默认接管时的 host_agent_id。
5. 仍无目标且存在活跃 root round 时，沿最近活跃 root round 的成员继续投递。

这些是用户输入路由规则，不是业务 Skill 状态。以上规则仍没有目标时，可以保存用户消息，但不启动 Agent。

### 4.2 Agent 的公开回复

普通公区发言直接使用当前 round 的 final reply，不调用 Room 工具。只有已收口的 final reply 才进入 public feed。

公区 final reply 中的非代码 `@成员` 默认先作为可点击的显示 mention；只有服务端选中的目标才是真实 handoff。默认只选文本顺序中的第一个有效目标，其他 `@` 保留展示但不唤醒。需要并行 fanout 时，Agent 必须在正文末尾显式附加 `<nexus_room_fanout/>`；服务端会剥离该控制标记，并为所有有效目标创建 handoff。源 Agent 的 final reply 持久化且 source slot 成功收口后立即处理，不等待同一 root round 的其他 slot；解析使用成员 name、display name 或 agent id，反引号代码区域中的 `@` 不触发唤醒。目标重复时只唤醒一次，不能唤醒自己。

用户消息里的 `@成员` 是用户显式输入目标，可按前端传入的 `target_agent_ids` 做并行 fanout；Agent final reply 则严格区分显示 mention 与 handoff intent。多个不同目标按目标拆成多个独立 handoff，目标的书写顺序只决定创建顺序，不承诺回复顺序。

公区 handoff 只传递事实和触发原因，不把源 Agent 的私域内容带给目标 Agent。目标 Agent 应输出新交付；没有新工作时使用 <nexus_room_no_reply/>，平台不写入空的公区回复。

### 4.3 @ mention 与消息注解

解析成功的 `@` 必须同时写入消息注解，供历史恢复和前端渲染使用。注解不改变正文，不新增独立的 content block：

```json
{
  "agent_mentions": [
    {
      "agent_id": "agent-devin",
      "label": "Devin",
      "content_block_index": 0,
      "start_rune": 17,
      "end_rune": 23,
      "handoff_id": "rh_..."
    }
  ]
}
```

- `start_rune/end_rune` 是半开区间，范围包含 `@`；普通字符串消息使用 `content_block_index=0`。
- `handoff_id` 只在 Agent public handoff 上存在；用户消息的目标注解可以没有。
- 没有 `handoff_id` 的 Agent mention 只是显示 span，不触发唤醒；前端仍按同一 span 渲染头像与可点击链接。
- `<nexus_room_fanout/>` 是平台控制标记，不进入正文、历史、上下文或时间线。
- 消息不持久化 avatar URL；前端按当前 Room agent directory 解析头像，找不到成员时使用 `label` 和 initials 兜底。
- 解析不明确、目标已移除或位于代码/链接 destination 中的 `@` 保留为普通文本，不创建 handoff。

### 4.4 主动公区广播

publish_public_message 仅在私域或 tool-driven 流程需要额外广播一条独立公区事实时使用。工具成功后，当前 slot 的默认 final reply 被抑制，避免重复公区消息。

## 5. Directed message

Directed message 是 Room 私域通信的唯一协议原语。单人私信、多人小范围同步、要求回答和只记录都使用同一结构。

### 5.1 作用域

- 只在 room_type=room 且 private_messages_enabled=true 时提供。
- recipients 必须是当前 conversation 的 Agent 成员。
- 工具由受控 runtime 注入；Agent 不能通过普通 HTTP body 伪造 source 或 Room scope。

### 5.2 请求字段

| 字段 | 语义与约束 |
| --- | --- |
| recipients[] | 能看到该消息的 Agent，必填且非空；重复值归一化。 |
| wake_targets[] | 实际要运行的 recipients 子集；触发唤醒时省略则默认为全部 recipients。wake_policy=none 时必须为空。 |
| content | 私域正文，必填；不会自动进入 public feed。 |
| wake_policy | none、immediate、delayed；默认 none。 |
| delay_seconds | 仅 delayed 有效，必须为正数并受平台上限限制（当前上限为 24 小时）。 |
| reply_route | 被唤醒成员 final reply 的投影路线，见下节。 |
| correlation_id | 可选、不透明关联值。 |

平台绑定 room_id、conversation_id、source_agent_id、root_round_id、caused_by_round_id、hop_index 和时间戳。绑定值由后端生成或校验，Agent 不能自行覆盖。

### 5.3 Reply route 约束

允许的形状：

    public
    private(recipients[], wake_policy=none|immediate, next_reply_route?)
    none

- private 必须显式列出一个或多个 Agent recipients。
- next_reply_route 只能挂在 private + wake_policy=immediate 上。
- next_reply_route 继续遵守同一规则，并受平台嵌套深度限制。
- private + wake_policy=none 只写入私域，不创建下一轮。
- none 表示本轮可以运行，但 final reply 不投影给任何成员。

### 5.4 语义组合

| 目的 | 组合 |
| --- | --- |
| 只记录 | wake_policy=none，reply_route=none |
| 请求目标公开回答 | wake_policy=immediate 或 delayed，reply_route=public |
| 请求目标私下回复发起者 | wake_policy=immediate 或 delayed，reply_route=private([source], wake_policy=immediate) |
| 私下回复后交给主持人公开推进 | reply_route=private([host], wake_policy=immediate, next_reply_route=public) |
| 多人共享但只运行汇总者 | recipients=[...]，wake_targets=[summarizer]，路线由 Skill 指定 |

多人 recipients 不产生新的消息类型。平台只保证可见性、唤醒和投影，不保证发言顺序、汇总正确性或讨论结束。

### 5.5 私域回复投影

当 reply_route=private 时，目标 Agent 的 final reply 会物化为一条新的 directed message：

- 原始私域正文仍不会进入 public feed。
- wake_policy=immediate 会唤醒 route recipients，并携带 next_reply_route；未声明时下一跳为 none。
- wake_policy=none 只记录结果，不产生下一轮。

只有 reply_route=public，或下一跳 route 明确为 public，才允许 final reply 进入公区。

## 6. Wake 与投递

### 6.1 目标空闲或忙碌

- 目标空闲：创建新的 Agent slot。
- 目标正在运行：Agent handoff 默认不 interrupt；优先使用已有 slot 的 guide，只有 runtime 协商了可靠 applied ACK 才能使用 guide，否则直接进入持久化 queue。
- 同一 Agent 不因一次 handoff 并发创建第二个执行槽；未消费的输入留在持久化队列。
- queue 是投递事实的唯一真相源；guide 只是对 queue item 的低延迟优化。guide 返回后在 applied ACK 前不得从 queue 删除，崩溃或无 ACK 时可安全重投。
- guide 是投递策略，不是第二条业务消息；同一份正文只能在时间线出现一次。
- 用户主动输入可以选择 queue、guide 或 interrupt；这是运行时投递策略，不是业务协议。

### 6.2 时序

- 源 Agent 的 final reply 持久化后先记录 `detected` handoff；source slot 成功收口时立即激活该 slot 产生的 handoff，不等待 sibling slots。
- source slot 失败或取消时，尚未激活的 handoff 必须取消；不能用失败或中断的半成品触发下一跳。
- source public message 的实时事件必须先于由它触发的 target slot 状态事件；target 回复在展示上不能出现在 source 之前。
- directed message 的 immediate wake 进入同一套队列和 slot 生命周期。
- delayed wake 在写入持久化日志后计时；进程重启会重放未完成计划，失败时按调度策略重试。
- 唤醒链保留 root/cause/hop 关联，便于停止、去重和诊断。

### 6.3 护栏

自动唤醒必须受平台护栏限制，包括：

- root 链 hop 上限。
- 目标 Agent 的串行执行和队列容量/过期。
- 重复 wake 的去重或合并。
- 服务重启后的 pending wake 恢复。
- 用户停止 root 链时收口派生任务。
- public handoff 的持久化日志、幂等 claim 和 queue item 关联。
- root 级 visited/cycle 检测、fanout 上限和取消传播；`hop` 上限只作为最后一道保险。

护栏只保护运行时资源，不推断业务完成。

## 7. 可见上下文

每次唤醒传给 Agent 的动态上下文由四部分组成：

1. public_feed：该 Agent 公区 cursor 之后的已发布事实。
2. latest_trigger：本次为何唤醒、源消息和 reply_route。
3. room_directed_messages：该 Agent private cursor 之后可见的私域增量。
4. public_anchor：冷启动时对较早公区历史的压缩锚点。

当前 directed message 在私域增量中优先展示，但不能越过更早未消费消息推进 private cursor。任何不属于目标 Agent 的私域消息都不得进入上下文。

### 7.1 公区事实筛选

Agent 的公区上下文只接受用户消息和其他 Agent 的已完成 assistant 终态；自身正在生成的 stream、工具过程和失败中间态不算事实。已经作为 handoff source 发布的原文可以出现在 latest_trigger，但不应被目标 Agent 重复回显。

### 7.2 冷启动与预算

- runtime 真正恢复成功时，从上次 cursor/checkpoint 之后继续。
- 新建或 resume 失效时，忽略无法证明有效的旧 cursor，发送 public_anchor + recent public delta。
- Room 附加上下文预算按当前模型窗口计算：clamp(context_window / 12, 2048, 12000)；该预算不包含系统提示词、runtime transcript、工具 schema 和输出空间。
- 预算先保障当前消息与 latest_trigger，再在 public/private delta 间分配；没有可见私域增量时，未使用的 private 配额回流 public_feed，同时为冷启动保留 public_anchor 的最小空间。
- 超出预算的消息只在自身优先级内截断，并保留截断提示；未实际消费的连续前缀不能标记为已读。

### 7.3 Checkpoint

Checkpoint 记录公区和私域实际消费边界。成功完成或明确 no-reply 的 round 可以推进；失败/取消的 round 默认不推进，除非平台能证明输入已安全消费。

## 8. 持久化与事件

| 数据 | 真相源 | 用途 |
| --- | --- | --- |
| Room、成员、conversation、session | SQL | 结构关系和归属校验 |
| 公区历史 | Room overlay + transcript reference | Room 页面和公区上下文 |
| Agent 私有正文 | Agent transcript + overlay | runtime 恢复和私有上下文 |
| Directed message | conversation 级 append-only message store | 私域可见性和回复投影 |
| Directed message cursor | conversation 级 cursor store | 私域增量消费 |
| Delayed wake | append-only wake log | 重启恢复和完成确认 |
| Public handoff | conversation 级 append-only handoff ledger | Agent `@` 的检测、派发、恢复和去重 |
| Agent mention annotation | shared message / transcript reference | 历史中的目标身份和前端可点击渲染 |
| Input queue | 持久化队列 | 忙碌 Agent 的串行接力 |
| WebSocket/事件 | 运行时投影 | 实时 UI、诊断和重同步 |

事件是投影，不是消息、handoff、队列或 checkpoint 的真相源。`room_seq` 只保证实时订阅和重放顺序，不替代历史持久化顺序；`correlation_id` 只用于关联展示，不驱动业务状态。

## 9. 稳定不变量

- public 与 private 是两种可见性，不是同一消息的两个 UI 标签。
- 只有明确的 public projection 才能写入 public feed。
- 同一 `source_message_id + target_agent_id` 只允许一个 public handoff；重试必须复用该 handoff 的 claim、queue item 或 target round。
- 同一 root 的 public handoff 必须通过 visited/cycle、fanout 和取消护栏；达到 hop 上限时只作为最终兜底拒绝。
- source public message 必须先于其 handoff 的 target 状态和回复；sibling slot 的快慢不能改变这条因果关系。
- 回复路线由消息记录携带，不能从自然语言或默认约定推断。
- Room 平台不维护业务级流程状态；需要这些状态时由 Skill 自己持久化并通信。

## 10. 非目标

本文不规定：

- Room Skill 的具体业务规则。
- Goal、定时任务、subagent task 或 connector 的生命周期。
- 前端具体 CSS、组件和交互实现；时间线的顺序、guide 展示和消息密度规则见 [消息处理规范](./message-processing-spec.md) 与 [前端设计规范](./frontend-design-spec.md)。
- 多用户权限模型和跨 Room 的协作。

一句话：public feed 记录共享事实，directed message 记录定向事实，wake 决定何时运行，reply route 决定 final reply 去哪里；业务意义由 Skill 负责。
