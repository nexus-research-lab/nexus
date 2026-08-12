# 定时自动化权限与投递规范

定时自动化运行时没有绑定用户轮次，临时 session prompt 不能作为授权或投递来源。Nexus 必须分别持久化任务来源、执行上下文、权限快照和结果投递目标；run ledger 记录每次执行实际采用的快照与结果。

## 结构模型

一个定时任务包含五个彼此独立的边界：

1. `Source`：谁在什么可信会话中创建了任务，只用于权限继承、审计和当前会话查询；创建后不可被页面编辑覆盖。
2. `SessionTarget`：任务在哪个 Agent runtime 上执行，可复用已有会话，也可每次使用独立上下文。
3. `PermissionMode + TaskPermissionPolicy`：创建时从来源 Session/Agent 复制，随后归任务独立所有。
4. `DeliveryTarget`：完成结果是否投递，以及投递到哪个稳定 Nexus 会话；外部平台临时 callback 不属于任务定义。
5. `ScheduledTaskRun`：保存本次执行使用的权限 revision、冻结的首次投递目标、执行结果和平台投递状态。

`DeliveryGrant` 是不对 HTTP/MCP 模型暴露的宿主授权快照：它记录最近一次明确配置投递目标时的可信 Agent/页面/CLI 调用方。旧任务升级时从 `Source` 精确复制一次；以后修改投递只替换 grant，不改写 `Source` provenance。

任务正文不携带 `job_id`、IM 地址或审批状态。后台 runtime 使用宿主签发的隐藏 `AutomationRunContext` 获取当前 `job_id/run_id`；Automation MCP 直接由该可信结构收窄到当前任务，不从模型文本、session 名称或模型参数推断身份。

一次性任务完成或错过最终窗口后只自动停用，不自动删除。任务、run 和事件继续保留，因此消息中出现的任务身份始终可以通过任务或历史工具核对。

## Session 绑定生命周期

任务的创建来源 Session 只用于 provenance，不属于执行依赖。执行复用的 Session 和结果投递 Session 才是持久绑定：单个 Session 被删除时，引用它的任务保留原配置，但立即停用并持久化为 `rebind_required`；新调度、手动运行、投递重试和仍在进行中的最终投递都必须 fail closed。用户可以分别替换执行或投递绑定，任务只有在所有已失效绑定都被替换后才能重新启用。

删除 Agent 或 Room 仍按父资源生命周期级联删除任务。Session tombstone 恢复必须重放任务绑定失效操作，使进程在 Session 主记录已经提交、任务尚未来得及停用时崩溃，也不会留下继续执行的悬空任务。

## 权限边界

- 创建任务时，优先复制来源 Session 的有效 permission mode；来源没有覆盖时复制 Agent mode。同时复制 Agent 的 `AllowedTools` 与 `DisallowedTools`。
- 复制完成后，任务拥有独立权限副本。修改任务不回写 Agent，Agent 或 Session 后续变化也不隐式改写已有任务。
- 任务快照中的 deny 是硬拒绝，任务级批准不能覆盖。
- owner 可以批准同一 run 的精确输入，也可以批准当前任务范围内的能力。任务授权可以绑定 Connector、effect 和 resource scope。
- Connector OAuth 可用性与能力授权分别检查。任务授权有效不代表 token 仍可使用。
- session key、round ID 和平台回执只用于路由与审计，不能单独证明调用者拥有权限。
- IM 创建、投递、重试和审批都必须重新验证完全匹配的 active pairing；配对停用、拒绝、解绑或改绑后立即失效。

## Runtime permission mode

Agent 任务持久化一个具体 SDK permission mode：`default`、`plan`、`acceptEdits`、`bypassPermissions` 或 `dontAsk`。UI 的“复制当前权限”和 MCP 省略 `permission_mode` 都执行上述复制语义；用户也可以在创建时或之后选择具体模式。

每次 DM 或 Room 派发都同时传入任务保存的 mode 与工具策略快照。`bypassPermissions` 会明确跳过 SDK 权限检查，只能由用户主动选择。SDK 仍产生额外权限请求时，下面的 Automation 持久审批流水线继续作为决定来源。

## 持久审批模型

每个任务拥有一份带版本的 `TaskPermissionPolicy` 和一个权限状态。每次 run 都记录启动时使用的策略修订、阻塞状态、造成阻塞的请求，以及外部或 workspace 副作用是否已经开始。每次用户交互对应一条 owner-scoped 的 `AutomationPermissionRequest`。

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

脚本权限与精确内容哈希、owner 和 Agent 绑定。脚本任务只允许人类控制面管理，Agent 对话不能创建、编辑、删除、运行、重试或恢复脚本任务。用户直接创建或编辑脚本时确认精确内容，内容变化后原授权立即失效。

缺少权限快照的存量任务会在首次读取或执行时，按照当前 Agent 默认设置初始化。存量脚本任务获得绑定内容哈希的兼容授权，在保持旧执行行为的同时继续服从人类控制面边界。

## 普通 Automation MCP

普通交互 runtime 的 create/update 只暴露：

- `context_mode=current|isolated`
- `deliver_result=true|false`
- 可选 `permission_mode`

在 active-paired IM 私聊中，`current + deliver_result=true` 表示可信当前会话。普通 schema 不暴露 legacy execution/reply 枚举，也不暴露 session、channel、account、target 或 thread 等宿主路由参数。Host 从可信 `ServerContext` 自动绑定；即使旧模型传入这些字段，也不能借此重定向任务。数据库仍保留完整目标模型，供存量任务和 owner main 高级控制兼容。

## IM transport 与审批

Discord、Telegram、DingTalk、WeCom、个人微信和飞书使用同一 channel-neutral 边界。完全匹配 active pairing 的私聊是同一个 Agent 的另一种 transport，因此加载同一 Agent 的 Skill、当前 permission mode、工具 allow/deny，并可使用同 Agent Automation mutation 工具。pairing 只证明 transport 身份，不构成第二套工具权限系统。群聊、失效配对及跨 Agent/owner 操作继续 fail closed。

普通 Agent runtime 的临时权限请求与 Automation 持久请求仍是两种状态，但复用同一套 IM 控制命令：

- `/y`：只允许本次
- `/a`：在当前请求支持持久建议时持续允许
- `/d`：拒绝

历史 `/approve`、`/always`、`/deny` 仅作为兼容输入。内部 request ID 不展示给用户。Ingress 在执行无 ID 命令前，必须合计当前 session 的普通 runtime 与 Automation pending 请求；总数恰好为一才可执行，多个请求一律 fail closed。命令由控制面消费，不进入 Agent 对话。

Automation 权限、重新连接、缺少输入、拒绝、恢复失败和投递 dead letter 等控制通知可以显示任务身份。普通完成结果不能被强制拼接任务前缀或后缀；Web UI 只根据结构化 metadata 显示轻量“定时任务”标识。

## IM 结果投递

外部 IM 任务只持久化结构化 `delivery_session_key` 和可长期复用的 route。投递前重新校验 active pairing，再由 route 解析当前 channel、account、recipient/chat 和可用上下文。

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
