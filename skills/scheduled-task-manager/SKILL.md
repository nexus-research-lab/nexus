---
name: scheduled-task-manager
description: 管理智能体与 Room 会话上的定时任务。当需要创建、查看、启停、立即执行、查看运行记录或删除定时任务时，使用此 skill。
---

# scheduled-task-manager

通过 `nexus_automation` MCP 工具管理 Nexus 平台的定时任务。这些工具和前端「新建任务」对话框一一对应——填工具参数等价于在 UI 上点字段。

## 使用原则

- **作用域**：普通 Agent 只能 CRUD 自己 `agent_id` 名下的任务，`list_scheduled_tasks` 也只会返回自己的任务，越权调用会被后端拒绝。主智能体（Nexus）豁免该限制，可指定任意 `agent_id`。
- **必填三件事**：`name` + `instruction` + `schedule`。其余字段都可缺省。
- **lenient 默认**：短文本（≤24 字、不含 "总结/汇总/报告/分析" 等重业务关键词）的提醒类任务，省略 `execution_mode` / `reply_mode` 时会自动按 `temporary + none` 创建——agent 跑临时会话，结果不回传，最贴合"提醒一下"的场景。
- **想让结果回到当前会话**：必须显式 `execution_mode=existing` + `reply_mode=execution`。`execution_mode=existing` 不传 `selected_session_key` 时默认使用当前会话。
- **schedule.timezone 缺省自动补**：服务器默认时区（通常 `Asia/Shanghai`），不必每次都写。
- 重业务任务（含"总结/汇总/分析…"等关键词或长指令）拿不到默认值，工具会返回 `missing required scheduling fields`，这时再 AskUserQuestion 跟用户确认 `execution_mode` / `reply_mode`。
- 删除或覆盖现有任务前，先 `list_scheduled_tasks` 确认目标 `job_id`。

## UI 字段 ↔ 工具参数对照

| UI 字段 | 工具参数 |
|---|---|
| 任务名称 | `name` |
| 目标智能体 | `agent_id` |
| 执行会话 = 使用主会话 | `execution_mode: "main"` |
| 执行会话 = 使用现有会话 | `execution_mode: "existing"` (+ `selected_session_key`，缺省=当前会话) |
| 执行会话 = 每次新建临时会话 | `execution_mode: "temporary"` |
| 执行会话 = 使用专用长期会话 | `execution_mode: "dedicated"` + `named_session_key` |
| 结果回传 = 不回传 | `reply_mode: "none"` |
| 结果回传 = 回到执行会话 | `reply_mode: "execution"` |
| 结果回传 = 回到指定会话 | `reply_mode: "selected"` + `selected_reply_session_key` |
| 调度 = 单次 | `schedule.kind: "single"` + `run_at` |
| 调度 = 每天 | `schedule.kind: "daily"` + `daily_time` (+ `weekdays`) |
| 调度 = 间隔 | `schedule.kind: "interval"` + `interval_value` + `interval_unit` |
| 调度 = 标准 cron | `schedule.kind: "cron"` + `expr`（5 段表达式） |
| 时区 | `schedule.timezone`（IANA，缺省按服务器默认时区） |
| 任务指令 | `instruction` |
| 创建后立即启用任务 | `enabled`（缺省 true） |

向用户描述时用左列 UI 语义，不要把右列的原始字段名甩给用户。

## Schedule 四种模式的参数模板

**单次**（对齐 UI「单次」Tab）：
```json
{
  "kind": "single",
  "run_at": "2026-04-21T18:00"
}
```

**每天**（对齐 UI「每天」Tab，不填 `weekdays` = 每天执行）：
```json
{
  "kind": "daily",
  "daily_time": "09:00",
  "weekdays": ["mon", "tue", "wed", "thu", "fri"]
}
```
`weekdays` 取值：`mon`/`tue`/`wed`/`thu`/`fri`/`sat`/`sun`。

**间隔**（对齐 UI「间隔」Tab）：
```json
{
  "kind": "interval",
  "interval_value": 30,
  "interval_unit": "minutes"
}
```
`interval_unit` 取值：`seconds`/`minutes`/`hours`。

**标准 cron**（无独立 UI Tab，工具会把表达式翻译回 daily 形态以保证 UI 可编辑）：
```json
{
  "kind": "cron",
  "expr": "0 9 * * 1-5"
}
```
`expr` 是标准 5 段 cron 表达式（minute hour dom month dow）。也接受别名 `cron` / `cron_expression`。
注意：只传 `expr` 不写 `kind` 也会自动推断为 `cron`。

**cron 仅限 UI 可编辑形态**：
- minute/hour 必须是单个整数（不能用 `*/15`、`1-5`、`1,30`）
- day-of-month 和 month 必须是 `*`（不能用月份/日期约束）
- day-of-week 支持 `*`、单数字、`1-5` 区间、`1,3,5` 列表
- 翻译不出来会被拒绝 → 该用 `kind=interval` 或拆成多个 `kind=daily`

## 兼容写法：平铺 schedule 字段

部分模型不喜欢嵌套对象，可以把 schedule 字段直接平铺到顶层，工具会自动重组：
```json
{
  "name": "晨会提醒",
  "instruction": "提醒晨会",
  "kind": "daily",
  "daily_time": "09:00"
}
```
等价于把这些键放在 `schedule` 对象里。

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
2. 短提醒类：直接 `create_scheduled_task` 填 name+instruction+schedule 即可
3. 重业务类：按 UI 四件事跟用户确认 execution_mode / reply_mode 后再创建
4. 必要时 `run_scheduled_task` 验证一次
5. 异常时 `get_scheduled_task_runs` 看失败原因
6. 调整走 `update_scheduled_task`（只传要改的字段）
7. 不再需要 → `delete_scheduled_task`
