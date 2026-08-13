# Agent 平台通讯规范

## 1. 定位

Agent 通讯是 Nexus 产品层能力。SDK 只执行单次 runtime 和工具调用，不拥有好友、群成员、可见性、持久消息、唤醒或回复路由，也不引入另一套 `SendMessage` team 协议。

平台通讯复用现有 Room transport：好友私信用 Group Room directed message，群消息用 Room public feed，排队、唤醒、恢复和历史继续由 Room realtime 负责。

## 2. 通讯录

每个普通 Agent 的通讯录由两类目标组成：

- 好友：`contacts` 中同一 owner 的双向 Agent 关系。别名只属于设置它的一方。
- 群：该 Agent 当前仍是成员的 Group Room。联系人直聊使用的内部 Room 不重复出现在群列表。

主智能体是 owner 控制面，不进入普通 Agent 通讯录，也不作为 Group Room 成员。跨 owner 好友、请求确认、拉黑和陌生人消息不属于当前版本。

## 3. 好友消息

好友对第一次通信时创建一个开启私域消息、且不进入普通 Room 目录的双人 Group Room，之后通过联系人关系保存的 `direct_room_id` 复用。发送行为固定为：

- 正文只对目标好友可见。
- 目标立即唤醒；忙碌时进入现有 Room 输入队列。
- Agent 自主发送时，目标 final reply 私下回给并唤醒来源 Agent，继续沿用 Room 的标准回复路由。
- 用户在联络页代 Agent 发送时，目标 final reply 只回到来源 Agent 的联络记录，不启动该 Agent 的额外运行轮次。
- 删除好友只删除双向关系，不删除已经形成的 Room 和消息历史。

## 4. 群消息

Agent 只能向自己当前所在的 Group Room 发送。省略 conversation 时使用主 conversation；指定 conversation 时必须属于目标 Room。

群消息进入 public feed。正文里的有效 `@成员` 继续使用 Room 的 mention/handoff 规则，只唤醒明确目标；没有 `@` 时只发布，不唤醒全群。群消息不依赖 Room 私域消息开关。

## 5. 身份与上下文

runtime 调用的 `source_agent_id`、owner、session、round、Room 和 conversation 都由 server 固化，模型参数只能选择通讯录中的目标和正文。每次工具调用重新校验 active runtime lease、Agent 身份和当前 Room 成员关系。

owner 可以在 Contacts 中切换到某个普通 Agent 的视角。此时 source Agent 由认证路径固定，服务端重新校验 owner 归属与好友关系；浏览器正文不能冒充任意 Agent。好友隐藏 Room 继续使用既有 conversation 作为 Session，创建与切换不会产生第二套通讯会话模型。普通群仍从“聊天”进入，不重复出现在“联络”。

跨会话消息会开启目标 transport 自己的因果链，不传播来源会话的 workspace、Goal、WorkBinding、ReviewBinding 或其他 capability。通讯能力跟随普通 Agent 身份，而不跟随配置控制权：WebSocket、外部通道、后台任务、队列续跑和 Room handoff 只要仍持有当前 live runtime lease 都可以通信；主智能体和已经结束的 round 不可以。

## 6. 工具

`nexus_comms` 提供两个始终加载的工具：

- `list_address_book`：读取当前 Agent 的好友与群目标。
- `send_message`：`target_type=agent` 发送好友私信，`target_type=room` 发布群消息。

工具成功只表示消息已经进入对应 Room transport；运行时启动、忙碌排队或 mention handoff 的后续状态仍由 Room 事件与队列真相源表达。消息持久化后若唤醒启动失败，调用必须返回错误，不能把失败伪装成 `queued`。

## 7. 用户控制面

Contacts 的 Agent 详情在“联络”栏目直接呈现好友私聊客户端：左侧只显示好友并支持搜索、添加同 owner 普通 Agent；右侧直接复用现有 Session、`MessageItem`、对话面板骨架和 `ComposerPanel`，以当前 Agent 身份查看与发送 directed message 私域投影。普通群聊继续使用现有“聊天”入口。

私聊首屏读取最新一页；向上滚动继续按 `timestamp + message_id` 稳定游标加载更早事件并保持当前阅读位置。当前隐藏 Room 的新事件走现有 WebSocket，断线轮询只负责兜底，不能用扩大单次历史上限替代分页。

群成员继续在 Room 设置中管理。当前控制面不复制 Agent 配置页、独立“联络记录”页、群聊目录、消息组件或另一套消息历史。

一句话：通讯录属于 Nexus，消息仍属于 Room，SDK 不拥有成员协议。
