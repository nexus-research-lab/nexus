---
name: scheduled-task-manager
description: 管理智能体与 Room 会话上的定时任务。当需要创建、查看、启停、立即执行、查看运行记录或删除定时任务时，使用此 skill。
---

# scheduled-task-manager

通过 `nexus_automation` MCP 工具管理 Nexus 平台的定时任务。这些工具和前端「新建任务」对话框一一对应——填工具参数等价于在 UI 上点字段。

## 使用原则

- **作用域**：普通 Agent 只能 CRUD 自己 `agent_id` 名下的任务，`list_scheduled_tasks` 也只会返回自己的任务，越权调用会被后端拒绝。主智能体（Nexus）豁免该限制，可指定任意 `agent_id`。
- 创建任务前，像在 UI 上一样明确四件事，缺一就用 AskUserQuestion 问用户：
  1. **目标智能体**：`agent_id`（不传则用当前智能体）
  2. **执行会话**：`execution_mode` = 主会话 / 现有会话 / 临时会话 / 专用长期会话
  3. **调度规则**：`schedule.kind` = 单次 / 每天 / 间隔
  4. **结果回传**：`reply_mode` = 不回传 / 回到执行会话 / 回到指定会话
- 删除或覆盖现有任务前，先 `list_scheduled_tasks` 确认目标 `job_id`。
- 只有短文本、一次一条的提醒/播报类任务，才允许默认按 `execution_mode=temporary` + `reply_mode=none` 创建；否则必须显式确认。

## UI 字段 ↔ 工具参数对照

| UI 字段 | 工具参数 |
|---|---|
| 任务名称 | `name` |
| 目标智能体 | `agent_id` |
| 执行会话 = 使用主会话 | `execution_mode: "main"` |
| 执行会话 = 使用现有会话 | `execution_mode: "existing"` + `selected_session_key` |
| 执行会话 = 每次新建临时会话 | `execution_mode: "temporary"` |
| 执行会话 = 使用专用长期会话 | `execution_mode: "dedicated"` + `named_session_key` |
| 结果回传 = 不回传 | `reply_mode: "none"` |
| 结果回传 = 回到执行会话 | `reply_mode: "execution"` |
| 结果回传 = 回到指定会话 | `reply_mode: "selected"` + `selected_reply_session_key` |
| 调度 = 单次 | `schedule.kind: "single"` + `run_at` |
| 调度 = 每天 | `schedule.kind: "daily"` + `daily_time` (+ `weekdays`) |
| 调度 = 间隔 | `schedule.kind: "interval"` + `interval_value` + `interval_unit` |
| 时区 | `schedule.timezone`（必填，IANA，例 `Asia/Shanghai`） |
| 任务指令 | `instruction` |
| 创建后立即启用任务 | `enabled`（缺省 true） |

向用户描述时用左列 UI 语义，不要把右列的原始字段名甩给用户。

## Schedule 三种模式的参数模板

**单次**（对齐 UI「单次」Tab）：
```json
{
  "kind": "single",
  "run_at": "2026-04-21T18:00",
  "timezone": "Asia/Shanghai"
}
```

**每天**（对齐 UI「每天」Tab，不填 `weekdays` = 每天执行）：
```json
{
  "kind": "daily",
  "daily_time": "09:00",
  "weekdays": ["mon", "tue", "wed", "thu", "fri"],
  "timezone": "Asia/Shanghai"
}
```
`weekdays` 取值：`mon`/`tue`/`wed`/`thu`/`fri`/`sat`/`sun`。

**间隔**（对齐 UI「间隔」Tab）：
```json
{
  "kind": "interval",
  "interval_value": 30,
  "interval_unit": "minutes",
  "timezone": "Asia/Shanghai"
}
```
`interval_unit` 取值：`seconds`/`minutes`/`hours`。

## 可用工具

| 工具 | 用途 |
|---|---|
| `list_scheduled_tasks` | 列出任务（可按 agent_id 过滤） |
| `create_scheduled_task` | 创建任务 |
| `update_scheduled_task` | 按 `job_id` 局部更新 |
| `delete_scheduled_task` | 按 `job_id` 删除 |
| `enable_scheduled_task` | 启用 |
| `disable_scheduled_task` | 停用（保留配置） |
| `run_scheduled_task` | 立即触发一次执行 |
| `get_scheduled_task_runs` | 查看运行历史 |

## 建议工作流

1. `list_scheduled_tasks` 看当前状态
2. 按 UI 四件事确认字段
3. `create_scheduled_task` 创建
4. 必要时 `run_scheduled_task` 验证一次
5. 异常时 `get_scheduled_task_runs` 看失败原因
6. 调整走 `update_scheduled_task`（只传要改的字段）
7. 不再需要 → `delete_scheduled_task`
