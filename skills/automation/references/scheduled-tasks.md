# Scheduled task

只在 create、update、delete、run 或 retry_delivery 时读取本文件。所有 mutation 都遵循主 Skill 的 `inspect → plan → apply → verify`，字段只认 current contract。

## create 与 schedule

create 需要 `name`、`instruction`、`schedule`。常用可选决策：

- `context_mode=current|isolated`；默认 isolated，只有任务确实需要沿用当前会话上下文才选择 current。
- `deliver_result` 控制是否回到当前可信会话；有当前会话时通常保持投递。
- 用户要求把结果发到同一 Agent 的另一个外部私聊时，先 inspect `delivery_targets`。可用 `delivery_channel` 过滤，其中个人微信使用 `weixin-personal`，企业微信机器人使用 `wechat`；按返回的 label 选择目标，把其 `session_key` 原样作为 create/update 的 `delivery_session_key`。不要从联系人名称、账号或平台 ID 自行拼装 SessionKey；零结果时说明尚无可用会话，多结果且无法确认联系人时请用户明确选择。
- `permission_mode` 只能是 `default|plan|acceptEdits|bypassPermissions|dontAsk`。省略时复制创建时 Session/Agent 的有效权限和工具 allow/deny；只有用户明确要求并理解风险时才选择更宽模式。
- `overlap_policy=skip|allow`；默认 skip。只有运行互不干扰且用户接受重叠时才 allow。
- `expires_at` 使用 RFC3339；`enabled` 默认 true。

schedule 形状：

- single：`kind=single` + `run_at` RFC3339。
- daily：`kind=daily` + `daily_time=HH:MM`，按需加 weekdays 与 timezone；weekday 只使用 `su|mo|tu|we|th|fr|sa` 或对应三字母值。
- interval：`kind=interval` + 正数 `interval_value` + `interval_unit=seconds|minutes|hours`；省略 unit 表示 seconds。
- cron：`kind=cron` + 标准五段 `expr`，按需加 timezone。不要把六段含秒表达式当成五段 Cron。

提醒、报表、周期自检、持续关注和结果投递都使用 scheduled task。

## update

- 使用 exact `job_id` 或唯一 query；其余字段是局部修改。`instruction` 完整替换，`instruction_append` 追加，两者不能同时出现。
- `expires_at` 与 `clear_expires_at=true` 互斥。空 update 会被拒绝，不用无效变更推进 revision。
- 取消卡住的 active run 时，同时提交 `enabled=false`、`cancel_active_run=true` 和当前 exact `run_id`；不能同时要求 `enabled=true`。run 已变化时重新 inspect/plan。
- 跨 Agent 修改必须在同一 update 中给出 current contract 允许的 execution/delivery Session 意图，并由服务端重新验证；不要只改裸 Agent ID 或路由字符串。

## delete、run 与 retry_delivery

- delete 不可恢复，只在用户明确要求且已核对唯一任务、影响范围和 plan 后 apply。
- run 触发一次新的执行，不改变 schedule，也不自动启用已停用任务。执行失败、需要重新计算结果时使用它。
- retry_delivery 需要失败投递的 exact `run_id`，只重投递已存在结果。结果已经产生而仅发送失败时用它，不要重新 run。

主智能体 Nexus 私有 DM 只有在 contract 返回 `cross_agent_allowed=true` 时，才可使用其中列出的高级 Agent/Session 字段；目标必须是当前 owner 下已存在且服务端可验证的真实 Session。
