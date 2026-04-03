# Room 统一规范

## 1. 文档目的

本文档定义 Nexus 中 `room` 体系的统一设计、对象边界、运行时分层、消息流协议、前端呈现规则，以及与 `session_key` / Agent 私有运行时之间的关系。

`room` 不是单一数据库表，也不是单一页面组件。它同时横跨以下几层：

- Room / Member / Conversation / Session 的 SQL 持久化模型
- 共享消息流的 JSONL 正文存储
- Agent 私有 Claude 运行时
- WebSocket 协议与 Room 事件广播
- Room 页面、Thread 面板与 Workspace UI

后续所有 Room 相关迭代，都必须以本文档为准推进。若局部实现与本文档冲突，应优先向本文档定义的分层和真相源规则收敛。

## 2. 几类概念先分开

### 2.1 room

- 表示一个协作容器
- 负责承载成员、对话集合和整体展示信息
- 当前 `room_type` 只有两种：
  - `dm`
  - `room`

### 2.2 member

- 表示某个 Room 的成员记录
- 当前主要承载 Agent 成员，用户成员只保留模型位置
- 成员关系属于 Room，而不是某条 Conversation

### 2.3 conversation

- 表示 Room 内的一条具体对话线程
- 所有 Conversation 都从属于某个 Room
- 当前 `conversation_type` 只有三种：
  - `dm`
  - `room_main`
  - `topic`

### 2.4 room session

- 表示某个 Conversation 下某个 Agent 的运行时记录
- 当前落在 SQL `sessions` 表中
- 它不是 gateway 暴露的统一 `session_key`
- 它的主要职责是：
  - 绑定 `conversation_id + agent_id`
  - 持有 `sdk_session_id`
  - 记录运行时状态与最近活动时间

### 2.5 shared room stream

- 表示 Room 主聊天面板消费的共享消息流
- 它使用共享 `session_key`
- 它聚合的是整个 Conversation 的公共消息历史
- 它不是单个 Agent 的私有 Claude 上下文

### 2.6 agent runtime session

- 表示某个 Agent 在 Room Conversation 中的私有 Claude 运行时
- 它使用 Agent 作用域的 `session_key`
- 它服务于：
  - Claude resume
  - workspace 隔离
  - permission route
  - 每个成员独立运行

### 2.7 thread

- `Thread` 是 Room UI 层的明细视图，不是后端独立实体
- 它按“某一轮中的某个 Agent 子回复”过滤消息
- 它不单独建表、不单独生成新的 conversation_id

## 3. 设计目标

统一规范必须长期满足以下目标：

1. Room 共享消息流与 Agent 私有运行时必须显式区分
2. Room 的结构化关系必须以 SQL 为真相源
3. Room 消息正文必须可回放、可重组、可按轮次归并
4. 多 Agent 并发回复必须能稳定映射到主时间线与 Thread
5. 删除 Room / 删除 Topic / 成员变更必须能清理运行时残留
6. 前后端都必须通过统一 builder/parser 处理 Room 相关 key，禁止手拼

## 4. 架构分层总览

Room 体系当前可以分成五层：

### 4.1 领域持久化层

- SQL 表：
  - `rooms`
  - `members`
  - `conversations`
  - `sessions`
  - `messages`
  - `rounds`
- 负责维护 Room 结构关系、运行时索引和轮次状态

### 4.2 共享消息正文层

- `RoomMessageStore` 负责把 Conversation 的共享消息正文写入 JSONL
- 路径按 `conversation_id` 组织
- 当前 Room 历史回放主要从这里读取

### 4.3 运行时编排层

- `RoomChatService` 负责：
  - 解析 `@mention`
  - 预分配占位消息
  - 并发调度多个 Agent
  - 绑定共享路由与私有 Claude runtime
- `RoomConversationOrchestrator` 负责为单个 Agent 组装共享快照 query

### 4.4 协议与路由层

- REST：Room CRUD、Conversation CRUD、Member CRUD、DM room ensure
- WebSocket：聊天、stream 状态、成员变更、Room 删除、resync
- `RoomRouteGuard` / `RoomInterruptService` 负责校验和中断收口

### 4.5 前端展示层

- Room 页面控制器负责 bootstrap、聚合上下文和路由切换
- Workspace Shell / Layout 负责 Surface 切换与右侧 inspector
- Chat Panel / Thread Panel 负责主时间线与 Agent 明细

## 5. 核心数据模型

### 5.1 Room

- 真相源：SQL `rooms`
- 关键字段：
  - `id`
  - `room_type`
  - `name`
  - `description`

语义规则：

- `room_type=dm` 表示单 Agent 的直接会话容器
- `room_type=room` 表示多人协作容器

### 5.2 Member

- 真相源：SQL `members`
- 关键字段：
  - `room_id`
  - `member_type`
  - `member_agent_id`
  - `member_user_id`

语义规则：

- 成员挂在 Room 上，不挂在 Conversation 上
- 新增 Topic 时不会复制成员，只复用同一组 Room 成员

### 5.3 Conversation

- 真相源：SQL `conversations`
- 关键字段：
  - `id`
  - `room_id`
  - `conversation_type`
  - `title`

语义规则：

- `dm`
  - 单 Agent DM Room 的主对话
- `room_main`
  - 多成员 Room 的默认主对话
- `topic`
  - Room 内新增出来的普通对话线程

### 5.4 Session

- 真相源：SQL `sessions`
- 关键字段：
  - `conversation_id`
  - `agent_id`
  - `runtime_id`
  - `sdk_session_id`
  - `status`
  - `last_activity_at`
  - `is_primary`

语义规则：

- 每个 `conversation_id + agent_id` 当前只保留一个 primary session
- `sdk_session_id` 只属于 Agent 私有运行时
- 不允许拿 `sessions.id` 替代 gateway `session_key`

### 5.5 Message / Round

- SQL `messages`
  - 是共享消息的索引层，不是唯一正文真相源
- SQL `rounds`
  - 是 Room 子轮次状态机

语义规则：

- `messages` 负责：
  - `status`
  - `sender_agent_id`
  - `conversation_id`
  - `session_id`
  - `round_id`
  - `jsonl_path`
- `rounds` 负责：
  - 记录某个 Agent 子轮次是否 `running / done / cancelled / error`
  - 让前端和中断逻辑拿到稳定终态

## 6. Room 与 Session Key 的关系

Room 与 `session_key` 的关系必须遵守 [session-key-spec.md](/Users/leemysw/Projects/nexus/docs/session-key-spec.md)；本文只定义 Room 侧的具体使用规则。

### 6.1 共享消息流 key

当前共享流 key 形状为：

```text
room:group:<conversation_id>
```

语义规则：

- 这把 key 代表某条 Conversation 的共享公共消息流
- 它用于：
  - Room 群聊主聊天面板
  - Room Thread 所依赖的共享消息集合
  - Room 历史回放
- 这里的 `group` 是当前冻结协议值，不按字面表示“必须多人群聊”

### 6.2 Agent 私有运行时 key

当前 Agent 运行时 key 形状为：

```text
agent:<agent_id>:ws:<chat_type>:<conversation_id>
```

其中：

- `room_type=room` 时，`chat_type=group`
- `room_type=dm` 时，`chat_type=dm`

它用于：

- Claude client 创建/恢复
- permission route 绑定
- cost 记录
- Agent workspace 上下文隔离

### 6.3 两把 key 的职责分离

必须明确：

- 共享流 key 负责公共历史
- Agent runtime key 负责私有执行
- 不允许把 Agent 私有 Claude 历史当成 Room 共享真相源

## 7. 真相源规则

### 7.1 结构关系真相源

以下信息一律以 SQL 为准：

- Room 是否存在
- 某 Conversation 属于哪个 Room
- 某 Agent 是否属于某个 Room
- 某 Conversation 下有哪些 Session
- 某条消息当前是否 `pending / streaming / completed / cancelled / error`
- 某个子轮次是否已结束

### 7.2 共享消息正文真相源

以下信息当前以 Room JSONL 为准：

- Room 主聊天面板展示的历史正文
- Thread 回放时的正文内容
- 共享流消息重建

### 7.3 运行时真相源

以下信息以 Agent 私有 runtime 为准：

- Claude 是否已有可 resume 的 `sdk_session_id`
- 当前 Agent 私有 workspace 能看到什么
- permission route 绑定到了哪条执行链

### 7.4 补充规则

- 一旦共享流正文与 Agent 私有 Claude 历史冲突，应以共享流为公共历史
- `sdk_session_id` 不得反向替代 Room 共享 key
- `conversation_id` 不得反向替代 Agent 运行时 key

## 8. 生命周期规范

### 8.1 创建 Room

创建 Room 的最小结果必须同时包含：

1. `rooms` 记录
2. `members` 记录
3. 一条主 Conversation
4. 每个成员对应的一条 primary Session

规则：

- 创建 Room 默认使用 `room_type=room`，即使当前只有一个 Agent 成员
- `room_type=dm` 只能通过显式的 DM 创建入口生成
- 主对话类型：
  - DM Room 用 `dm`
  - 普通 Room 用 `room_main`

### 8.2 创建 Topic Conversation

创建 Topic 的结果必须同时包含：

1. 新的 `conversations` 记录，类型为 `topic`
2. Room 内每个 Agent 的 primary Session

规则：

- Topic 复用原 Room 的成员集
- Topic 删除后，优先回退到主对话
- 主对话不允许删除

### 8.3 添加 / 移除成员

规则：

- 只允许在 `room_type=room` 中增删成员
- `room_type=dm` 不支持追加成员
- 成员追加成功后，必须为 Room 下所有 Conversation 补齐对应 Session
- 成员移除后，必须清理该成员在所有 Conversation 上的 Session 与相关运行时映射

### 8.4 删除 Room

删除 Room 时必须同时清理：

- SQL 中的 Room / Member / Conversation / Session 级联数据
- Room Conversation 对应的 JSONL 共享消息正文
- 相关 SessionManager 运行时映射
- 前端订阅端收到 `room_deleted` 事件

## 9. 消息流规范

### 9.1 入口约束

Room 聊天必须通过 WebSocket `chat_type="group"` 进入 `RoomChatService`。

入参至少应包含：

- `room_id`
- `conversation_id`
- `session_key`
- `content`
- `round_id`

### 9.2 `@mention` 解析

Room 消息不会默认广播给所有 Agent。

规则：

- 只对被 `@` 到的 Agent 发起执行
- 没有 `@` 任何成员时：
  - 仍保存用户消息到共享流
  - 不触发 Claude 执行
  - 追加一条 info result 提示用户先 `@成员名`

### 9.3 多 Agent 并发执行

当前流程：

1. 保存用户消息到共享流
2. 为每个目标 Agent 预分配 `msg_id`
3. 发送 `chat_ack`
4. 为每个 Agent 创建独立子轮次
5. 并发执行所有 Agent

补充规则：

- `msg_id` 只表示 Room 前端占位槽位和中断定位键
- `msg_id` 不是 assistant 消息 ID
- Room 的真实 assistant turn 必须继续使用 SDK 自己的 `message_id`

子轮次规则：

- 单 Agent 时，直接复用用户 `round_id`
- 多 Agent 时，子轮次为：

```text
<user_round_id>:<agent_id>
```

### 9.4 共享快照 + 私有执行

单个 Agent 调度时必须遵守以下规则：

1. Claude client 使用 Agent 私有 runtime key
2. 发送给 Claude 的 query 必须由共享快照编排器组装
3. 共享快照只读取“当前轮次开始前已完成”的共享消息
4. 每轮使用 fresh client，禁止把旧 Claude 内部历史当成第二真相源

### 9.5 持久化规则

Room 回复写入时必须区分两类数据：

- 共享消息正文
  - 落 Room JSONL
- SQL 索引与轮次终态
  - 落 `messages` / `rounds`

result 到达后还必须：

- 记录该子轮次终态
- 通过 Agent runtime key 记账 cost

## 10. WebSocket 事件契约

Room 当前核心事件包括：

- `chat_ack`
  - 服务器已为目标 Agent 分配占位槽位
- `stream_start`
  - 某个占位消息进入 streaming
- `stream_end`
  - 某个 Agent 子轮次结束
- `stream_cancelled`
  - 某个占位消息被取消
- `room_member_added`
  - Room 成员新增
- `room_member_removed`
  - Room 成员移除
- `room_deleted`
  - Room 已删除
- `room_resync_required`
  - 提示前端重拉 Room 上下文

补充规则：

- `chat_ack` 是 Room UI 能够立即渲染 pending slot 的前提
- `chat_ack` 不是 assistant 消息
- 前端不能把 `chat_ack.msg_id` 写入共享消息流
- `permission_request` 必须携带并保留事件路由元信息：
  - `agent_id`
  - `message_id`
  - `caused_by`
- Room 前端应优先用这些元信息把权限请求挂到正确的 Agent 卡片 / Thread
- `room_resync_required` 表示前端不应继续假设本地上下文绝对正确，应回源刷新

## 11. 中断与状态修复规范

Room 中断必须以“共享流路由 + 子轮次状态机”收口。

规则：

1. 中断请求必须先经过 `RoomRouteGuard`
2. 只要是 Room 共享 key，就必须校验：
   - `session_key`
   - `room_id`
   - `conversation_id`
   - `msg_id`
   - `target_agent_id`
3. 中断后仍处于 `pending / streaming` 的槽位必须统一修复为 `cancelled`
4. 对应 `rounds` 记录也必须同步标记为 `cancelled`

## 12. 前端工作区规范

### 12.1 页面骨架

当前 Room 页面主链路为：

```text
RoomPage
→ useRoomPageController
→ RoomWorkspaceShell
→ RoomWorkspaceLayout
→ Chat / History / Workspace / About
```

### 12.2 Surface 规则

当前 Surface 规则如下：

- `chat`
  - 默认聊天主区域
- `history`
  - Room Conversation 列表
- `workspace`
  - 当前 Agent workspace 文件视图
- `about`
  - 仅 `room_type=dm` 时展示 Agent 详情

### 12.3 DM Room 与普通 Room 的前端差异

- `room_type=dm`
  - 复用 Room 数据模型
  - Chat Surface 走 `DmChatPanel`
  - 右侧 inspector 以上下文面板为主
- `room_type=room`
  - Chat Surface 走 `RoomChatPanel`
  - 主消息流使用共享 Room key
  - 右侧 inspector 可在上下文面板与 Thread 面板之间切换

## 13. 时间线与 Thread 规范

### 13.1 主时间线分组

Room 主时间线必须先把多 Agent 子轮次折叠回同一条用户轮次。

规则：

- 单条用户消息为一个主 round
- 多 Agent 子回复共享同一个主 round
- 前端按 `base_round_id` 聚合，不把 `round_id:agent_id` 拆成多条时间线段

### 13.2 已完成与未完成回复的呈现

当前 UI 规则：

- 已完成的 Agent 回复
  - 直接进入主时间线
  - 按完成时间排序
- 未完成的 Agent 回复
  - 保留在底部占位卡片区域
  - 点击后进入 Thread 查看实时过程

### 13.3 Thread 的语义

Thread 只表示“某一轮中某个 Agent 的完整明细”。

展示内容只包含：

- 当前轮的用户消息
- 目标 Agent 的 assistant 消息

Thread 规则：

- 从进行中的占位卡片打开时，可设置 `auto_close_on_finish`
- 用户在过程中仍可手动关闭
- 已完成回复也可单独查看 Thread，但不会自动关闭

### 13.4 Thread 与主时间线的边界

- Thread 不是独立消息源
- Thread 只对共享流消息做过滤视图
- Thread 只消费真实 assistant 执行链，不消费占位槽位
- Agent 的流式过程先体现在 Thread
- 但权限确认不能只藏在 Thread 里
- 如果某个 Agent 正在等待权限，主时间线中的 pending card 也必须给出明确提示
- 子轮次完成后，最终回答进入主时间线

## 14. 命名与边界规则

### 14.1 必须统一的命名

- `room_id`
  - 只表示 Room 主键
- `conversation_id`
  - 只表示 Room 内某条对话
- `room_session_id`
  - 只表示 SQL `sessions.id`
- `sdk_session_id`
  - 只表示 Claude runtime resume id
- `session_key`
  - 只表示 gateway 协议键

### 14.2 禁止混用

禁止：

- 把 `room_id` 当 `conversation_id`
- 把 `conversation_id` 当共享 `session_key`
- 把 `room_session_id` 暴露给前端当路由键
- 把 `sdk_session_id` 误当共享历史 key
- 直接手拼 `room:group:` 或 `agent:...:ws:...`

Builder/Parser 统一入口：

- 后端：`agent/service/room/room_session_keys.py`
- 前端：`web/src/lib/session-key.ts`

## 15. 反模式

以下做法都属于 Room 体系反模式：

1. 把 Agent 私有 Claude 历史当成公共共享历史
2. 跳过共享快照，直接把用户原始文本发给多个 Agent
3. 只写 JSONL 不写 SQL 状态索引
4. 只写 SQL 状态索引，不写共享正文
5. 把 Thread 当成独立后端实体设计
6. 在前端继续散落手拼 Room key
7. 在删除 Topic / 删除 Room 时漏清运行时映射和共享 JSONL

## 16. 核心入口文件

为方便后续维护，Room 当前关键入口如下：

### 16.1 后端

- API：
  - `/Users/leemysw/Projects/nexus/agent/api/room/api_room.py`
- 聊天编排：
  - `/Users/leemysw/Projects/nexus/agent/service/chat/room_chat_service.py`
- Room 领域服务：
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_service.py`
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_conversation_service.py`
- 共享消息与轮次：
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_message_store.py`
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_round_store.py`
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_conversation_orchestrator.py`
- 路由与中断：
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_session_keys.py`
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_route_guard.py`
  - `/Users/leemysw/Projects/nexus/agent/service/room/room_interrupt_service.py`

### 16.2 前端

- 页面入口：
  - `/Users/leemysw/Projects/nexus/web/src/pages/room/room-page.tsx`
- 控制器：
  - `/Users/leemysw/Projects/nexus/web/src/hooks/use-room-page-controller.ts`
- 布局：
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/room-workspace-shell.tsx`
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/room-workspace-layout.tsx`
- 主聊天：
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/room-chat-panel.tsx`
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/room-conversation-feed.tsx`
- Thread：
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/thread/room-thread-context.tsx`
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/thread/room-round-card-group.tsx`
  - `/Users/leemysw/Projects/nexus/web/src/features/room-conversation/thread-detail-panel.tsx`

## 17. 最终结论

Room 的本质不是“多人版 DM”，而是：

- 以 SQL 结构关系为骨架
- 以共享消息流为公共历史
- 以 Agent 私有 runtime 为执行单元
- 以前端主时间线 + Thread 过滤视图承载多 Agent 协作结果

后续任何 Room 改造，都必须先回答四个问题：

1. 这是改共享流，还是改 Agent 私有运行时？
2. 真相源在 SQL、JSONL，还是 Claude runtime？
3. 这条数据属于 Room 主时间线，还是只属于 Thread 过滤视图？
4. 改动后 `room_id / conversation_id / room_session_id / sdk_session_id / session_key` 是否仍然各司其职？
