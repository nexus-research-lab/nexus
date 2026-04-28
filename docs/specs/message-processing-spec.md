# 消息处理规范

## 1. 文档目标

本文档定义当前消息链路的三件事：

- 实时消息怎么流动
- 历史消息怎么落盘和读取
- 前端为什么按 round 展示和分页

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
- `chat_ack` 上限 10 秒（常量 `protocol.ChatAckTimeoutMS`），超时视为发送失败

### 3.2 前端展示

前端只做两类处理：

- stream：增量展示过程
- durable message：写入最终消息列表

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

- 历史时间线按 round 组织
- 中间过程默认折叠
- 工具 / thinking / AskUserQuestion 都是 block
- 用户消息和结果消息都走 Markdown 渲染链

## 10. 一句话总结

当前消息系统是：

- 实时态：WebSocket 增量
- 历史态：transcript / overlay 归一化结果
- 分页单位：round
- 对外终态：统一为 `assistant + result_summary`
