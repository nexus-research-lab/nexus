# Session Key 统一规范

定义 `session_key` 的格式、各类 key 的语义，以及哪些字段可作路由主键。以 Go 后端实现为准。

## 1. 相关标识

| 标识 | 含义 | 边界 |
| --- | --- | --- |
| `session_key` | Gateway、WebSocket、权限运行时、runtime 复用的统一会话键 | 必须可解析，业务层不得手拼 |
| `conversation_id` | Room 页面和 HTTP Room API 的主路由键，定位一条共享对话 | 不是 SDK resume id，也不是 `session_key` |
| `sdk_session_id` | runtime（nxs 与 Claude Code 均经 bridge）返回的 transcript / resume 标识，用于恢复单个 agent 私有运行时 | 不承担 UI 路由语义 |
| `room_session_id` | SQL `sessions` 记录主键 | 仅限数据库内部，不对外暴露为会话协议 |

## 2. 协议族

只有两族。

### 2.1 Agent 私有会话

```text
agent:<agent_id>:<channel>:<chat_type>[:acct:<account_id>]:<ref>[:topic:<thread_id>]
```

- 用途：普通 DM；Room 内某个 agent 的私有 runtime；外部通道映射到某个 agent 的私有会话。
- 可绑定一个 `sdk_session_id`。
- 历史真相源是 runtime transcript + Nexus overlay：`assistant` 来自 transcript，`result` 来自 overlay。

### 2.2 Room 共享会话

```text
room:group:<conversation_id>
```

- 用途：Room / DM 页面主聊天面板的共享消息流。
- 不直接绑定任何单个 agent 的 `sdk_session_id`。
- 历史真相源是 Room shared overlay：只直接保存 user/result/synthetic，assistant 通过 `transcript_ref` 回指成员 transcript。
- `group` 是冻结协议段，表示“共享流”，不表示多人群聊。
- `group` 不是 DM 的执行类型，不能据此选择或复用任何 SDK resume。DM 页面同时订阅共享流时，仍由 `agent:<agent_id>:ws:dm:<conversation_id>` 的 DM runtime 独占 resume。

## 3. Agent Key 字段

- `agent_id`：目标 agent 的稳定业务标识，不能复用为展示名。
- `channel` 保留值：`ws`、`dg`、`tg`、`dt`、`wx`、`weixin-personal`、`fs`、`internal`。
- `chat_type` 保留值：`dm`（agent 直接会话）、`group`（agent 运行在某个 room conversation 语境中）。
- `account_id`：外部通道配置多个账号时，用 `acct:<account_id>` 把同一联系人或群聊隔离到具体连接账号。位于 `chat_type` 与 `ref` 之间；无多账号歧义时不添加。
- `thread_id`：需要区分通道内 thread/topic 时，在末尾追加 `topic:<thread_id>`。`ref` 可含冒号，但不能跨过保留的 `:topic:` 边界。

`ref` 是该通道内的唯一定位：

| channel + chat_type | `ref` |
| --- | --- |
| `ws + dm` | 浏览器会话 uuid |
| `ws + group` | `conversation_id` |
| `dg + dm` | discord user id |
| `dg + group` | `guild_id:channel_id` |
| `tg + dm` | telegram user id |
| `tg + group` | telegram chat id |
| `dt + dm` | 钉钉 conversation_id 或 sender id |
| `dt + group` | 钉钉 openConversationId / conversationId |
| `wx + dm` | 企业微信 user id |
| `weixin-personal + dm` | 个人微信 iLink `from_user_id`；`context_token` 只进入 remembered delivery target，不参与 session_key 主键 |
| `fs + dm` | 飞书 open_id / user_id / union_id |
| `fs + group` | 飞书 chat_id |
| `internal + dm` | 内部保留值 |

## 4. 路由主键与禁止项

- Room 页面主路由：`room_id + conversation_id`。
- Agent runtime：`session_key`。
- 冷恢复：`sdk_session_id`。

禁止：

- 用 `conversation_id` 充当 agent 私有 `session_key`。
- 用 `sdk_session_id` 替代 `session_key`。
- 用 `room_session_id` 充当前端路由键。
- 从 `session_key` 反推数据库主键。

## 5. Builder / Parser

前后端都不得手拼 `session_key`，统一使用协议 builder / parser：

- 普通 Agent key：`BuildAgentSessionKey`。
- 多账号外部通道：`BuildAgentAccountSessionKey`。
- Room 共享流 / 成员运行时：`BuildRoomSharedSessionKey` / `BuildRoomAgentSessionKey`。
- 解析与校验：`ParseSessionKey`、`RequireStructuredSessionKey`。

## 6. 实现约束

- 浏览器入口必须显式传结构化 `session_key`。
- Room 历史接口：`/nexus/v1/rooms/{room_id}/conversations/{conversation_id}/messages`。
- 不提供 `/nexus/v1/sessions/{session_key}/messages` HTTP 读取链。
