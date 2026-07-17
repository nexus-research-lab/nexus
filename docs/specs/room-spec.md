# Room 模块规范

## 1. 文档定位

本文只定义 Room 模块的领域边界和数据归属，不重复消息历史、`session_key` 或 Room Skill 的细节。

相关规范：

- [消息处理规范](./message-processing-spec.md)：实时消息、历史投影和 round 分页。
- [Session Key 规范](./session-key-spec.md)：共享会话键、Agent 私有会话键和恢复键。
- [Room 协作协议](./room-collaboration-spec.md)：公区、私域、唤醒和回复投影。
- [Room Skill 编写指南](./room-collaboration-mechanism.md)：面向 Skill 作者的最小行为规则。

本文描述 Room 的当前领域边界，以及已经确认的 P0 持久化、派发和投影约束；具体实现可以分阶段落地，但不得偏离这些边界。

## 2. 模块范围

Room 模块负责：

- 房间、成员和 conversation 的生命周期。
- Room conversation 的共享消息投影。
- 每个成员 Agent 的私有 runtime 启动、恢复、中断和清理。
- 公区输入、成员目标解析、Room round 和输入队列。
- Group Room 的 directed message、私域上下文和唤醒。
- Room 级配置：host 默认接管、Room Skill 和私域消息开关。

以下内容不属于本规范：

- Goal 的业务状态、预算和续跑规则。
- Agent runtime 内部的 provider、工具执行和 transcript 格式。
- 通用消息归一化与前端时间线渲染。
- Room Skill 的业务规则（例如投票、顺序、胜负和收口条件）。

## 3. 核心对象

### 3.1 Room

Room 是成员和 conversation 的容器。当前有两种类型：

- `room`：多人协作 Room，可配置 host、Room Skill 和 directed message。
- `dm`：单 Agent 直聊的 Room 外壳，不启用 Room Skill 或 directed message。

### 3.2 Member

Member 是 Room 的成员关系。成员类型只有：

- `user`：Room owner。
- `agent`：参与该 Room 的 Agent。

成员属于 Room，不属于某条 conversation。Agent 是否能被路由，以当前 Room 成员表为准。

### 3.3 Conversation

Conversation 是 Room 内独立的共享对话。每个 Room 至少有一个主 conversation，也可以有 `topic` conversation。

`conversation_id` 是 Room 页面和 Room HTTP API 的共享对话路由键。删除 topic 会同时关闭其运行时；主 conversation 不能删除。

### 3.4 Session

Session 是数据库中的 `conversation + agent` 运行时索引，保存 runtime 标识、版本、状态和最近活动时间。它不是前端路由键，也不是 SDK resume id。

每个 Group Room conversation 的每个 Agent 有独立的私有 runtime session。DM 只有一个 Agent session。

### 3.5 Round 与 slot

- `round`：一次共享输入或一次 Room 唤醒形成的执行批次。
- `slot`：该 round 中某个 Agent 的实际执行槽。

Room 可以在同一 root round 下运行多个 Agent slot；`agent_round_id` 标识 slot，`round_id` 对外表示根 round。

## 4. 两层运行模型

Room 必须把共享协作层和成员执行层分开：

| 层 | 负责什么 | 主要真相源 |
| --- | --- | --- |
| Shared conversation | 公区事实、用户消息、共享历史和 Room 页面 | SQL 关系 + Room overlay + transcript reference |
| Agent runtime session | 单个 Agent 的模型上下文、工具执行、恢复和私域消费位置 | runtime transcript + Agent overlay |

共享层可以引用成员 transcript 的已完成 assistant，但不拥有成员的完整私有正文。成员 runtime 也不能直接代替 Room shared 视图。

## 5. 历史与投影边界

### 5.1 Room shared 历史

Room shared 历史由两类行组成：

- inline overlay：用户消息、合成 assistant、result 摘要和其他 Nexus 语义。
- `transcript_ref`：指向成员 transcript 中已完成 assistant 的引用。

读取时解析引用并统一投影；对外展示以已收口的 assistant 为主，result 作为 `result_summary` 附着在 assistant 上。未完成的过程态不能成为公区事实，错误和中断只以终态摘要出现。
公区 assistant 的 `agent_mentions` 随 transcript reference 一起保留，不能只存在于实时事件或内存 handoff 中。

### 5.2 成员私有历史

成员 runtime 的完整上下文保留在自己的 transcript 与 overlay 中。Room 公区 cursor、directed message cursor 和 checkpoint 只表示消费边界，不是业务阶段状态。

### 5.3 禁止替代

- 不用 shared overlay 代替 Agent runtime transcript。
- 不用 Agent transcript 直接代替 Room shared 历史。
- 不用 `sdk_session_id`、数据库 `sessions.id` 或 `session_key` 反推 Room 页面路由。

## 6. 输入与输出主链

### 6.1 用户公区输入

1. 入口校验共享键 `room:group:<conversation_id>`；DM 的执行由唯一 Agent session 承接。
2. 解析目标 Agent：显式 `target_agent_ids`、文本 `@`、单成员默认、host 默认接管；仍无目标时沿最近活跃 root round 的成员继续投递。
3. 用户消息最终写入 shared overlay 并广播实时事件；忙碌目标先登记持久化输入队列，派发时补齐或更新公区投影。
4. 为目标 Agent 创建、复用或排队 round slot；忙碌目标进入 guide/queue/interrupt 路径。
5. 已收口的执行终态按 transcript 引用或合成 assistant 投影到 shared overlay。
6. assistant 公区终态中的非代码 `@成员` 在其 source slot 成功收口后立即触发 handoff；同一 root round 的其他 slot 可以继续运行。

以上规则仍没有可解析目标时，消息仍可记录，但不会启动 Agent；平台只返回目标提示，不替业务规则猜测目标。

### 6.2 Agent 私域输入

Group Room 且 `private_messages_enabled=true` 时，runtime 才获得 Room 协作工具。`send_directed_message` 负责写入私域记录并按策略唤醒；被唤醒成员的 final reply 按 `reply_route` 投影。

### 6.3 公区主动广播

普通公开发言使用当前 round 的 final reply。`publish_public_message` 只用于私域或 tool-driven 流程需要额外发布一条公区事实的场景；成功后当前 slot 不再重复投影默认 final reply。

## 7. 路由键的职责

| 用途 | 使用的键 |
| --- | --- |
| Room 页面/API | `room_id + conversation_id` |
| Room/DM shared stream | `room:group:<conversation_id>` |
| 某 Agent 的 Room runtime | 由 `BuildRoomAgentSessionKey` 生成的 Agent key |
| SDK transcript 恢复 | `sdk_session_id` |
| 数据库运行时索引 | `sessions.id` |

任何跨层调用都必须使用对应 builder/parser，不手拼字符串。

## 8. 稳定不变量

- Room 成员、conversation 和 session 的归属由 SQL 校验。
- 共享正文与私有正文的来源显式分离。
- 私域正文不会因普通投影自动进入 public feed。
- public handoff 由独立的 append-only ledger 持久化；Input queue 只负责忙碌目标的投递。
- `source_agent_id`、Room scope 和 root/cause 关联由受控运行时/后端绑定；`reply_route` 必须由后端校验并按成员范围归一化。
- cursor 只在实际消费到连续输入后推进；失败或取消不能无条件标记已读。
- Room Skill 决定业务流程，Room 平台只负责路由、可见性、持久化、唤醒和运行时护栏。

## 9. 不在这里解决的问题

以下问题应在对应规范或业务模块中讨论，不回填到 Room 核心模型：

- Goal 是否完成、如何续跑以及如何计费。
- 业务 Skill 的阶段、顺序、投票、主持人和超时。
- 前端时间线如何分组、折叠和分页。
- runtime provider、MCP 工具和 transcript 的内部协议。

一句话：Room 是共享协作容器；conversation 是共享对话；session/slot 是成员执行边界。共享视图和私有运行时必须协同，但不能混成一层。
