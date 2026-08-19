# Automation CLI 操作

先运行：

```bash
"${NEXUS_COMMAND_PATH}" --json automation contract
```

本文件与当前 CLI contract 不一致时，以 CLI 为准。
只有 contract 返回 `cross_agent_allowed=true` 时才能使用跨 Agent/Session 高级输入。

contract 同时返回宿主为当前 physical round 预建的 `input_staging.path`，初始内容为 `{}`。每次新的 input 写入前都重新运行 contract，并且只使用刚返回的路径；不要复用记忆、旧输出或上一轮中的绝对路径。某个新返回路径第一次写入前先用 Read 工具读一次，再用 Write 覆盖为单个 JSON 对象。然后在 Bash 单行命令中使用 `--input-file "${NEXUS_COMMAND_INPUT_PATH}"`；Windows PowerShell 对应使用 `& "${env:NEXUS_COMMAND_PATH}" ... --input-file "${env:NEXUS_COMMAND_INPUT_PATH}"`。CLI 未显式传 `--input` / `--input-file` 时也默认读取这个槽。`--input` 与 `--input-file` 互斥，文件上限 1 MiB，`-` 只用于人工 stdin。不要用 `$(cat ...)`、heredoc、重定向或多行 shell 传 JSON。

## 查询

统一形式：

```bash
"${NEXUS_COMMAND_PATH}" --json automation inspect --operation OPERATION --input-file "${NEXUS_COMMAND_INPUT_PATH}"
```

- `list`：`query`、`agent_id`、`include_active`、`include_deleted`、`enabled`、`limit`。
- `get`：`job_id` 或可唯一定位的 `query`；可选 `run_limit`、`event_limit`。
- `runs` / `events`：`job_id` 或唯一 `query`；支持已删除任务历史。
- `report`：可选 `date`、`timezone`、`agent_id`、`job_id` 或唯一 `query`。
- `heartbeat`：可选 `agent_id`；外部 IM round 不开放。

后台 scheduled run 省略 `job_id` 时自动使用宿主绑定的当前任务；不能改查其他任务。

## 变更

每项变更先 `plan`，再保持受管文件不变并用完全相同的 operation/input `apply`。apply 必须携带 plan 的 `current_revision` 和稳定 `request_id`。

### create

必填：`name`、`instruction`、`schedule`。可选：

- `context_mode`: `isolated`（默认）或 `current`。
- `deliver_result`: 是否回到当前可信会话；有当前会话时默认 true。
- `permission_mode`: `default|plan|acceptEdits|bypassPermissions|dontAsk`。省略时复制创建时 Session/Agent 的有效权限和工具 allow/deny；只有用户明确要求才用 `bypassPermissions`。
- `overlap_policy`: `skip`（默认）或 `allow`。
- `expires_at`: RFC3339。
- `enabled`: 默认 true。

主智能体私有 DM 的跨 Agent/Session 高级输入只有 contract 允许时可用：`agent_id`、`execution_mode`、`reply_mode`、`selected_session_key`、`named_session_key`、`selected_reply_session_key`、`reply_session_key`。目标必须是当前 owner 下真实且服务端可验证的 Agent/Session。

### schedule

```json
{"kind":"single","run_at":"2026-08-18T09:00:00+08:00"}
```

```json
{"kind":"daily","daily_time":"09:00","weekdays":["mo","tu","we","th","fr"],"timezone":"Asia/Shanghai"}
```

```json
{"kind":"interval","interval_value":30,"interval_unit":"minutes"}
```

```json
{"kind":"cron","expr":"0 9 * * 1-5","timezone":"Asia/Shanghai"}
```

Cron 只接受能回写 Nexus UI 的标准五段日/周表达式：minute/hour 为单个整数，day-of-month 和 month 都为 `*`。

### update

提供 `job_id` 或唯一 `query`，其余字段为局部修改：

- `name`、`schedule`、`permission_mode`、`context_mode`、`deliver_result`、`overlap_policy`、`enabled`。
- `instruction` 完整替换；`instruction_append` 追加。两者不能同时出现。
- `expires_at` 设置截止；`clear_expires_at=true` 清除。两者不能同时出现。
- `cancel_active_run=true` 会隐含停用；同时传当前 `run_id` 防止取消错误运行。当前没有 active run、run_id 已变化或同时传 `enabled=true` 都会在 plan 阶段拒绝。
- 空 update、脱离 `cancel_active_run=true` 的 `run_id` 都会拒绝，不会只推进配置版本。
- 跨 Agent 修改必须在同一 update 中提供新的 execution/delivery Session 意图，并重新验证权限与投递 grant。

### delete / run / retry_delivery

- `delete`：`job_id` 或唯一 `query`。
- `run`：`job_id` 或唯一 `query`；只触发一次，不改变排程，也不自动启用停用任务。
- `retry_delivery`：`job_id` 或唯一 `query`，以及失败投递的精确 `run_id`；只重投递结果。

### set_heartbeat / wake

- `set_heartbeat`：可选 `agent_id`、`enabled`、`every_seconds`、`target_mode=none|last`、`ack_max_chars`。
- `wake`：可选 `agent_id`、`mode=now|next-heartbeat`、`text`。

Heartbeat 是宽松周期机制。需要保证用户在准确时间看到结果时创建 scheduled task。
