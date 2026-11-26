# 主智能体规范

主智能体是每个 owner 的默认控制面 Agent，不是全局唯一的系统账号。

## 1. 身份与归属

- 身份真相源是当前 owner 下激活的 `is_main` Agent 记录；服务端初始化时保证该记录及其 workspace 可用。
- `is_main` 只能由服务端持久化记录和可信运行时上下文产生。
- 系统 owner 可沿用部署配置的默认 Agent ID；其他 owner 使用稳定的 owner-scoped Agent ID。业务代码不得依赖具体字符串。

## 2. Workspace 与运行时

主智能体与普通 Agent 使用同一套 owner 隔离布局：

```text
<users_root>/<owner_segment>/workspace/<agent_id>/
```

- 主智能体保留宿主控制面身份，但只能操作当前 owner scope，不获得跨 owner 的隐式管理权限。
- 普通 Agent 不因与主智能体共享 provider、模型或 Skill 而获得控制面权限。

## 3. 入口与 Room 边界

- 默认入口使用标准 Agent DM session，不新增专用会话协议；其运行时、历史和恢复规则与其他 Agent DM 一致。
- 主智能体 DM 是 owner 唯一的主窗口对话：服务端拒绝为其新建或分支 conversation（HTTP 409），`EnsureDirectRoom` 始终返回同一条；早期遗留的其他 conversation 只保留数据、不再作为入口。
- 侧栏固定的主智能体直接进入该主窗口；Header 和窄窗不提供会话标签、历史、新建或分支入口。
- 主窗口的用户输入统一追加到该 Session 的 inbox（`agent_inbox_events`，按 `seq` 有序，按 `client_message_id` 幂等），WebSocket `chat` 与 `input_queue` enqueue 等价：
  - 空闲时立即起一轮；积压的 pending 输入合并成一轮（正文以空行拼接、附件按序合并），斜杠命令独占一轮，不同真人身份的输入不合并。
  - 运行中在 PostToolUse 安全点把斜杠命令之前的全部 pending 输入注入当前轮；runtime 确认 applied 后才标记 consumed，并作为 durable 用户消息写入当前轮。最后一个安全点之后到达的输入在本轮结束后立即开下一轮；用户主动停止的轮次结束后不自动开跑。
  - 只有 pending 输入可以撤回；已取走注入或已开跑的输入拒绝撤回。主窗口不提供排队/引导选择与重排。
  - 状态只经条件更新迁移（pending → delivering → consumed，pending → withdrawn，宿主事件另有 → cancelled），不依赖内存登记；进程启动时 delivering 退回 pending。
  - 事件可带 `available_at`：未到点的事件对取走、合并与注入都不可见。dm service 的 inbox 定时器（duework）在启动时立即 reconcile，之后按全部 inbox 最近的 `available_at` 唤醒到点且有 pending 的主窗口；写入或取消未来事件后主动通知。
  - 宿主也可代写事件进入同一 inbox：目标为 main 的定时任务到点时写入 `scheduled_task` 事件（payload 带 job/run/任务名，按「任务 + 计划时刻」或 run 去重）。宿主事件空闲时独占一轮、不与用户原话合并，运行中与用户输入一样在安全点注入；它沿用主窗口会话的权限，不进入待发送列表、不可撤回。
  - 模型看到带来源包装的指令（`<scheduled-task name=…>`，注明不是用户此刻输入）；时间线把该行（`metadata.input_kind=scheduled_task`，`is_synthetic`）渲染为「定时任务 · 名称」分隔条，而非用户气泡，也不计为用户输入。
  - **跟进（follow_up）**：主智能体在主窗口轮次里经 `nexus.command` 的 `follow_up` domain 给未来的自己留一次性跟进——`list_follow_ups`（inspect）、`schedule_follow_up`（`after_minutes` 或 RFC3339 `at` 二选一，加必填 `note`）、`cancel_follow_up`（`follow_up_id`）。
    - 只对可信主智能体主窗口 round 开放，目标会话由宿主 Actor 固定；到点时间 5 分钟到 7 天，每个主窗口至多 10 条待触发；同一 `request_id` 的重试只留一条。用户要的提醒与周期工作仍是定时任务。
    - 跟进是 `follow_up` 宿主事件，到点后与定时任务同路：空闲独占一轮，运行中在安全点注入；模型看到 `<follow-up id=… set_at=…>` 包装的备注，时间线以「跟进 · 备注摘要」分隔条呈现（`metadata.input_kind=follow_up`）。
    - 存在待触发跟进时，宿主把 `<pending_follow_ups>` 作为上下文告知注入（变化时追加，compact 后重新告知），模型据此避免重复设定。
  - **Echo**：主动跟进只在主窗口生效，是宿主写入的 `echo` 事件，规则见 [echo-spec](echo-spec.md)。它只在主窗口空闲时起隐藏的无工具轮，从不在安全点注入；任何用户输入都会取消待定的 echo 事件。
  - 宿主内部的恢复输入继续走原输入队列，在 inbox 之后处理；其他 Agent 的 DM 与 Room 不受影响。
- 主窗口是长期单会话，上下文压缩与其他 Agent DM 共用 runtime 机制，宿主不另设压缩流程：
  - runtime 在每次模型请求前按自身窗口与阈值自动压缩（Nexus 通过 `NEXUS_AUTOCOMPACT_PCT_OVERRIDE` 设为 70%），不更换会话 key、不打断当前轮；手动 `/compact` 作为斜杠命令独占一轮，走同一路径。
  - 长期记忆由 runtime 在每轮结束后提取，受用户「自动记忆」偏好约束；宿主不在压缩前另起落盘轮，避免绕过该偏好。
  - 压缩边界之后宿主清空已告知记录，下一轮重新告知 connector 等宿主事实。
  - 时间线只显示一条「已整理较早的对话」标记，有摘要时可展开：实时取边界自带的 `summary`（nxs），历史取 transcript 中紧随边界的 `isCompactSummary` 条目（两种 runtime 一致），该条目本身不展示、不成为用户轮次。
- `/nexus/v1/runtime/options` 返回当前 owner 的 `default_agent_id`、头像和默认模型偏好；前端只消费该结果，不维护第二份默认 ID。
- 主智能体不能作为多人 Room 的普通成员，也不充当其成员模板。

## 4. 平台能力

- 默认加载普通基础 Skill，并额外加载 `nexus-manager`。
- 需要主智能体权限的 MCP、配置、通道或 Connector 授权能力，必须同时校验当前 owner、Agent 记录、会话类型和可信 `is_main` 上下文，不能只检查工具是否出现在 runtime 中。
- 职责：默认入口对话；当前 owner 范围内的 Nexus 配置与组织动作；需要可信控制面身份的平台能力。
- 不负责代替专业 Agent 执行具体协作。

## 5. 禁止项

- 通过 `"nexus"`、`"main"`、名称或头像判断主智能体。
- 由前端、模型参数、Agent 名称或展示文案设置 `is_main` 或把普通 Agent 提升为主智能体。
- 使用部署级默认 Agent ID 代替当前 owner 的主智能体查询。
- 把主智能体控制面能力公开成所有 Agent 都可调用的普通能力。
