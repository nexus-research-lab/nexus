# Session Key 统一规范

## 1. 文档目的

本文档定义 `session_key` 的统一协议、字段语义、适用边界和变更治理规则。

`session_key` 是系统的全局 gateway 标识，不只是前端字符串，也不是单纯的 SDK 会话 ID。它承担以下职责：

- 标识一次可路由的会话入口
- 让前后端在不查库的前提下完成大部分路由判断
- 区分共享消息流与 Agent 私有运行时
- 作为 WebSocket / REST / 文件存储 / 运行时映射之间的稳定桥梁

这个协议一旦稳定，后续不要轻易变更。新增能力优先扩展解析器，不要直接改已有字符串形状。

## 2. 几类概念先分开

### 2.1 session_key

- 面向 gateway 的全局字符串标识
- 出现在前端、WebSocket、Session API、文件存储、运行时映射中
- 需要可解析、可比较、可路由

### 2.2 room_session_id

- SQL `sessions` 表的主键
- 表示某个 Room Conversation 下某个 Agent 的运行时记录
- 是数据库内部标识，不是统一路由协议

### 2.3 sdk_session_id

- Claude SDK 返回的 resume / session id
- 是 `session_key` 最终绑定出的运行时结果
- 不应反向替代 `session_key`

### 2.4 conversation_id

- 这是 Room 前端 UI 路由层的会话标识
- Room 页面、URL、列表选中只认 `conversation_id`
- 它不允许再从 `room_session_id`、`session_key` 或其他字段回退推导
- 它不等于 gateway `session_key`
- 它也不应该被拿去替代 Claude 运行时的 `sdk_session_id`

结论：

- 对外协议看 `session_key`
- 数据库存储关联看 `room_session_id`
- Claude 恢复上下文看 `sdk_session_id`
- Room 页面路由切换看 `conversation_id`

## 3. 设计目标

统一规范必须长期满足以下目标：

1. 全局唯一
2. 无需查库即可完成大部分路由
3. 前后端都能稳定解析
4. 同一语义只能有一个中心 builder，不允许手拼
5. 协议向后兼容优先，禁止随意改前缀和字段顺序
6. Room 共享流与 Agent 私有运行时必须显式区分

## 4. 协议总览

当前 `session_key` 协议分两族：

1. Agent 作用域键
2. Room 共享作用域键

### 4.1 Agent 作用域键

格式：

```text
agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]
```

用途：

- Web 普通 DM 会话
- Room 内某个 Agent 的私有 SDK 运行时
- Discord / Telegram / internal 等通道会话

特点：

- 对应一个 Agent 私有上下文
- 通常可以绑定一个 `sdk_session_id`
- 适合 Claude 运行时恢复、workspace 隔离、权限绑定

### 4.2 Room 共享作用域键

格式：

```text
room:group:<conversation_id>
```

用途：

- Room 主聊天面板的共享消息流
- Room / DM 的共享对话视图入口

特点：

- 这是共享消息流，不是单个 Agent 的私有运行时
- 它不直接等于某个 `sdk_session_id`
- 它聚合的是整个 Room Conversation 的历史消息

注意：

- 这里的 `group` 是历史冻结值
- 它现在的语义应理解为“共享会话流协议标记”
- 不应再按字面理解成“只代表多人群聊”
- 当前 DM Room 共享面板也复用这个前缀

## 5. 三个维度不要混淆

### 5.1 协议族

- `agent`
- `room`

它决定这是 Agent 私有会话还是共享消息流。

### 5.2 channel

仅对 `agent:*` 生效。

当前保留值：

- `ws`：WebSocket / Web 前端
- `dg`：Discord
- `tg`：Telegram
- `internal`：内部委派/系统内通信

### 5.3 chat_type

当前保留值：

- `dm`
- `group`

说明：

- 对 `agent:*`，`chat_type` 表示该 Agent 运行时面对的是 DM 还是群体协作上下文
- 对 `room:*`，目前第二段历史上固定写成 `group`

## 6. 字段语义定义

### 6.1 `agent_id`

- 仅 `agent:*` 存在
- 表示这把 key 最终路由到哪个 Agent 私有运行时
- 这是稳定字段，不允许复用为 display name、alias 或 title

### 6.2 `channel`

- 仅 `agent:*` 存在
- 表示入口通道
- 由通道层统一枚举，不允许业务层自造缩写

### 6.3 `chat_type`

- 对 `agent:*`：表示上下文拓扑
- 对 `room:*`：当前是历史冻结协议段，不建议作为业务判断主来源

### 6.4 `ref`

- 仅 `agent:*` 存在
- 表示“该 channel 内部如何唯一定位本会话”
- `ref` 的所有权属于 channel，而不是通用业务层

不同通道下的 `ref` 约定：

- `ws + dm`
  - 普通 DM：前端生成的 uuid
  - 自 2026-04-01 起，浏览器入口不再接受 `agent_id -> session_key` 退化
- `ws + group`
  - Room 成员私有运行时：使用 `conversation_id`
- `dg + dm`
  - `discord_user_id`
- `dg + group`
  - `<guild_id>:<channel_id>`
- `tg + dm`
  - `telegram_user_id`
- `tg + group`
  - `<chat_id>`
- `internal + dm`
  - 当前保留值 `chat`
  - 只允许内部系统使用，外部业务不要复用

### 6.5 `thread_id`

- 仅 `agent:*` 可选存在
- 由保留段 `:topic:<thread_id>` 挂接
- 用于 Discord thread / Telegram topic 等细分线程场景

## 7. 协议示例

### 7.1 Web 普通 DM

```text
agent:agent_xxx:ws:dm:8a2d5f18-2d8a-4f4f-82ad-0f6c3f8d1c8d
```

### 7.2 Web Room 成员私有运行时

```text
agent:agent_xxx:ws:group:conversation_xxx
```

### 7.3 Room / DM 共享聊天面板

```text
room:group:conversation_xxx
```

### 7.4 Discord 私聊

```text
agent:main:dg:dm:123456789
```

### 7.5 Discord 群聊 Thread

```text
agent:main:dg:group:987654321:123123123:topic:456456456
```

### 7.6 Telegram 群 Topic

```text
agent:main:tg:group:-100123456:topic:12
```

### 7.7 Internal Agent 委派

```text
agent:agent_target:internal:dm:chat
```

## 8. 场景映射矩阵

| 场景 | 给前端/网关用的 key | 给 Claude 运行时用的 key | 说明 |
| --- | --- | --- | --- |
| Web 普通 DM | `agent:*:ws:dm:*` | 同一把 key | 单 Agent 会话 |
| Room 主聊天面板 | `room:group:<conversation_id>` | 不直接用 | 共享流，只看 Conversation |
| DM Room 主聊天面板 | `room:group:<conversation_id>` | 不直接用 | 共享流，历史上仍走 `room:group` |
| Room 某成员 Agent 工作区 | 页面主入口不直接暴露 | `agent:<agent_id>:ws:group:<conversation_id>` | 私有运行时 |
| DM Room 某成员 Agent 工作区 | 页面主入口不直接暴露 | `agent:<agent_id>:ws:dm:<conversation_id>` | 私有运行时 |
| Discord / Telegram | `agent:*:<channel>:<chat_type>:<ref>` | 同一把 key | 由通道层拥有 `ref` 规则 |

## 9. 解析契约

统一解析器必须支持以下输出语义：

- `kind`
  - `agent`
  - `room`
  - `unknown`
- `is_structured`
  - 是否匹配统一协议
- `is_shared`
  - 是否为共享消息流
- `agent_id`
- `channel`
- `chat_type`
- `ref`
- `thread_id`
- `conversation_id`

解析约束：

1. `agent:*` 和 `room:*` 必须分别解析
2. 解析 `room:*` 时不能再套用 `agent:*` 的位置含义
3. 解析器必须容忍 `ref` 中包含普通冒号
4. `:topic:` 是保留分隔符，不能在未转义语义里乱用

## 10. 代码使用规范

### 10.1 必须通过统一 builder 生成

禁止：

- 业务代码手写 `f"agent:{...}"`
- 前端手拼 `"room:group:" + id`
- 用字符串切片推断语义

必须：

- 后端统一走 `agent/service/session/session_router.py`
- Room 相关统一走 `agent/service/room/room_session_keys.py`
- 前端统一走 `web/src/lib/session-key.ts`

### 10.2 共享流和私有运行时不要混用

禁止把：

- `room:group:<conversation_id>` 当成 Claude 私有运行时 key
- `agent:*` 当成 Room 共享历史聚合键

正确做法：

- 共享历史 / 主聊天面板用 `room:*`
- Agent 工作区 / Claude resume / 权限绑定用 `agent:*`

### 10.3 浏览器入口必须显式传递结构化 `session_key`

从现在开始，以下入口必须显式传递合法的结构化 `session_key`：

- Browser WebSocket `chat`
- Browser WebSocket `interrupt`
- Browser WebSocket `permission_response`
- Session REST API `/sessions/**`

网关不会再为浏览器请求执行 `agent_id -> session_key` 自动补全，也不会再接受裸字符串 session key。

历史上，部分 Web DM 入口会在缺少 `session_key` 时退化成：

```text
agent:<agent_id>:ws:dm:<agent_id>
```

这个退化规则现在只允许保留在“服务端自有通道内部构建”的语境里，例如：

- Discord / Telegram 通道适配层
- internal agent delegate
- 其他由服务端 builder 直接生成 session_key 的非浏览器入口

这只是内部构建路径，不是外部协议能力。

新代码必须显式创建真正的 `session_key`，而不是只传 `agent_id`。

### 10.4 前端命名必须区分 gateway key、Room conversation id 和状态派生名

前端代码里至少要区分以下几类字段：

- `session_key`
  - 只表示 gateway 协议键
  - 用于 WebSocket、Session API、消息归属、Claude 运行时绑定
- `conversation_id`
  - 只表示 Room 页面当前会话的路由标识
  - 用于 URL、列表选中、Room 对话切换
  - Room UI 不允许再从 `room_session_id` 或 `session_key` 回退推导它
- `current_session_key`
  - 只表示前端全局会话 store 里当前选中的 DM/session_key
  - 仅用于 home/DM 状态层，不参与 Room 路由含义
- `legacy launcher-app session_key`
  - 只表示历史遗留的 Launcher 内嵌 Nexus 会话键
  - 只允许用于前端过滤旧数据，不允许再作为任何新会话的构建结果
- `current_agent_sessions` / `current_session`
  - 只表示 home/DM controller 层基于 `current_session_key` 派生出的当前 Agent 会话列表和当前选中会话
  - 它们是 UI 派生结果，不是协议字段，也不是数据库字段
- `bind_session_key` / `start_session` / `load_session` / `clear_session` / `reset_session`
  - 只表示前端 hook 层按 `session_key` 绑定、启动、加载、清空和重置当前会话
  - 不允许继续暴露 `bind_conversation_key`、`start_conversation`、`load_conversation`、`clear_conversation`、`reset_conversation` 这类旧命名
- `SessionLoaderOptions` / `useSessionLoader`
  - 只表示按 `session_key` 监听并触发加载的 hook 约定
  - 不允许继续保留 `ConversationLoaderOptions` / `useConversationLoader` 这类旧命名
- `SessionSnapshotPayload.session_key`
  - 只表示 DM / Home 快照回写时用于定位会话 store 记录的 `session_key`
- `RoomConversationSnapshotPayload.conversation_id`
  - 只表示 Room 快照回写时用于定位页面会话列表项的 `conversation_id`
- `ConversationSnapshotPayload`
  - 这是 `SessionSnapshotPayload | RoomConversationSnapshotPayload` 的联合类型
  - 只允许作为 Room 工作区这类同时承载 DM 与 Room UI 的上层桥接参数使用

禁止：

- 把 `session_key` 参数命名成 `conversation_id`
- 把历史 `launcher-app` session_key 继续当成正式会话键使用
- 把按 `session_key` 工作的 hook 方法继续命名成 `*conversation*`
- 把 `conversation_id` 传给 Session API / WebSocket 充当 `session_key`
- 把 `current_session_key` 误当成 Room 页面里的 `conversation_id`
- 在 Room UI 里从 `room_session_id`、`session_key` 或其他字段回退生成路由用的 `conversation_id`
- 在一个变量里同时承载“路由选中值”和“gateway 协议键”

### 10.5 前端兼容迁移字段只允许存在于持久化迁移代码

历史上前端有两组已经废弃的字段名：

- `current_conversation_id`
  - 这是旧版 conversation store 的当前 session 选择字段
  - 这层兼容已经移除，不再允许出现在当前代码里
  - 遇到仍保留该字段的旧浏览器缓存时，允许直接丢弃该本地状态
- `conversation_key`
  - 这是旧版 launcher app 面板的 session 持久化字段
  - 这层兼容已经移除，不再允许出现在当前代码里
  - 遇到仍保留该字段的旧浏览器缓存时，允许直接丢弃该本地状态

不允许：

- 新的 store state、props、controller 返回值继续使用这两个名字
- 在运行时代码里继续读写这两个字段
- 把迁移兼容字段重新暴露成对外 API

## 11. 稳定性规则

下面这些内容一旦冻结，不允许直接改：

1. 一级前缀 `agent` / `room`
2. 分隔符 `:`
3. `agent:*` 的字段顺序
4. `:topic:` 保留段语义
5. `room:group:<conversation_id>` 的既有协议值

可以扩展，但不能直接改现有含义：

- 新增 `channel` 枚举
- 新增解析别名
- 新增对未来 `room:<scope>:<conversation_id>` 的只读兼容

不允许：

- 无迁移直接替换前缀
- 前后端分别维护不同格式
- 在某个业务模块里偷偷增加私有变体

## 12. 变更治理流程

如果未来必须升级协议，流程必须是：

1. 先补解析器双读兼容
2. 再补 builder 双写或新写入策略
3. 明确旧数据/旧消息日志/旧前端缓存的兼容期
4. 更新本文档
5. 给出迁移窗口和回滚策略

只有满足以上条件，才能讨论协议升级。

## 13. 反模式清单

以下做法明确禁止：

1. 用 `session_key` 反推出 `room_id`
2. 把 `session_key` 当 SQL 外键
3. 在前端把 `session_key` 和 `conversation_id` 混为一谈
4. 在后端把 `room_session_id` 误当 `session_key`
5. 手工比较 Room key 的完整字符串而不考虑共享语义
6. 让 `ref` 同时承载多个层次的业务含义

## 14. 当前代码落点

当前协议相关代码应以这些文件为准：

- 后端统一协议入口：`agent/service/session/session_router.py`
- 浏览器 WS 入参校验：`agent/service/channels/ws/dispatcher.py`
- 浏览器 Session API 校验：`agent/api/session/api_session.py`
- Room 协议辅助：`agent/service/room/room_session_keys.py`
- 前端统一协议入口：`web/src/lib/session-key.ts`
- 前端会话运行时 Hook：`web/src/hooks/agent/use-agent-conversation.ts`
- 前端会话加载 Hook：`web/src/hooks/use-session-loader.ts`
- Home / DM 会话状态收口：`web/src/hooks/use-home-agent-conversation-controller.ts`
- Home / DM 会话 store：`web/src/store/conversation/index.ts`
- 会话遗留过滤：`web/src/store/conversation/actions.ts`
- Nexus DM 入口页：`web/src/pages/nexus-session/nexus-session-page.tsx`
- Room 页面路由状态：`web/src/hooks/use-room-page-controller.ts`
- Room 主聊天面板：`web/src/features/room-conversation/room-chat-panel.tsx`
- DM 主聊天面板：`web/src/features/dm-conversation/dm-chat-panel.tsx`
- Session 聚合服务：`agent/service/session/session_service.py`

## 15. 最终结论

后续要长期坚持这条原则：

- `session_key` 是全局 gateway 协议，不是随手拼出来的业务字段
- `agent:*` 代表 Agent 私有运行时入口
- `room:*` 代表 Room 共享消息流入口
- `room_session_id`、`sdk_session_id` 都是内部实现，不得替代 `session_key`

协议稳定比“看起来更优雅”更重要。只有在完整兼容策略成立的前提下，才允许做下一次升级。
