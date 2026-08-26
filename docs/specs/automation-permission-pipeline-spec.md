# 定时自动化权限与投递规范

定时自动化运行时没有绑定用户轮次，临时 session prompt 不能作为授权或投递来源。Nexus 必须分别持久化任务来源、执行上下文、权限快照和结果投递目标；run ledger 记录每次执行实际采用的快照与结果。

## 结构模型

一个定时任务包含五个彼此独立的边界：

1. `Source`：谁在什么可信会话中创建了任务，只用于 provenance、审计和当前会话查询；创建后不可被页面编辑覆盖，也不决定执行或投递。
2. `SessionTarget`：任务在哪个 Agent runtime 上执行，可复用已有会话，也可每次使用独立上下文。
3. `PermissionMode + TaskPermissionPolicy`：创建时从执行 Session/Agent 复制，随后归任务独立所有。
4. `DeliveryTarget`：完成结果是否投递，以及投递到哪个真实、稳定的 Nexus/Room/IM Session；Room 目标还保存与执行 Agent 独立的结果回复/署名 Agent。外部平台临时 callback 和合成“收件箱”都不属于新任务定义。
5. `ScheduledTaskRun`：保存本次执行使用的权限 revision、冻结的首次投递目标、执行结果和平台投递状态。

`DeliveryGrant` 是不对 HTTP/CLI 模型输入暴露的宿主授权快照：它记录最近一次明确配置投递目标时的可信 Agent/页面/CLI 调用方。旧任务升级时从 `Source` 精确复制一次；以后修改投递只替换 grant，不改写 `Source` provenance。

页面控制面可以把现有任务重新绑定到同 owner 的另一个 Agent，但必须在一次配置更新中同时提交新的 `AgentID` 及其执行/投递 Session。后端以新 Agent 重新验证 Session 所属关系、Room 成员关系和 IM active pairing，并推进权限 revision；`Source` 仍保留最初创建 provenance，不能用它回显或覆盖当前任务 Agent。

任务正文不携带 `job_id`、IM 地址或审批状态。后台 runtime 使用宿主签发的隐藏 `AutomationRunContext` 获取当前 `job_id/run_id`；round-scoped Nexus command capability 直接由该可信结构收窄到当前任务且只允许 inspect，不从模型文本、session 名称或模型参数推断身份。

一次性任务完成或错过最终窗口后只自动停用，不自动删除。任务、run 和事件继续保留，因此消息中出现的任务身份始终可以通过任务或历史工具核对。

## Session 绑定生命周期

任务的创建来源 Session 只用于 provenance，不属于执行依赖。执行复用的 Session 和结果投递 Session 才是持久绑定：单个 Session 被删除时，引用它的任务保留原配置，但立即停用并持久化为 `rebind_required`；新调度、手动运行、投递重试和仍在进行中的最终投递都必须 fail closed。用户可以分别替换执行或投递绑定，任务只有在所有已失效绑定都被替换后才能重新启用。

删除 Agent 或 Room 仍按父资源生命周期级联删除任务。Session tombstone 恢复必须重放任务绑定失效操作，使进程在 Session 主记录已经提交、任务尚未来得及停用时崩溃，也不会留下继续执行的悬空任务。

## 权限边界

- 创建任务时，若执行目标明确复用 Session，则优先复制该执行 Session 的有效 permission mode；否则复制执行 Agent mode。同时复制执行 Agent 的 `AllowedTools` 与 `DisallowedTools`。创建来源 Session 不参与权限继承。
- 复制完成后，任务拥有独立权限副本。修改任务不回写 Agent，Agent 或 Session 后续变化也不隐式改写已有任务。
- 任务快照中的 deny 是硬拒绝，任务级批准不能覆盖。
- owner 可以批准同一 run 的精确输入，也可以批准当前任务范围内的能力。任务授权可以绑定 Connector、effect 和 resource scope。
- Connector OAuth 可用性与能力授权分别检查。任务授权有效不代表 token 仍可使用。
- session key、round ID 和平台回执只用于路由与审计，不能单独证明调用者拥有权限。
- IM 创建、投递、重试和审批都必须重新验证完全匹配的 active pairing；配对停用、拒绝、解绑或改绑后立即失效。

## Runtime permission mode

Agent 任务持久化一个具体 SDK permission mode：`default`、`plan`、`acceptEdits`、`bypassPermissions` 或 `dontAsk`。UI 的“复制当前权限”和 Automation command 省略 `permission_mode` 都执行上述复制语义；用户也可以在创建时或之后选择具体模式。

每次 DM 或 Room 派发都同时传入任务保存的 mode 与工具策略快照。`bypassPermissions` 会明确跳过 SDK 权限检查，只能由用户主动选择。SDK 仍产生额外权限请求时，下面的 Automation 持久审批流水线继续作为决定来源。

## 持久审批模型

每个任务拥有一份带版本的 `TaskPermissionPolicy` 和一个权限状态。每次 run 都记录启动时使用的策略修订、阻塞状态、造成阻塞的请求，以及外部或 workspace 副作用是否已经开始。每次用户交互对应一条 owner-scoped 的 `AutomationPermissionRequest`。

当任务配置了结果投递 Session 时，工具或存量脚本审批还必须把同一持久请求投影到该接收 Session 的 DM/Room Composer，而不是只显示在能力面板。接收 Session 只是通知与交互路由，不成为任务或请求的授权所有者；服务端仍按 owner、精确 request/job/run/policy revision 和请求冻结的 `delivery_session_key` 校验决策。浏览器重新绑定 Session 时从持久请求重放，Room 级订阅同时恢复待确认标记。允许操作只映射为 `allow_once` 或当前任务的 `allow_task`，拒绝映射为 `deny`；请求解决后向同一 Session 投影 resolved 事件以清除交互面。

客户端必须提交界面实际展示的 job、run、request 和策略修订。只有该请求仍是任务当前交互，并且对应 run 仍被它阻塞时，存储层才接受处理结果。修改 Agent、指令、执行类型或 session target 会推进策略修订；待处理请求随即失效，旧修订的阻塞 run 被取消，已有批准不能越过变化后的执行边界。

## 运行流程

1. run 启动时保存当前任务策略修订，并冻结本次首次投递目标。
2. 工具请求被规范化为一项能力，其中包含 runtime 工具名、Connector、effect、resource scope 和精确输入指纹。
3. 系统依次检查任务快照中的硬拒绝、任务授权，以及已经批准的单次 run 授权。
4. 缺少授权时，系统持久化请求，把任务和 run 转为阻塞状态，并释放 scheduler runtime claim，不增加连续失败次数。当前物理 attempt 正在中断时，来自该 attempt 的后续工具请求全部拒绝。
5. `allow_once` 只批准同一 logical run 的精确输入指纹。`allow_task` 写入带作用域的任务授权并推进策略修订。`deny` 结束本次 run。
6. Connector 工具通过能力授权后，还要检查当前连接。连接不可用时创建独立的重新授权请求。
7. 非只读工具执行前先写入 `effect_started`。尚未产生副作用时，批准可以自动恢复 run；已经产生副作用时，run 进入 `ready_to_retry`，等待 owner 明确确认。

每次恢复沿用原 `run_id`，并启动新的 attempt。原任务指令保持不变；宿主签发的隐藏 `AutomationRunContext` 指明此前被阻塞的工具。任务权限处理器只有观察到同一工具被重新请求并获准后，才允许恢复后的 attempt 以成功结束。模型只在文本里声称成功、没有重新调用工具时，本次 attempt 记为失败。

## Main Session 对齐

Main Session 任务在宿主持有的 system event 中保存 `job_id`、`run_id`、owner 和策略修订。权限恢复还会携带已处理的 request ID。heartbeat 每次只消费一个绑定任务的事件，并使用该任务自己的权限处理器派发原任务；普通 heartbeat 工作继续使用 Agent 默认权限处理器。批准恢复 Main Session run 时，系统重新入队同一个 logical run，不派发匿名 heartbeat 指令。

## 脚本边界

新建和编辑界面不再暴露独立的 `script` 任务类型；它让用户误以为指令文本会被保存为 `.sh` 并由 shell 执行。用户需要定时运行脚本时，仍创建普通 Agent 任务，由 Agent 在自己的 workspace 中通过常规工具编写或调用 `.sh`；整个过程继续服从任务权限快照、工具 allow/deny 与审批流水线。

存量 `script` 任务只保留兼容读取和原执行行为，可启停、立即运行、查看历史或删除，但不能通过页面或 Agent 对话新建、编辑。缺少权限快照的存量任务在首次读取或执行时按当前 Agent 默认设置初始化，既有脚本授权继续绑定精确内容哈希、owner 和 Agent。

## Agent Automation MCP

所有 Agent 通过内置 `automation` Skill 调用 physical-round scoped `nexus.automation_read|plan|apply`。宿主把 owner、Agent、DM/Room/IM Session、source、DeliveryGrant 与可选 job/run 固定在进程内 MCP server 实例中；模型直接提交 operation 与其 closed 业务字段，apply 另带 `expected_revision/plan_digest`，不能覆盖 identity 或 request ID。

业务参数直接作为 MCP tool input 的 closed object 传入，沿现有 SDK `stream-json` 通道到达宿主并复用同一领域 parser、schema 校验、权限确认与 typed receipt。MCP schema 自身替代 contract 调用和通用 command envelope。Nexus 不创建 `runtime-command-inputs/.../input.json`，不注入命令 broker/token/path，不授予临时可写目录，也不为 transport 启动 shell 或 loopback HTTP 请求。每轮 server 实例由 bridge 原地替换，因此 actor/authority 更新不会要求 runtime 进程重启。

当前三个 Nexus Automation 工具由受管工具策略自动审批进入领域处理；最终写权限仍由 round actor、Automation service、revision/digest 栅栏和原生真人确认共同决定。后台 run 只暴露宿主绑定 job/run 的 `automation_read`，mutation 工具不进入其 surface。

查询使用 `automation_read`；所有变更固定使用 `automation_plan -> automation_apply`。plan 不写入并返回 target、risk、current revision 与 plan digest；apply 在 service 内重新 plan，要求完全相同的 revision/digest，并通过当前 Nexus/Room/IM Session 的 runtime permission context 取得真人 allow 后才写入。确认载荷必须投影规范化变更字段，不能只显示泛化标题。真正写入继续使用 plan 观察到的 configuration version；`cancel_active_run` 还把 plan 观察到的 run_id 放进同一条件更新，过期确认必须在任何配置字段落库前失败。模型伪造的确认字段、聊天正文同意或通用工具 allow 都不能替代这次领域确认。

每个 apply 使用 Bridge 从 provider MCP `params._meta` 透传的真实 tool-use ID 作为 owner-scoped 稳定 `request_id`，模型不生成该字段。durable command ledger 把它绑定到 Actor、operation、不含瞬时 revision 的 intent digest，以及当前 Session permission context 生成的真人 approval request ID：相同 tool use 重放返回首次结果，不同意图冲突；命令已经开始但进程未能持久化结果时进入 uncertain 并禁止自动重放，调用方必须读取权威状态后以新 tool use 处理，不能冒险重复 run、wake 或外部投递。

普通交互 runtime 的 create/update 只表达：

- `context_mode=current|isolated`
- `deliver_result=true|false`
- 可选 `permission_mode`

在 active-paired IM 私聊中，`current + deliver_result=true` 表示可信当前 IM Session。普通 tool schema 不暴露 channel、account、target 或 thread 等宿主路由参数；严格结构化解码拒绝未知字段，service 再按 round actor 自动绑定真实路由。只有主智能体自己的 Nexus WebSocket 私有 DM authority 可以使用跨 Agent/Session 高级字段，且目标仍必须是当前 owner 下已存在、可验证的真实 Session，不能创建合成 Agent 收件箱。后台 scheduled run authority 固定为只读并绑定 exact job/run。

list/get 的“这里、当前群、当前会话”等词按 Actor 的结构化 SessionKey 匹配任务 source、bound execution Session 和 delivery Session。active-paired 外部 IM 的空 list/report 默认也只覆盖当前会话，零匹配返回空而不能回退到 Agent 全量；外部 IM 不开放 heartbeat 配置查询。已删除任务的 runs/events 仍接受精确 job_id；`enabled` 和会话过滤必须在用户 limit 之前执行。

## IM transport 与审批

Discord、Telegram、DingTalk、WeCom、个人微信和飞书使用同一 channel-neutral 边界。完全匹配 active pairing 的私聊是同一个 Agent 的另一种 transport，因此加载同一 Agent 的 Skill、当前 permission mode、工具 allow/deny，并可使用同 Agent Automation command mutation。pairing 只证明 transport 身份，不构成第二套工具权限系统。群聊、失效配对及跨 Agent/owner 操作继续 fail closed。

普通 Agent runtime 的临时权限请求与 Automation 持久请求仍是两种状态，但复用同一套 IM 控制命令：

- `/y`：只允许本次
- `/a`：在当前请求支持持久建议时持续允许
- `/d`：拒绝

历史 `/approve`、`/always`、`/deny` 仅作为兼容输入。内部 request ID 不展示给用户。Ingress 在执行无 ID 命令前，必须合计当前 session 的普通 runtime 与 Automation pending 请求；总数恰好为一才可执行，多个请求一律 fail closed。命令由控制面消费，不进入 Agent 对话。

Automation 权限、重新连接、缺少输入、拒绝、恢复失败和投递 dead letter 等控制通知可以显示任务身份。普通完成结果不能被强制拼接任务前缀或后缀；Web UI 只根据结构化 metadata 显示轻量“定时任务”标识。

权限请求一旦与 run 阻塞状态原子持久化，`permission_requested` 审计、Nexus Session 事件和外部 IM 控制通知就是已受理副作用。中断当前 physical attempt 后，这三项必须在保留 owner principal 的有界 `context.WithoutCancel` 上完成；物理 round 的 cancellation 不能撤销已持久化请求的通知。DeliveryGrant 仍在 detached context 中实时复验，真实失效与 context cancellation 必须使用不同诊断。

每条持久权限请求在创建时冻结唯一审批 Session：优先使用该 run 开始时固化的结果接收 Session；没有结果接收 Session 时才使用任务来源 Session；两者都不存在时只保留 Automation 看板审批。后续实时通知、WebSocket 重放、Composer 决策和 IM Slash 都只消费请求中冻结的 SessionKey，不能按任务最新配置重新猜路由。有效的 Nexus Agent、Room 与 active-paired 外部 IM Session 都必须收到同一 `permission_request` 投影；外部 IM 另外发送 `/y`、`/a`、`/d` transport 通知。浏览器在外部 IM Session 上处理该持久卡片只解析 exact request，不注入聊天 Slash，也不得改写外部投递 route。

Nexus 内部 DM/Room 不使用 IM Slash 命令承载 Automation 审批。工具与存量脚本请求复用 Composer 权限确认面，直接提供“允许本次”“此任务始终允许”和“拒绝”；Connector 重新连接与缺少执行输入仍由能力面板完成相应配置动作。

## IM 结果投递

外部 IM 任务只持久化结构化 `delivery_session_key` 和可长期复用的 route。投递前重新校验 active pairing，再由 route 解析当前 channel、account、recipient/chat 和可用上下文。

执行 Agent 与接收 Session 相互独立：同 owner 的 Agent A 可以执行，结果投影进 Agent B 已存在的真实 Nexus/IM Session；逻辑会话和 workspace 归接收 Agent B，消息 metadata 另外保留 producer Agent A。新建或改绑必须提供结构化、真实存在的 Session，不能只保存裸 channel/chat ID，也不得合成“定时任务收件箱”。历史裸路由与收件箱目标仅作为旧数据读取/投递兼容；打开编辑时必须重新选择真实 Session，且它们不出现在页面或 MCP 候选中。

“真实存在”以对外统一 Session 读模型为准：SQL 拥有的 Room-backed DM/成员 Session 与 workspace 拥有的 Nexus/IM Session 都是合法候选。页面按身份而不是存储位置分类，并对执行与投递使用同一层级：DM 是 `Agent -> chat_type=dm Session`，Room 是 `room_type=room -> 共享 room:group:<conversation_id> Session -> 该 Session 的有效成员 Agent`。Room 不选成员时由服务端在保存阶段解析当前房主；执行 Agent 与结果回复 Agent 分别保存、分别授权。Room-backed DM 即使带 `room_id` 也不能被排除，`chat_type=group` 成员 Session 也不能混入 Agent/DM 候选。服务端不得因为 Session 尚未生成 workspace meta 就判定不存在。首次投递可在统一读模型验证 owner、Agent 和精确 session key 后物化 workspace 投影；任意伪造 key 仍必须 fail closed。

- 第一次投递使用 run 启动时冻结的目标，避免运行中无关编辑把结果重定向。
- 首次投递失败后，用户明确修复任务 route，重试使用任务最新目标。
- 投递外部平台前，结果先按 `run_id` 幂等投影到目标 Nexus 会话；成功后关联平台回执，下一轮对话也可获得该 assistant 结果作为历史上下文。
- 个人微信只保留稳定 `context_token`。明确 token 过期时清除并无 token 重试一次；限流与鉴权错误不得误判为过期。
- WeCom callback `req_id` 与 stream ID 只属于当前即时回复。延迟任务使用 `aibot_send_msg` 和新的 request ID，不能把 callback 身份持久化进任务。

## 用户交互 API

- `GET /capability/scheduled/permission-requests?status=actionable`
- `POST /capability/scheduled/permission-requests/{request_id}/decision`
- `POST /capability/scheduled/tasks/{job_id}/runs/{run_id}/permission/resume`

定时任务面板只关联仍处于阻塞状态或明确等待重试的 run 请求。界面可以处理工具批准、Connector 跳转与复查、任务输入编辑、拒绝和明确重试。权限状态是首要待处理原因，Provider 或投递失败只作为附加诊断显示。controller 负责全部 API 调用，并在后台刷新前提交服务端返回的权威任务结果。
