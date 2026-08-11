# 定时自动化权限规范

定时自动化运行时没有绑定用户轮次，临时 session prompt 不能作为授权来源。持久授权决定归 Nexus，session 只承载执行路由和历史记录。

## 权限边界

- 创建任务时，当前 Agent 的 `AllowedTools` 列表用于生成初始授权。只要当前 Agent 仍允许该工具，这份快照产生的授权才继续有效。
- 当前 Agent 的 `DisallowedTools` 列表属于硬拒绝，任务级批准不能覆盖它。
- owner 可以明确批准同一 run 的精确输入，也可以批准当前任务范围内的能力。任务授权可以绑定 Connector、effect 和 resource scope。
- Connector OAuth 可用性与能力授权分别检查。任务授权有效不代表 token 仍可使用。
- session key 和 round ID 只用于路由与审计，不能证明调用者拥有权限。

## 持久化模型

每个任务拥有一份带版本的 `TaskPermissionPolicy` 和一个权限状态。每次 run 都记录启动时使用的策略修订、阻塞状态、造成阻塞的请求，以及外部或 workspace 副作用是否已经开始。每次用户交互对应一条 owner-scoped 的 `AutomationPermissionRequest`。客户端必须提交界面实际展示的 job、run、request 和策略修订。只有该请求仍是任务当前交互，并且对应 run 仍被它阻塞时，存储层才会接受处理结果。

修改 Agent、指令、执行类型或 session target 会推进策略修订。待处理请求随即失效，使用旧修订的阻塞 run 会被取消，已有批准不能越过已经变化的执行边界。

## 运行流程

1. run 启动时保存当前任务策略修订。
2. 工具请求会被规范化为一项能力，其中包含 runtime 工具名、Connector、effect、resource scope 和精确输入指纹。
3. 系统依次检查 Agent 硬拒绝、任务授权，以及已经批准的单次 run 授权。
4. 缺少授权时，系统持久化请求，把任务和 run 转为阻塞状态，并释放 scheduler runtime claim，不增加连续失败次数。当前物理 attempt 正在中断时，来自该 attempt 的后续工具请求全部拒绝，已经获得任务授权的工具也不例外。
5. `allow_once` 只批准同一 logical run 的精确输入指纹。`allow_task` 写入带作用域的任务授权并推进策略修订。`deny` 结束本次 run。
6. Connector 工具通过能力授权后，还要检查当前连接。连接不可用时创建独立的重新授权请求。
7. 非只读工具执行前先写入 `effect_started`。尚未产生副作用时，批准可以自动恢复 run；已经产生副作用时，run 进入 `ready_to_retry`，等待 owner 明确确认。

每次恢复沿用原 `run_id`，并启动新的 attempt。恢复指令会写明此前被阻塞的工具，旧拒绝不能继续作为本轮结论。任务权限处理器只有观察到同一工具被重新请求并获准后，才允许恢复后的 attempt 以成功结束。模型只在文本里声称成功，却没有重新调用工具时，本次 attempt 记为失败。这样可以保留完整重试审计，同时不会把恢复过程记录成一次新的定时触发。

## Main Session 对齐

Main Session 任务在宿主持有的 system event 中保存 `job_id`、`run_id`、owner 和策略修订。权限恢复还会携带已经处理的 request ID，heartbeat 据此重建精确的重试指令和验证边界。heartbeat 每次只消费一个绑定任务的事件，并使用该任务自己的权限处理器派发原任务。普通 heartbeat 工作继续使用 Agent 默认权限处理器。批准恢复 Main Session run 时，系统重新入队同一个 logical run，不派发没有任务身份的 heartbeat 指令。

## 脚本边界

脚本权限与精确内容哈希、owner 和 Agent 绑定。脚本任务只允许人类控制面管理，Agent 对话不能创建、编辑、删除、运行、重试或恢复脚本任务。用户直接创建或编辑脚本时会确认精确内容，脚本内容变化后原授权立即失效。

缺少权限快照的存量任务会在首次读取或执行时，按照当前 Agent 默认设置初始化。存量脚本任务会获得绑定内容哈希的兼容授权，在保持原有执行行为的同时继续服从人类控制面边界。

## 用户交互 API

- `GET /capability/scheduled/permission-requests?status=actionable`
- `POST /capability/scheduled/permission-requests/{request_id}/decision`
- `POST /capability/scheduled/tasks/{job_id}/runs/{run_id}/permission/resume`

定时任务面板只关联仍处于阻塞状态或明确等待重试的 run 请求。界面可以处理工具批准、Connector 跳转与复查、任务输入编辑、拒绝和明确重试。权限状态是首要待处理原因，Provider 或投递失败只作为附加诊断显示。controller 负责全部 API 调用，并在后台刷新前提交服务端返回的权威任务结果。
