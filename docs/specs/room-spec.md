# Room 规范

## 1. 文档目标

本文档定义 Room 体系的当前边界：

- Room / Conversation / Session 各是什么
- 共享历史和私有 runtime 如何分层
- Room 页面到底消费哪一层数据

## 2. 核心对象

### 2.1 room

- 协作容器
- 挂成员、对话和整体展示信息

### 2.2 member

- Room 成员关系
- 挂在 Room 上，不挂在 Conversation 上

### 2.3 conversation

- Room 内的一条共享对话
- 前端主路由以 `conversation_id` 为准

### 2.4 room session

- 某个 `conversation + agent` 的运行时记录
- 主要保存：
  - `sdk_session_id`
  - 运行状态
  - 最近活动时间

### 2.5 shared conversation

- 前端主聊天面板消费的共享消息流
- 对应共享 `room:group:<conversation_id>` 语义

### 2.6 private runtime session

- 某个 agent 在该 conversation 内的私有 runtime
- 对应 `agent:<agent_id>:ws:group:<conversation_id>`

## 3. 真相源分层

### 3.1 结构关系

SQL 是真相源：

- rooms
- members
- conversations
- sessions

### 3.2 共享历史

Room shared 历史当前是：

- inline overlay
- transcript_ref

不再保存第二份完整正文副本。

硬规则：

- `assistant` 共享正文来自成员 transcript
- `result` 共享终态来自 shared inline overlay
- `transcript_ref` 只允许引用 assistant

### 3.3 私有历史

成员私有历史来自：

- `cc transcript`
- 私有 session overlay

补充约束：

- transcript assistant 是否完成，只认 `message.stop_reason`
- room / dm 页面看到的 assistant 终态，不依赖独立 `result` 消息是否存在
- `round_marker` 只负责把 transcript user 绑定回 Nexus round 语义，不负责定义 assistant 终态
- 私有 `result` 不读 transcript，只读 overlay

## 4. Room 与 session 的关系

### 4.1 共享面板

- 前端看的是共享 conversation
- 路由主键是 `room_id + conversation_id`

### 4.2 成员运行时

- 每个成员拥有自己的 runtime session
- 只用于执行、恢复、权限绑定和 transcript 真相源

### 4.3 不允许混用

- `conversation_id` 不是私有 runtime key
- `sdk_session_id` 不是前端路由键
- SQL `sessions.id` 不是共享会话协议

## 5. 当前消息链路

### 5.1 用户发消息

1. 前端向共享会话发送 chat
2. 后端创建主 round
3. mention / 调度逻辑唤起相关成员 runtime

### 5.2 成员执行

每个被调度的成员：

- 在自己的 transcript 中产生私有历史
- 共享层对 assistant 写入 transcript_ref
- 共享层对 result / synthetic 写入 inline overlay

### 5.3 前端展示

Room 页面读取共享历史，再按 round 归一化展示。

## 6. 当前上下文接口

Room 页面主要依赖：

- room context 聚合
- conversation messages 分页

room contexts 现在需要能直接给出足够的 member summary，避免页面再额外拉全量 agent 列表。

## 7. 分页规则

Room 历史现在按 round 分页，不再按 message 行分页。

规则：

- 首屏最近一页
- 上滚加载更早 round
- Room 多 agent 子轮次会折回主 round
- 同一 round 对外稳定顺序是 `user -> assistant`
- assistant 的终态摘要通过 `result_summary` 挂载

## 8. 当前实现约束

- Room shared 已不再读旧 `messages.jsonl`
- 成员私有完整副本也已移除
- 旧历史只允许通过迁移命令转换，不再参与运行时

## 9. 禁止项

- 用共享历史替代私有 runtime transcript
- 用私有 runtime transcript 直接替代 Room shared 视图
- 从 `session_key` 回推 Room 路由

## 10. 一句话总结

Room 是共享协作层，session 是成员运行时层；两者必须协同，但不能混成一层。
