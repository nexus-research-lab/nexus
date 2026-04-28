# Session Key 统一规范

## 1. 文档目标

本文档定义 Nexus 当前有效的 `session_key` 协议。

它只回答三件事：

- `session_key` 长什么样
- 不同类型的 key 分别服务什么语义
- 哪些字段可以当路由主键，哪些不可以

当前实现以 Go 后端为准，不再描述旧 Python 链路。

## 2. 相关概念

### 2.1 `session_key`

- Gateway、WebSocket、权限运行时、runtime 复用的统一会话键
- 必须可解析，不允许业务层手拼

### 2.2 `conversation_id`

- Room 页面和 HTTP Room API 的主要路由键
- 用于定位一条共享对话
- 不是 SDK resume id，也不是 `session_key`

### 2.3 `sdk_session_id`

- `cc` runtime 的 resume 标识
- 用于恢复单个 agent 私有运行时
- 不对外承担 UI 路由语义

### 2.4 `room_session_id`

- SQL `sessions` 记录主键
- 只属于数据库内部
- 不对外暴露为会话协议

## 3. 当前协议族

当前只保留两族：

### 3.1 Agent 私有会话

格式：

```text
agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]
```

用途：

- 普通 DM
- Room 内某个 agent 的私有 runtime
- 外部通道映射到某个 agent 的私有会话

特点：

- 可以绑定一个 `sdk_session_id`
- 历史真相源是 `cc transcript + Nexus overlay`
- 其中 `assistant` 来自 transcript，`result` 来自 overlay

### 3.2 Room 共享会话

格式：

```text
room:group:<conversation_id>
```

用途：

- Room / DM 页面主聊天面板的共享消息流

特点：

- 不直接绑定某个单独 agent 的 `sdk_session_id`
- 历史真相源是 Room shared overlay
- Room shared overlay 只直接保存 user/result/synthetic，assistant 通过 `transcript_ref` 回指成员 transcript
- 当前 `group` 是冻结协议段，表示“共享流”，不是严格的多人群聊字面义

## 4. Agent Key 字段语义

### 4.1 `agent_id`

- 指向目标 agent
- 必须是稳定业务标识
- 不能复用为展示名

### 4.2 `channel`

当前保留值：

- `ws`
- `dg`
- `tg`
- `internal`

### 4.3 `chat_type`

当前保留值：

- `dm`
- `group`

语义：

- `dm`：agent 直接会话
- `group`：agent 运行在某个 room conversation 语境中

### 4.4 `ref`

表示该通道内唯一定位方式。

约定：

- `ws + dm`：浏览器会话 uuid
- `ws + group`：`conversation_id`
- `dg + dm`：discord user id
- `dg + group`：`guild_id:channel_id`
- `tg + dm`：telegram user id
- `tg + group`：telegram chat id
- `internal + dm`：内部保留值

## 5. 真相源与边界

### 5.1 路由主键

- Room 页面主路由：`room_id + conversation_id`
- Agent runtime：`session_key`
- 冷恢复：`sdk_session_id`

### 5.2 明确禁止

- 用 `conversation_id` 充当 agent 私有 `session_key`
- 用 `sdk_session_id` 替代 `session_key`
- 用 `room_session_id` 充当前端路由键
- 从 `session_key` 反推出数据库主键

## 6. Builder / Parser 规则

`session_key` 必须统一由协议 builder / parser 处理。

约束：

- 前端不手拼
- 后端不手拼
- 新能力优先扩展 builder，不改已有字符串形状

## 7. 当前实现约束

- 浏览器入口必须显式传结构化 `session_key`
- Room 历史接口已经切到：

```text
/nexus/v1/rooms/{room_id}/conversations/{conversation_id}/messages
```

- 不再保留旧的 `/nexus/v1/sessions/{session_key}/messages` HTTP 读取链

## 8. 一句话总结

- `session_key` 负责运行时和协议路由
- `conversation_id` 负责 Room 共享会话路由
- `sdk_session_id` 负责 agent 私有 runtime 恢复
- 三者各司其职，禁止混用
