# Agent 平台通讯规范

## 1. 定位

Agent 通讯是 Nexus 产品层能力。SDK 只执行单次 runtime 和工具调用，不拥有好友、群成员、可见性、持久消息、唤醒或回复路由，也不引入另一套 `SendMessage` team 协议。

平台通讯按场景适配：好友私信用 Group Room directed message，群消息用 Room public feed，IM 投递与来源记录由 Channels 和宿主数据库负责。IM 回传再交给原 Session 的 DM 队列或 Room 私域收件箱；各场景保留自己的排队、可见性与回复语义。

## 2. 通讯录

每个普通 Agent 的通讯录由三类目标组成：

- 好友：`contacts` 中同一 owner 的双向 Agent 关系。别名只属于设置它的一方。
- 群：该 Agent 当前仍是成员的 Group Room。联系人直聊使用的内部 Room 不重复出现在群列表。
- 外部私聊：同一 owner、同一 Agent 下仍 active-paired 且真实存在的外部 DM Session。模型只能使用目录返回的完整 SessionKey，不能拼装平台收件人。

主智能体是 owner 控制面，不进入普通 Agent 通讯录，也不作为 Group Room 成员。跨 owner 好友、请求确认、拉黑和陌生人消息不属于当前版本。

## 3. 好友消息

好友对第一次通信时创建一个开启私域消息、且不进入普通 Room 目录的双人 Group Room，之后通过联系人关系保存的 `direct_room_id` 复用。发送行为固定为：

- 正文只对目标好友可见。
- 目标立即唤醒；忙碌时进入现有 Room 输入队列。
- Agent 自主发送时，目标 final reply 私下回给并唤醒来源 Agent，继续沿用 Room 的标准回复路由。
- 用户在联络页代 Agent 发送时，目标 final reply 只回到来源 Agent 的联络记录，不启动该 Agent 的额外运行轮次。
- 删除好友只删除双向关系，不删除已经形成的 Room 和消息历史。

## 4. 群消息

Agent 只能向自己当前所在的 Group Room 发送。Room runtime 向当前 Room
发送时，省略 `conversation_id` 必须使用宿主固化的当前 conversation；不能
回退到 Room 主 conversation。向另一个 Room 发送必须显式提供
`conversation_id`，且该 conversation 必须属于目标 Room，否则 fail closed。
owner 通讯客户端和 DM/外部 Agent runtime 没有可信 Room conversation 上下文，
省略时继续使用目标 Room 主 conversation，保持旧调用兼容。

群消息进入 public feed。正文里的有效 `@成员` 继续使用 Room 的 mention/handoff 规则，只唤醒明确目标；没有 `@` 时只发布，不唤醒全群。群消息不依赖 Room 私域消息开关。

## 5. 身份与上下文

runtime 调用的 `source_agent_id`、owner、session、root round、Room 和 current
conversation 都由 server 固化，模型参数只能选择通讯录中的目标和正文。每次
工具调用重新校验 active runtime lease、Agent 身份和当前 Room 成员关系。

owner 可以在 Contacts 中切换到某个普通 Agent 的视角。此时 source Agent 由认证路径固定，服务端重新校验 owner 归属与好友关系；浏览器正文不能冒充任意 Agent。好友隐藏 Room 继续使用既有 conversation 作为 Session，创建与切换不会产生第二套通讯会话模型。普通群仍从“聊天”进入，不重复出现在“联络”。

跨会话消息的可见内容进入目标 transport，但因果链保留宿主固化的 source root
round。若来源是当前 Room Goal continuation，host 还携带 exact Goal ID 与
objective revision 作为 collaboration attribution；它跨 directed-message、handoff
ledger、InputQueue 和重启恢复保持，并只用于等待、审计及重新调度来源 Goal。
该 attribution 绝不授予目标 round Goal mutation authority，也不传播来源
workspace、WorkBinding、ReviewBinding 或其他 capability。普通 Room round 不会
伪造 Goal attribution。

通讯能力跟随普通 Agent 身份，而不跟随当前聊天 transport：WebSocket、外部通道、后台任务、队列续跑和 Room handoff 只要仍持有当前 live runtime lease，都共享该 Agent 的联系人、Room 与 active-paired 外部 DM 目录；主智能体和已经结束的 round 不可以。跨 transport 发送不会合并 transcript，消息仍写入实际目标 Session，并在每次发送时重新校验 owner、Agent、真实 Session 与 active pairing；撤销配对立即失效。

## 6. 工具

`nexus` MCP 中的平台通讯工具组提供两个始终加载的工具：

- `list_targets`：无参数或 `scope=address_book` 读取当前 Agent 的好友、群与已配对外部私聊目标；`scope=delivery_sources` 在当前已配对 IM 会话内查询投递来源。
- `send_message`：DM/外部 Agent runtime 使用 `destination=contact|room|external_session|delivery_source`；Room runtime
  额外支持宿主绑定的 `destination=current_room`，并以
  `visibility=private|public` 选择当前 Room 私域或公区。

`send_message` 的工具名保持统一，但 Schema 随可信来源上下文收窄。DM 不暴露当前
Room 的 recipients、wake 或 reply route；Room 的当前私域发送才接受这些投递参数。
`external_session` 的 `target_id` 必须来自 `list_targets.external_sessions[].session_key`；当前外部私聊的正常回复仍直接使用 final reply。内部继续复用 directed message、public feed 与 Channels delivery，不扩张为多个模型工具。

工具成功只表示消息已经进入对应 Room transport；运行时启动、忙碌排队或 mention handoff 的后续状态仍由 Room 事件与队列真相源表达。消息持久化后若唤醒启动失败，调用必须返回错误，不能把失败伪装成 `queued`。

成功的 `send_message` 本身不算 Goal continuation progress。只有持久化且带 exact
Goal revision 的 handoff/queue receipt 可以让 continuation 暂缓；真正清零空进展
只能来自显式 applied Goal mutation，或 exact Goal-bound 的 applied WorkGraph
mutation。`list_targets`、普通消息发送和其他 read/list/todo 工具都不能作为
续跑存活证据。

## 7. 用户控制面

Contacts 的 Agent 详情在“联络”栏目直接呈现好友私聊客户端：左侧只显示好友并支持搜索、添加同 owner 普通 Agent；右侧直接复用现有 Session、`MessageItem`、对话面板骨架和 `ComposerPanel`，以当前 Agent 身份查看与发送 directed message 私域投影。普通群聊继续使用现有“聊天”入口。

私聊首屏读取最新一页；向上滚动继续按 `timestamp + message_id` 稳定游标加载更早事件并保持当前阅读位置。当前隐藏 Room 的新事件走现有 WebSocket，断线轮询只负责兜底，不能用扩大单次历史上限替代分页。

群成员继续在 Room 设置中管理。当前控制面不复制 Agent 配置页、独立“联络记录”页、群聊目录、消息组件或另一套消息历史。

通讯录和投递来源属于 Nexus；各 transport 拥有自己的消息与入队事实，SDK 不拥有成员协议。


## 8. IM 投递来源与反馈回传

### 8.1 入口与记录

不新增 MCP server、工具或定时任务。`send_message(destination=external_session)` 的宿主调用上下文，以及 Automation 向已配对 IM 私聊投递结果的 producer 上下文，进入同一个 IM 来源适配器。

发送前写入宿主数据库的 `im_deliveries`：owner、来源 Agent、精确 Session 及创建时间、round/tool call（或 job/run）、目标 Session 及创建时间、pairing、正文不可变快照和投递时间。不是 Agent MEMORY 文件，也不是 Room 私聊 ledger。正文快照用于重启后查询和同意图重试核对，不靠改写后的 transcript 重建。旧投递不按文本或时间推断来源。

普通调用按可信 round/tool-use/目标身份去重；真实 tool-use ID 由 Bridge 从运行时 `params._meta["claudecode/toolUseId"]` 传入；不以业务正文哈希代替调用身份。相同真实调用重试复用记录，两个正文相同但 tool-use ID 不同的调用分别记录；缺少调用身份时明确报 runtime/Bridge 合同不可用，不解释为用户权限不足；在物理发送之前持久化 `unknown`。成功记录 `sent` 和平台回执，已知尚未调用外部平台的失败记录 `not_sent`；外部调用结果未知不自动补发。Automation 重投继续由原 Automation attempt 机制授权，来源记录不授予重投权。

### 8.2 IM 内查询和转交

当前 IM round 获得最近至多 5 条投递的有界上下文。更多记录由模型调用：

```json
{"scope":"delivery_sources","query":"草案","limit":10,"offset":0}
```

指定 `delivery_id` 查看完整正文及回传收据，不能与分页/搜索混用。查询只返回当前 owner、当前已配对 IM Session 的记录，不接受来源 Session 参数。结果包含 `can_reply`、不可用原因以及可用时的 `current_input_message_id`。

智能体结合人类消息判断“确认”“修改”“回复过去”等反馈；多条来源无法确定时先消歧，不能自动选最新一条。确定后使用同一工具：

```json
{"destination":"delivery_source","target_id":"<delivery_id>","content":"请给 T4 增加人力资源部协办"}
```

如果当前消息只是对上一条反馈的消歧，可以指定 `content_source_message_ids`（1–10 条）；默认取当前人类消息。原始消息必须属于当前 IM Session 和 pairing。输入证据来自宿主 `im_ingress_messages.delivery_input_json`，以真实 ingress request、round、实际派发正文校验；模型正文、长期记忆或被编辑的工作区队列不能充当人类消息证据。

宿主按 delivery 解析返回地址，模型不能填写 Session、任意收件人、wake 或 reply route。回传包含转交内容和可核对的人类消息原文。模型负责理解反馈；系统不把自然语言升级成审批状态，不改变任务完成条件。

### 8.3 原会话受理与恢复

`im_delivery_replies` 持久保存反馈意图。同一次人类请求对同一 delivery 只建立一条不可变回传；不同人类消息允许多次补充。重试相同意图返回已有收据，改变内容会冲突。

- DM：进入精确来源 Session 的现有 InputQueue。忙碌时排队，不作为 guide 注入旧 round；空闲后让原 Agent 开始新 round。`accepted` 只表示已入队，`started` 表示已领取派发，不代表任务成功或业务批准。
- Room：进入精确原 conversation 中来源 Agent 的私域收件箱，由原 directed-message wake ledger 负责恢复。回传不会自动公开，final reply route 固定为 `none`；私域关闭或成员离开时拒绝，不退回主 conversation 或公区。来源 Agent 仍可根据任务使用原有公开消息工具。
- Automation：核对原 job/run/Session/round 与当前权限版本，反馈新轮次仅使用该任务可验证的工具限制；不复活旧 run、不改其终态或调度。没有可验证快照、任务删除/会话失效、权限版本变化，以及不支持隔离工具策略的 Automation Group 来源均拒绝回传。

启动按分页读取未完成的本地回传意图，修复已持久化但尚未入队的反馈。已领取派发但执行结果不明时不自动重放；入队凭据存在但队列项已消失且没有派发证据时记为 `needs_attention`。这一恢复只处理本地反馈，不补发外部 IM 消息。

### 8.4 权限与兼容

发送、查询、回传和 DM 队列派发分别核对各自当前身份。返回地址按 owner、Agent、Session 创建时间、当前 pairing 及 Room 成员定位；原 Session 删除/替换后不能创建替代 Session。配对禁用、改绑或删除与旧投递回传资格撤销在同一数据库事务中提交，重新启用不恢复旧记录。

反馈是外部输入，只进入新的执行轮次。它不携带来源旧 round 的 Goal、Execution、WorkBinding、ReviewBinding、configuration 或 Automation command authority。

`contact` 和 `current_room/private` 的参数、final reply、唤醒与 ledger 不变；无参 `list_targets` 仍是原通讯录。IM 会话与 App 会话仍有独立 transcript，新增的只是可追溯投递和明确回传。

## 9. IM 私聊绑定本地 Room 成员

配对保留既有 IM 传输身份，执行目标可选独立 IM 会话，或同 owner 的本地 Group Room、精确 Conversation 和当前配对 Agent。Room 必须开启私域消息，Agent 必须仍是有效且未暂停的成员。第一版不绑定在线 Team Room，也不将 IM 群聊开放给成员私域。

能力页的配对行提供「会话目标」入口。Room 输入作为带外部来源标记的成员私域定向消息，忙碌时排入原成员队列，复用原成员上下文和协作能力。仅同一 root、成员、Session 的完整 assistant 回复发回 IM，其他成员输出与公区消息不会自动镜像。原有 delivery_id 反馈回传继续独立工作。

切换携带当前 binding_version；目标变化递增版本，不重新登录、不迁移历史、不取消已提交任务。返回地址在受理前持久冻结，发送前持锁校验版本，因此切回旧目标也不会恢复旧回复资格。未知物理发送不自动重发。Room 命令和私域队列按持久输入 ID 幂等，恢复沿用原始目标及正文，禁止改投当前新目标。

权限通知只呈现绑定话题及成员，`/y`、`/a`、`/d` 只解析该成员 runtime key 的请求；多个请求要求在 Nexus 逐项处理。改绑后旧任务的后续权限请求仍可在原 Room 处理。
