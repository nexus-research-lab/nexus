# IM 投递来源记录与按需回传方案

> 状态：non-normative，历史设计提案（2026-09-15）；实现后的当前合同见 [平台通讯规范](../specs/platform-communication-spec.md#8-im-投递来源与反馈回传)。下文保留设计过程，不作为当前字段和行为定义。
> 依据：当前仓库源码和用户确认的产品边界。当前行为仍以代码与 docs/specs 中的规范为准。

## 实现与原提案的差异（2026-09-16）

- 宿主数据库保留不可变正文快照，用于重启后核对和消歧；不只依赖历史消息引用。
- 人类输入证据复用现有 ingress ledger，未增加独立人类消息表。
- Group 来源反馈固定进入来源 Agent 私域，不自动继承原来的公开回复路由。
- Automation 反馈使用新轮次和当前可验证的任务工具限制；不能安全施加该限制的来源明确拒绝。
- 当前覆盖本地存储、适配器、排队与恢复测试；真实微信/飞书往返仍需部署后验收。

## 1. 要解决的问题

用户在电脑端给某个智能体安排任务，任务需要手机端微信或飞书确认。手机能收到材料，也能回复；电脑能看到 IM 会话的消息，但原任务所在的 Session 没有收到该反馈，因而不能继续。

同一个智能体的 App Session 与 IM Session 有独立的上下文。电脑显示 IM 会话历史，并不等于该消息进入原任务的模型输入。本次保留这个会话边界，为跨会话投递补足来源和回传能力。

IM 会话里的智能体应结合投递内容理解“确认”“不同意”“增加人力”等自然回复，找到该次投递来源，并把对应反馈交回原 Session。人也可以明确说“回复过去”，但无需固定口令。

用户已明确确认：回传后默认让原智能体继续处理反馈。

本次能力的完整链路是：

1. 每次跨会话投递时，宿主保存发送者、来源 Session 和投递记录。
2. IM 收信仍进入当前已配对的 IM Session。
3. IM 智能体判断人正在反馈哪次投递，必要时查询或消歧，再调用回传工具。
4. 宿主根据记录确定返回地址，将反馈持久提交到原会话。
5. 原智能体在原会话处理反馈；忙碌时按顺序排队。

IM 侧智能体负责理解这条消息应关联哪次投递，并如实转交；“收到”“同意”“进度 100%”能否推动任务继续，由原 Session 中的智能体及已有业务流程判断。通讯层只确认反馈的转交状态。

## 2. 产品边界

### 2.1 本次包含

- 普通 send_message 外部投递与 Automation 结果外投的来源记录。
- 当前 IM 私聊内对已收到投递的来源查询。
- 由智能体调用工具、针对某次投递的显式回传，同一投递允许多次独立反馈；显式指工具选定记录，不要求人使用固定转交措辞。
- 返回精确来源 Session，保留来源说明，并启动原智能体的新一轮处理。
- 必要的持久化、去重、当前权限复核和失败反馈。
- 复用已有消息、队列、会话和通道展示。

### 2.2 本次不承担的业务

不建立审批单、等待审批状态机、草案版本管理、任务验收、自动催办或升级督办。也不因一次外投就暂停 Goal、挂起工具调用或创建定时任务。

普通 IM 消息继续在 IM 会话处理。仅收到一条新 IM 消息，不会自动转发到之前发送过消息的全部 Session。

回传记录只指向该次投递的直接来源。如果 A 让 B 代发，而真正调用外投工具的是 B 的某个 Session，来源就是 B 的该 Session；不从正文猜测 A，也不递归追踪委托链。

外部收件人范围继续服从现有 active pairing 和 Agent 通讯授权。本次不增加跨 owner、任意 Agent 或任意 Session 的寻址能力。

## 3. 当前实现与准确修改点

| 现有位置 | 当前行为 | 本次需要补充 |
| --- | --- | --- |
| [runtime/communication_mcp.go](../../internal/app/runtime/communication_mcp.go) | 宿主构造 owner、Agent、逻辑 Session、round、Room/conversation 和 runtime lease | 将经过验证的来源身份交给投递服务；IM 回传调用还需要可信的当前人类入站消息身份 |
| [communication/service.go](../../internal/service/communication/service.go) | SendMessage 已获得 Actor；外部目标分支只向下传 Agent、目标 Session 和正文 | 保存完整的出站来源；外部回传根据记录解析目标 |
| [channels/router_delivery.go](../../internal/service/channels/router_delivery.go) | 普通主动外投先投影到目标 Session，再调用平台发送 | 为投影和物理发送携带同一个 delivery_id，回写发送结果 |
| [channels/automation_delivery.go](../../internal/service/channels/automation_delivery.go) | 按 run_id 投影结果，已有 producer Agent、job/run、execution Session/round 元数据 | 使用相同的投递来源记录，保持每个 run 的独立来源 |
| [channels/ingress_accept.go](../../internal/service/channels/ingress_accept.go) | 入站经过配对验证、去重和控制命令处理后，进入已配对 Agent 的 IM Session | 保持接收路线；把当前入站消息身份交给回传工具的可信上下文 |
| [conversation/automation_delivery_context.go](../../internal/service/conversation/automation_delivery_context.go) | 已有机制把接收会话中新到的 Automation 投递补进下一轮模型上下文 | 将有来源的普通外投也纳入同一会话投递上下文适配，提供记录 ID 和原投递内容，避免模型只看到一句孤立的“确认” |
| [dm/input_queue.go](../../internal/service/dm/input_queue.go) | 持久队列接收输入并串行派发 | 增加有来源的外部反馈接收入口，保留回传身份，不伪装成 Web 用户输入 |

当前外部发送分支没有保存完整 Actor 来源，也没有“按某次外投回到原 Session”的操作。只给模型增加一段提示词，无法补齐持久回路。

现有 ExternalDeliveryReceipt 主要关联目标会话中的 assistant 消息与平台发送回执；它不等于来源查询索引。现有 remembered delivery route 只解决往哪个外部地址发送，也不能替代逐条来源记录。

## 4. 来源记录模型

建议增加一个聚焦于本能力的 storage/communication repository，保存投递记录和回传记录。现有 transcript、overlay 和平台回执继续负责各自的事实，通过 ID 引用连接。

物理落点是 Nexus 已配置的宿主主数据库。SQLite 默认文件为 NEXUS_STATE_ROOT/app/data/nexus.db，见 [数据库配置](../../internal/config/config.go)；若部署使用 PostgreSQL，则使用同一个已配置主库中的对应表。无需新增数据库、Agent 工作区文件或 IM 专用缓存。

新增两张表：

- im_deliveries：本次外投的来源、目标、消息引用及发送事实。
- im_delivery_replies：针对某次投递的反馈、真实人类消息引用及原 Session 的转交定位。

按 owner + 目标 IM Session + 时间/ID 建查询索引，并按来源 Session 建失效处理索引。智能体通过既有工具的新增范围读取；宿主服务负责写入、校验和维护。

正文继续保存在目标会话已有消息历史中；来源表只保存引用、必要预览及摘要校验值。数据库行不复制来源 Session 全部历史，也不保存 IM token、SDK 恢复凭据或旧 round capability。

目标 IM Session 删除时按该会话的数据清理流程移除对应查询索引和回传记录；来源 Session 删除时，使对应返回地址失效，目标会话已经收到的消息仍服从自身历史清理规则。App 重启不会清空这些记录，也不因一次回传成功就删除原投递。

### 4.1 投递记录 Delivery

一条记录代表一次逻辑投递，不代表一个联系人或整个 IM Session。

| 字段组 | 含义 |
| --- | --- |
| delivery_id、owner_user_id | 宿主生成的稳定记录 ID 与租户边界 |
| source_agent_id、source_session_key | 真正发起消息的智能体和 Nexus 逻辑 Session |
| source_kind、source_round_id、source_tool_use_id | 来源类型和发送因果；round/tool 身份用于审计、去重，不用于恢复旧授权 |
| source_room_id、source_conversation_id、return_visibility | Room 来源的精确会话及允许的回传可见范围，由宿主取得 |
| job_id、run_id | 仅自动化来源填写；不同 run 独立关联 |
| target_agent_id、target_session_key、pairing_id | 实际接收投递的已配对 IM Session 与配对身份 |
| channel、account_id、external_ref | 平台地址事实，由宿主从已经验证的目标解析 |
| target_message_id、content_preview、content_digest、created_at | 定位投递内容；预览为派生信息，完整正文复用目标会话已有消息 |
| send_state、physical_attempt_id、platform_receipt_refs | 发送事实、本次物理发送边界及平台回执关联，不能表示业务批准 |
| source_unavailable | 来源删除等情况下的回传失效事实 |

来源必须使用产品的 session_key，不能使用会随 runtime 切换而变化的 SDK Session ID。名称和标题只用于展示，不能作为路由键。

source_agent_id 与 target_agent_id 分别保存，即使现有授权多数情况下要求它们相同。二者分开记录不扩大跨 Agent 发送授权。

来源与目标会话删除时要使关联失效。之后即使出现相同文本的 session_key，也不能自动解除旧记录的失效状态。

### 4.2 回传记录 DeliveryReply

一条记录代表智能体根据真实人类回复，明确关联某次投递后提交的一次反馈。

| 字段组 | 含义 |
| --- | --- |
| reply_id、delivery_id、owner_user_id | 本次回传与原投递的关联 |
| inbound_session_key、request_message_id | 当前提供关联反馈、明确要求回传或最终消歧选择的真实人类入站消息，由宿主固定 |
| content_source_message_ids | 实际提供反馈正文的人类消息引用，可包含当前 IM 会话中较早的消息 |
| external_sender_ref | 实际入站发送者的可验证身份引用；名称仅作展示 |
| forwarded_content、content_digest | 这次实际转交的正文，提交后不可被工具重试改写 |
| destination_message_id、destination_queue_id | 原 Session 的持久接受定位 |
| admission_state、failure_code | 转交、排队或需要处理的失败事实 |

唯一约束使用 owner + delivery_id + request_message_id。同一条已明确目标的人类请求针对同一次投递，工具重试返回同一个回传结果；同键不同正文或内容来源引用返回冲突。

同一个人在之后又发一条补充意见，会产生新的入站消息身份，因此可继续回传。同一条消息也可在明确指定时回复不同的投递记录，各自去重。

delivery_id 不是持有即可使用的授权令牌。每次查询和回传都重新校验当前身份及配对。

## 5. 工具合同

以下新增字段和目标类型是拟议接口，尚未加入当前 MCP schema。

### 5.1 复用现有两个通讯工具

采用现有 nexus MCP 下的 send_message 和 list_targets：前者负责发送与回传，后者负责查询可联系或可回传的目标。

当前这两个工具实际均设置了 AlwaysLoad=true。本方案复用已有挂载，只增加少量选择参数，不增加工具数量、不新增 query_deliveries 工具、不为这项能力扩展 nexus.command 领域，也不建立平行的通讯入口。

工具定义仍常驻；具体投递记录按需查询返回，不把来源目录和消息历史塞进常驻 schema。权限在服务端逐次判断，不通过按轮卸载工具代替鉴权。

### 5.2 外部发送：send_message 自动记录来源

保持现有 destination=external_session 的使用方式。模型仍然只选择已授权目标和正文；来源 Agent、Session、Room、round 由宿主自动填写。

成功结果增加 delivery_id。普通发送无需声明“等待反馈”，也无需预先知道收件人以后是否需要回复。

来源登记和发送幂等身份由宿主从精确 runtime/tool-use identity 取得，不能让模型自行填写来源，也不为这一改动增加模型侧 request_id 参数。相同调用身份的重试复用来源记录，已有发送结果不明时先核对，不自动再发。

### 5.3 来源查询：list_targets 的 delivery_sources 范围

list_targets 不带参数时保持当前好友、群与已配对外部私聊的通讯录行为。新增按需查询范围：

    list_targets({
      "scope": "delivery_sources",
      "query": "任务草案",
      "limit": 10
    })

scope=delivery_sources 时，只返回当前 IM Session 已收到投递的可回传来源；按一次投递形成一个目标，即使多条记录来自同一 Agent/Session 也不合并。来源不可用的匹配记录可返回失效原因，避免模型误判为完全不存在。

支持分页与关键词筛选。指定 delivery_id 时只读取该条来源详情及已有回传接受状态，供消歧和结果不明后的核对；不能同时指定其他查询条件来改变其范围。模型不能指定另一个 IM Session 来扩大查询。

列表返回 delivery_id、原智能体名称、原会话标题、发送时间、正文预览、发送状态和当前是否可回传。详情只读取这次已经发往当前 IM 的内容及其来源，不授予浏览来源 Session 全部历史的能力。

用户说“回复刚才那份草案”时，模型使用实际投递内容识别候选。存在多个合理候选时询问用户，例如“回复总管理员的任务草案，还是督办员的日报？”不能只按最新时间选一个。

平台带有可信的引用消息 ID 时可帮助定位，但不能假设所有平台都提供它。当前个人微信入口没有填充通用 ReplyToID，所以文本检索和消歧是必要路径。

这只是查询回复目标，服务内部使用独立的 ListDeliverySources/GetDeliverySource 方法，不把投递日志混进联系人或 Room 目录存储。

### 5.4 回传：send_message 的 delivery_source 目标

示例：

    send_message({
      "destination": "delivery_source",
      "target_id": "<查询返回的 delivery_id>",
      "content": "领导意见：T4 增加人力资源部协办，修改后再次送审。"
    })

沿用现有工具通过 destination 选择发送目标的结构。该分支的 target_id 是投递 ID，不能是任意 Session、Agent 或平台收件人。

模型选择原投递和要转交的内容。返回的 Agent、Session、Room、可见性和唤醒方式全部由宿主从来源记录解析，不能接受任意 session_key、Agent ID、conversation_id、wake_policy 或 reply_route。各目标使用封闭的分支 schema，业务读取或副作用之前先校验。

该能力只在可信的、由人输入触发的 IM 会话 round 中生效。request_message_id 从 ingress 和持久上下文取得，不能从工具参数、正文或模型声称的“用户已同意”中构造。后台任务或被回传唤醒的 round 不因此获得再次自动回传的能力。

“这条回复是否在反馈此前投递、应交回哪一条来源”由模型依据真实人类消息、投递内容和上下文判断。此前材料要求手机端确认，用户直接回复“确认”时，模型可据此回传，不需要再要求用户说“回复过去”。宿主不做关键词审批，但强制验证调用来源、记录范围和返回地址。

需要回传的文字由模型整理时，必须保留原话引用和来源事实，不能把“确认收到”改成“批准执行”。仅要求如实转交时优先原文；如用户明确要求组织措辞，转交正文与原始入站消息保持关联以便核对。

跨消息消歧时要区分“本次转交请求”和“反馈正文来源”。例如 M1 是“把意见转过去：增加人力”，M2 是对候选问题的回答“第一份”：M1 不创建回传；M2 触发真正回传，request_message_id=M2，content_source_message_ids 包含 M1。工具重试继续使用 M2 去重，不能把“第一份”当作反馈正文，也不能另用 M1 再创建一次回传。

现有 IM runtime 上下文需提供可引用的人类消息 ID。回传分支可选接受 content_source_message_ids，默认当前请求消息；使用较早消息的反馈文字时必须显式引用。宿主验证这些引用均属于当前 IM Session 的真实人类消息，且只能用于正文溯源，不能作为任意来源查询、返回地址或新的授权凭据。

服务端按 owner + delivery_id + request_message_id 去重。同一反馈即使换一次工具调用也返回同一个 reply_id；同一身份却提交不同正文或内容来源引用则返回冲突。后续新的人类消息仍可形成独立补充反馈。

工具成功只报告“反馈已交给原会话”或“已在原会话排队”。结果不明时可按 delivery_id 查询既有回传的接受状态。原智能体是否处理完、业务是否批准，继续使用原会话的实际结果。

## 6. 出站与回传流程

### 6.1 出站

1. 验证当前来源身份和目标配对。
2. 在外部发送之前持久登记来源，获得 delivery_id；保存失败则不发送。
3. 使用该 ID 将内容投影到目标会话。
4. 调用平台之前，先持久标记本次 physical attempt 已开始；该标记保存失败则不调用平台。
5. 发送到外部平台，保存平台返回的发送事实；发送失败或结果不明与来源记录一起保留。

普通工具调用按宿主的精确调用身份关联逻辑投递；同一调用重试不新建一条来源。Automation 结果使用 owner + run_id + 本次固定目标归并逻辑投递，人工重投保留同一个来源，并记录新的物理发送 attempt。

来源登记的初始状态为待发送。重启时，已开始但未收口的物理 attempt 按结果不明处理，不自动再次外投；只有明确的未调用或平台拒绝证据才能判定未发送。Automation 沿用自己的持久 attempt 身份和既有人工重投规则，不另建一套 attempt 权威。

发送状态只区分发送事实，不能把正文中的“确认”写成发送成功或业务完成。

### 6.2 入站与选择

1. 通道保持现有收信、配对校验和去重流程，消息进入当前 IM Session。
2. IM 智能体看到本会话的新到投递内容及 delivery_id，根据人类回复判断其关联；需要查旧记录或消歧时使用查询工具。
3. 没有可验证的记录时直接说明无法定位来源；多个候选时消歧。
4. 调用 send_message 的 destination=delivery_source；宿主再次核验当前 owner、IM Session、配对身份、来源可用性和当前人类入站消息。

当前会话已展示但尚未进入 SDK transcript 的跨会话投递，需像现有 Automation 结果一样补充到该轮上下文。补充内容仅限发往本 IM 的消息、来源标签、delivery_id、发送状态和当前可回传性，按消息身份去重并限量；更多历史通过 list_targets 的 delivery_sources 范围查询。普通外投和 Automation 投递共用该适配，不重复注入同一结果，不复制来源 Session 的完整历史。

目标会话中的本地投影不等于平台已经送达。明确未发送的记录不能作为人已经收到的候选；结果不明的记录须在上下文中标明，并按下述人工选择规则处理。宿主回传时再次核验发送事实和当前可回传性，不能仅凭模型选择放行。

人工明确选择一条发送结果不明的投递，可以转交针对该记录的反馈；这不把原发送状态追认为成功。能够证明未进行发送的失败记录不能伪装成对方已经收到的投递。

### 6.3 回传与原会话处理

1. 先持久保存不可变的 DeliveryReply。
2. 用稳定 reply_id 将反馈提交到精确来源 Session 的接收入口。
3. 原会话空闲时启动新的 round；忙碌时使用 queue，避免插入现有工具执行链或打断当前工作。
4. 回传携带原投递的引用和“来自某 IM 联系人，由某智能体转交”的说明，不能显示为 App 本地用户亲自输入。
5. 原智能体按当前会话状态处理。后续结果沿原会话既有展示规则发布；要再发到 IM 时，仍使用正常发送工具。

历史来源 round 已结束不影响回传。重新启动的是原逻辑会话中的新 round，不恢复旧 tool call、旧 runtime lease 或旧 Goal/WorkGraph mutation authority。

### 6.4 持久提交与恢复

SQL 回传记录与现有会话队列不在同一个存储事务中，不能声称一次跨存储原子提交。

采用明确的两段提交：

- SQL 先保存回传意图和可信来源。
- 再以稳定 reply_id 调用已有队列的 EnqueueIdempotent；只有持久接受成功后才向调用方报告已经转交。
- 最后把队列接受定位补记到回传记录；此步失败可由同一回传记录恢复。

现有队列会在 append-only 日志中保存已接受的 client_message_id，即使队列项已经消费，重复入队也能找到旧接受结果。回传使用这一事实，不能依赖内存去重或仅检查当前队列快照。

恢复读取已在 SQL 持久保存的回传意图。尚无队列接受事实的，使用保存时的可信人类请求证明、重新核验当前配对和来源准入后，以同一 reply_id 幂等补交；不要求旧 IM runtime round 仍然存活，也不伪造一次新的工具授权。已有队列接受事实的只补记定位。

恢复不主动外发 IM，不重跑已经结束的模型 round。队列消费与模型开始之间若出现不能证明结果的故障，必须显示需要处理的状态，不能因队列项消失就报告处理完成或盲目重跑。新接收入口必须保留 reply_id 的派发及消息接受定位，以便精确核对这个窗口。

这部分只补本能力所需的入口、身份传递和核对，不重建全局调度系统。

## 7. 不同来源如何返回

| 来源 | 保存和返回规则 |
| --- | --- |
| 普通 Agent Session | 回到当时的 source_session_key，由 source_agent_id 处理 |
| Room-backed DM | 回到真实逻辑 conversation，使用现有 DM/Room 路由，不新建另一个私聊 |
| 本地 Group Room | 保存具体 conversation 和出站时的回传可见性；只唤醒原 Agent，私域内容不能升级成公区消息 |
| Automation 主会话、bound 或 named 执行 | 保存真实执行 Session；反馈作为新的外部输入进入该 Session，不重新执行旧 run |
| Automation isolated 执行 | 保存本次 run 的真实 Session；仍可交互且存在时可接收新输入，已清理或不可交互时明确返回不可回传 |
| 无真实会话的控制面发送 | 可保存来源事实，但标记不可回传，不猜测一个主会话 |
| 来源就是当前 IM Session | 直接使用当前会话正常回复，拒绝回传给自己造成额外唤醒 |

Automation 的创建来源、实际执行会话和结果接收会话分别记录；返回地址使用实际产生这次投递的执行会话，不能把 job.Source.SessionKey 或当前最新 run 当作通用回退。

对自动化来源的新输入，必须通过 Automation 的会话接收策略重新核验任务、Session 和权限约束；不得借回传绕过其仍适用的权限限制。旧 run 的终态、用量、投递次数及调度状态不因此改变。来源不可用时不新建会话、不重新 run。

Room 的返回可见性由宿主在出站时固定。若当前私域能力、成员资格或 conversation 已不可用，拒绝该次回传；不能退到群主会话、主 conversation 或广播全群。

历史版本未保存精确来源的普通发送不自动补齐。现有 Automation 元数据只有在能够逐条验证 owner、run、真实来源与目标时才可兼容导入；禁止根据时间相邻、相同正文或最近活跃会话推测。

## 8. 权限与内容边界

- 查询只能访问当前 IM Session 收到的记录；知道别的 delivery_id 也不能读取。
- 回传先复核发送时记录的目标身份和当前 active pairing；重新绑定到其他 Agent 后不能沿旧记录回传。
- 来源会话删除、Agent 删除或身份失效后，返回明确不可用原因。
- 回传是来源明确的外部输入，不伪造 Web 用户 principal，不继承 TrustedConfigurationContext。
- 队列需要新增可验证的外部回传来源类型及 reply_id 引用；可信身份由宿主查持久记录恢复，用户编辑队列项不能伪造这种输入。
- 来源消息只提供因果关系，不传递旧的 WorkBinding、ReviewBinding 或 Goal mutation capability。
- 本次工具回传正文为文本。附件作为原始入站事实保留；不凭路径复制跨 Session 文件、不新增跨 workspace 附件授权。要回传附件时继续使用另有授权的文件交付能力。
- 不增加任意会话阅读器、任意 Session 发送器或整个 IM 收件箱搜索器。

## 9. 工程职责与文件组织

| 层 | 本次职责 |
| --- | --- |
| protocol | 定义投递来源、回传引用和有来源的输入投影；共享线格式由此生成 |
| storage/communication | 投递记录、回传记录、范围查询、唯一约束和失效标记；SQLite/Postgres 对等迁移 |
| service/channels | 配对、目标会话投影、平台收发及回执；不解释业务批准 |
| service/communication | 来源固化、来源查询、回传校验、接收协调；从现有大文件拆成 external_delivery.go、delivery_query.go、delivery_reply.go 等职责文件 |
| service/dm、service/room/realtime | 原会话的持久接收、带来源展示、排队、启动与派发核对 |
| service/conversation | 向 IM 模型提供尚未进入 transcript 的投递事实，统一普通外投和 Automation 的上下文适配；不判断回复应该转给谁 |
| service/automation | 签发真实 run 来源、保持固定目标与重投事实，提供自动化来源 Session 的新输入准入 |
| mcp/communication | 扩展现有 list_targets 查询范围和 send_message 目标分支；schema、解析和结果映射按职责拆分，不增加工具定义 |
| app/runtime、app/server | 注入当前来源与 IM 入站身份，装配服务接口和恢复入口；wiring 保持独立小文件，不承载回传业务规则 |
| Web 消息展示 | 复用现有消息与队列 UI，补来源说明和转交失败文案 |

外投来源记录应在普通发送和 Automation 结果发送共用的 Channels 编排边界完成；上层分别提供经过验证的来源，平台 adapters 不感知业务 Session。

communication 通过消费侧的小接口调用 DM/Room 接收器。接收器验证回传引用时依赖持久记录的只读接口，由 app 装配；不反向依赖 communication service，避免形成 service 循环依赖。

不新建通讯 MCP server，不把逻辑塞进技能、平台 adapter 或 app wiring。

## 10. 用户看见的行为

投递记录随原消息关联，不新增审批中心或独立任务面板。

例如总管理员从“创建并跟进自动化任务”发来“请确认这份草案，确认后继续下发”，领导在微信直接说：

> T4 增加人力资源部协办，修改后再发我。

IM 智能体查到唯一对应投递后，回传并回复：

> 已将意见转交总管理员的“创建并跟进自动化任务”会话。

原会话出现：

> 来自微信联系人的反馈，由总管理员转交
>
> 关于此前发送的任务草案：T4 增加人力资源部协办，修改后再发我。

原智能体继续处理该反馈。如果它决定重新送审，就按正常发送产生下一次独立投递记录。

若人只回复“确认”，IM 智能体同样可以关联该草案并转交原文“确认”；原 Session 的智能体决定按任务约定继续下发。“已转交”只承诺原会话持久接受；失败时明确说未转交或结果待核对，不用“领导已批准”“任务已完成”等业务结论代替传输状态。

## 11. 验收要求

### 功能与范围

1. 同一 Agent 的两个 App Session 先后向同一 IM 发送不同材料，能分别查到并精确回传。
2. 相同标题、相同正文的两次投递仍有不同的记录，不按正文覆盖。
3. 与投递无关的 IM 聊天不回传；面对要求手机确认的材料，人自然回复“确认”即可由 IM 智能体关联并调用回传工具，不要求固定转交措辞。
4. 多个合理候选时询问用户；明确引用可用时选中相应记录。
5. 原智能体空闲时继续处理，忙碌时排队，不打断原执行。
6. 回传成功显示来源，原消息和反馈各自只出现一次。
7. 同一投递后续可继续补充反馈。
8. Room 回到原 conversation，只唤醒原 Agent，保持可见性。
9. Automation 两个 run 分别关联真实来源；回传不重跑旧 run、不改变旧终态。

### 可靠性与权限

10. 外投前来源保存失败，不调用平台发送；发送后结果不明不自动重发。
11. 重复 IM callback、相同工具调用重试、回传结果 ACK 丢失均不重复转交。
12. 覆盖 SQL 保存后未入队、入队后未补记 SQL、队列消费后启动结果不明三个故障窗口；覆盖 M1 提出反馈、M2 消歧选择后回传和重试的正文溯源及去重。
13. 重启后可查询旧记录，并核对已接受回传；无法确认模型启动结果时不盲目重跑。
14. 跨 owner、其他 IM Session、伪造 delivery_id、失效配对、已删除来源均拒绝。
15. 队列中的回传身份不能被普通用户输入伪造；自动唤醒不获得旧权限或后台配置权限。
16. 历史来源不足时说明不可定位，不猜 Session。
17. 当前 IM 回传给自己和无人类入站来源的自动回传被拒绝。
18. 普通外投尚未进入 IM 的 SDK transcript 时，模型仍能看到该投递及来源；与 Automation 结果同时出现时不重复注入。
19. MCP 工具数量不因本能力增加；list_targets 无参数行为和 send_message 的所有既有目标行为保持兼容，新范围只返回当前 IM 的投递来源。
20. send_message 回传分支的封闭 schema 与领域去重均生效；传入任意返回地址、借用旧 Goal/Automation authority 或从无人类 IM 输入的 round 调用均不能放行。

### 交付证据

先运行变更包的定向测试、架构检查和所需前端检查；不以全量测试代替具体行为验证。

真实 IM 联调必须演示：App 投递、IM 查询和回传、原 Session 新 round、忙碌排队、重启后查询及去重。模拟平台单测只能证明本地路径，不能作为真实微信链路已通过的证据。

当前此前通过的 7 项定向测试仅验证现有外投、收信、配对和去重基线，本提案新增行为尚无实现或验收结果。

## 12. 实现顺序与文档收口

完整模型按以下依赖顺序落地，每一步保持同一目标：

1. 定义来源与回传协议、存储模型和迁移，建立逐条查询与唯一约束。
2. 接通普通外投与 Automation 结果外投的来源写入及目标消息关联。
3. 通过现有 list_targets/send_message 接通可信 IM 来源查询、显式回传、DM/Room 原会话接收、队列身份与恢复核对。
4. 接通来源展示、模型工具说明、运行检查和真实 IM 联调。

实现时同步更新相关 Go 包的 L2/L3、AGENTS.md 中受影响的通讯边界、CHANGELOG.md，以及当前平台通讯规范；不要将提案直接复制成另一份当前规范。

本文件是完整方案的唯一提案入口。实现完成后，以当前规范保存最终合同，并把本提案标明为已收口的设计记录。
