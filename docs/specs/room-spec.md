# Room 模块规范

## 1. 文档定位

本文定义 Room 模块的领域边界、数据归属、路由键与侧栏活动投影。

相关规范：

- [消息处理规范](./message-processing-spec.md)：实时消息、历史投影和 round 分页。
- [Session Key 规范](./session-key-spec.md)：共享会话键、Agent 私有会话键和恢复键。
- [Room 协作协议](./room-collaboration-spec.md)：公区、私域、目标解析、handoff、唤醒和回复投影。
- [Agent 平台通讯规范](./platform-communication-spec.md)：好友/群通讯录与跨当前会话发送。
- [Execution Orchestration 协议](./execution-orchestration-spec.md)：Plan、Work Item、Assignment、验收与 Goal 持续性边界。
- [Room Skill 编写指南](../guides/room-skill-authoring.md)：面向 Skill 作者的最小行为规则。

## 2. 模块范围

Room 模块负责：

- 房间、成员和 conversation 的生命周期。
- Room conversation 的共享消息投影。
- 每个成员 Agent 的私有 runtime 启动、恢复、中断和清理。
- 公区输入、成员目标解析、Room round 和输入队列。
- Group Room 的 directed message、私域上下文和唤醒。
- Room 级配置：host 默认接管、Room Skill 和私域消息开关。
- Agent 平台通讯复用的好友私信与群消息 transport。

不属于本模块（在对应规范或业务模块中定义）：

- Goal 的业务状态、预算、续跑和计费。
- Work Item、Assignment、依赖、交付和 Acceptance（属于 Execution Orchestration）。
- runtime provider、工具执行、MCP 与 transcript 内部格式。
- 通用消息归一化与前端时间线的分组、折叠和分页。
- Room Skill 的业务规则（阶段、顺序、投票、主持人、胜负、超时和收口条件）。

## 3. 核心对象

### 3.1 Room

Room 是成员和 conversation 的容器。`room_type` 只有两种：

- `room`：多人协作 Room，可配置 host、Room Skill 和 directed message。
- `dm`：单 Agent 的 Room，不启用 Room Skill 或 directed message。

好友私信通道是带内部 `is_contact_channel` 标记的 `room`，不进入普通 Room 目录。该标记不是第三种 Room 类型，也不能由 HTTP 请求创建。

### 3.2 Member

Member 是 Room 的成员关系，类型只有：

- `user`：Room owner。
- `agent`：参与该 Room 的 Agent。

成员属于 Room，不属于某条 conversation。Agent 能否被路由，以当前 Room 成员表为准。

### 3.3 Conversation

- Conversation 是 Room 内独立的共享对话。
- 每个 Room 至少有一个主 conversation，也可以有 `topic` conversation。
- `conversation_id` 是 Room 页面和 Room HTTP API 的共享对话路由键。
- 删除 topic 会同时关闭其运行时；主 conversation 不能删除。

### 3.4 Session

- Session 是数据库中的 `conversation + agent` 运行时索引，保存 runtime 标识、版本、状态和最近活动时间。
- 它不是前端路由键，也不是 SDK resume id。
- Group Room 每个 conversation 的每个 Agent 有独立的私有 runtime session；DM 只有一个 Agent session。

### 3.5 Round 与 slot

- `round`：一次共享输入或一次 Room 唤醒形成的执行批次。
- `slot`：该 round 中某个 Agent 的实际执行槽。
- 同一 root round 下可运行多个 Agent slot；`agent_round_id` 标识 slot，`round_id` 对外表示根 round。

## 4. 两层运行模型

Room 必须把共享协作层和成员执行层分开：

| 层 | 负责什么 | 主要真相源 |
| --- | --- | --- |
| Shared conversation | 公区事实、用户消息、共享历史和 Room 页面 | SQL 关系 + Room overlay + transcript reference |
| Agent runtime session | 单个 Agent 的模型上下文、工具执行、恢复和私域消费位置 | runtime transcript + Agent overlay |

- 共享层可以引用成员 transcript 中已完成的 assistant，但不拥有成员的完整私有正文。
- 成员 runtime 不能直接代替 Room shared 视图。

`dm` 与 Group Room 共用 Room 数据模型，但执行所有权不同：

- `room:group:<conversation_id>` 只代表共享消息投影和页面订阅入口，不能持有或选择 SDK resume。
- Group Room 的成员 runtime 由 Room realtime 按 `BuildRoomAgentSessionKey` 启动、恢复和中断。
- DM 的唯一 Agent runtime 只由 DM service 按 `agent:<agent_id>:ws:dm:<conversation_id>` 承接。
- Room 订阅恢复可以订阅 DM 的共享事件，但不得读取、派发 DM 输入队列，也不得创建 Room runtime。

## 5. 历史与投影边界

### 5.1 Room shared 历史

Room shared 历史由两类行组成：

- inline overlay：用户消息、合成 assistant、result 摘要和其他 Nexus 语义。
- `transcript_ref`：指向成员 transcript 中已完成 assistant 的引用。

读取规则：

- 读取时解析引用并统一投影。
- 对外以已收口的 assistant 为主；result 作为 `result_summary` 附着在 assistant 上。
- 未完成的过程态不能成为公区事实；错误和中断只以终态摘要出现。
- 公区 assistant 的 `agent_mentions` 随 transcript reference 一起保留，不能只存在于实时事件或内存 handoff 中。

### 5.2 成员私有历史

- 成员 runtime 的完整上下文保留在自己的 transcript 与 overlay 中。
- 公区 cursor、directed message cursor 和 checkpoint 只表示消费边界，不是业务阶段状态；推进规则见 [Room 协作协议 §7](./room-collaboration-spec.md)。

### 5.3 禁止替代

- 不用 shared overlay 代替 Agent runtime transcript。
- 不用 Agent transcript 直接代替 Room shared 历史。
- 不用 `sdk_session_id`、数据库 `sessions.id` 或 `session_key` 反推 Room 页面路由。
- 不能把 DM 与 Room 的队列项放进同一执行域；共享物理日志必须按 `InputQueueScope` 回放过滤。
- Room realtime 不得为 `room_type=dm` 启动进程；共享流键和执行键可以同时存在，但执行所有权只能有一个。

### 5.4 Room-backed Session 读模型

- SQL 只拥有 Room 身份、标题与配置；workspace/Room ledger 拥有运行历史进度。
- 统一读模型必须单调合并两者。
- 旧 SQL `messages` 计数只能作为兼容下限，禁止覆盖 canonical Goal 控制记录、标题、最近活动、消息数、上下文占用或 transcript lineage。

## 6. 用户公区输入主链

目标解析、handoff、directed message、公区广播和投递策略的细则见 [Room 协作协议](./room-collaboration-spec.md) §4–§6。主链顺序：

1. Group Room 入口校验共享键 `room:group:<conversation_id>`；DM 的消息、队列和中断都走唯一 Agent session。
2. 解析目标 Agent：显式 `target_agent_ids`、文本 `@`、单成员默认、host 默认接管；仍无目标时沿最近活跃 root round 的成员继续投递。
3. 用户消息写入 shared overlay 并广播实时事件；忙碌目标先登记持久化输入队列，派发时补齐或更新公区投影。
4. 为目标 Agent 创建、复用或排队 round slot。
5. 已收口的执行终态按 transcript 引用或合成 assistant 投影到 shared overlay。

仍没有可解析目标时，消息可记录但不启动 Agent；平台只返回目标提示，不替业务规则猜测目标。

## 7. 路由键的职责

| 用途 | 使用的键 |
| --- | --- |
| Room 页面/API | `room_id + conversation_id` |
| Room/DM shared stream | `room:group:<conversation_id>` |
| 某 Agent 的 Room runtime | 由 `BuildRoomAgentSessionKey` 生成的 Agent key |
| SDK transcript 恢复 | `sdk_session_id` |
| 数据库运行时索引 | `sessions.id` |

跨层调用必须使用对应 builder/parser，不手拼字符串。

## 8. 侧栏活动投影

- 聊天侧栏的执行态只投影为瞬时 `active room_id` 集合：任一 Agent slot 仍在执行时 Room 保持激活；root 终态或全部 slot 终态后清除。
- 执行态与待确认人工交互只按 Room ID 输出；容器内部必须按精确 Conversation/Session source 隔离后取并集，空快照或终态不得清除其他 source。
- Room 活动快照必须携带捕获时的 `room_seq` 作重放栅栏。
- 持久 Assistant 历史只表达结构和终态，不得独立复活执行态。
- `dm` 与 `room` 使用同一规则；禁止把 Agent runtime 状态或持久化 `is_active/status` 混入聊天行。
- 联系人侧栏只展示 Agent 目录元数据，不订阅 runtime、不显示执行徽标或任务计数；运行态事件只属于打开会话的工作区链路。

## 9. 稳定不变量

- Room 成员、conversation 和 session 的归属由 SQL 校验。
- 共享正文与私有正文的来源显式分离；私域正文不会因普通投影自动进入 public feed。
- Room Skill 决定业务流程；Room 平台只负责路由、可见性、持久化、唤醒和运行时护栏。
- handoff ledger、输入队列、`source_agent_id`/scope 绑定、`reply_route` 校验与 cursor 推进的不变量见 [Room 协作协议](./room-collaboration-spec.md) §5、§8、§9。
