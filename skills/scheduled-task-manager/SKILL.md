---
name: scheduled-task-manager
description: 创建、查看、启停、运行、更新或删除 Nexus 定时任务时使用。尤其当用户用自然语言说“提醒我”“每天/每周/每隔一段时间”“定时做某事”时，先用本 skill 决策，再调用 nexus_automation MCP 工具。
---

# scheduled-task-manager

你负责把用户的自然语言需求稳定转换成 `nexus_automation` 定时任务工具调用。不要把底层字段丢给用户，也不要在工具报错后反复猜参数。

## 必须遵守

1. 用户要创建定时任务时，先按本文判断，再调用 `create_scheduled_task`。
2. 短提醒任务直接创建，不要询问 `execution_mode` / `reply_mode`。
3. 复杂任务缺少上下文时，先问用户，不要猜。
4. 工具报错后最多修正一次明确错误；如果仍失败，直接把错误说明给用户。
5. 向用户解释时使用产品语义，例如“执行会话”“结果回传”，不要讲 `session_target`、`delivery` 这类内部结构。

## 快速判断

**短提醒任务**满足全部条件时直接创建：

- 指令不超过 24 个中文字符或 24 个英文单词左右
- 只是提醒、打卡、喝水、站立、吃药、会议等轻量动作
- 不需要读取历史上下文、总结、分析、汇报、监控、整理、复盘

短提醒默认：

- `execution_mode` 不填
- `reply_mode` 不填
- 让工具默认 `temporary + none`

**复杂任务**命中任一条件时必须确认：

- 包含“总结、汇总、报告、分析、研究、整理、复盘、检查、监控、跟进”
- 需要读取当前会话、Room、公区、文件、历史消息、外部系统
- 用户明确希望结果发回某个会话、群、Room、频道
- 任务会长期维护状态或上下文

复杂任务至少确认两件事：

- 执行在哪里进行：主会话、当前会话、临时会话、专用长期会话
- 结果发到哪里：不回传、回到执行会话、回到指定会话

## 一句话解析规则

从用户句子提取三件事：

- `name`: 用户明确给出的任务名；没有则用动作生成短名称
- `instruction`: 到点要做什么，只写执行动作，不要夹带调度描述
- `schedule`: 频率或时间

示例：

用户：“test 每分钟提醒我喝水”

```json
{
  "name": "test",
  "instruction": "提醒我喝水",
  "schedule": {
    "kind": "interval",
    "interval_value": 1,
    "interval_unit": "minutes"
  }
}
```

不要为这个例子补 `execution_mode` 或 `reply_mode`。

## 创建模板

### 每隔一段时间

用户：“每 30 分钟提醒我站起来”

```json
{
  "name": "站立提醒",
  "instruction": "提醒我站起来",
  "schedule": {
    "kind": "interval",
    "interval_value": 30,
    "interval_unit": "minutes"
  }
}
```

`interval_unit` 只用 `seconds` / `minutes` / `hours`。

### 每天固定时间

用户：“每天早上 9 点提醒我开晨会”

```json
{
  "name": "晨会提醒",
  "instruction": "提醒我开晨会",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

工作日用：

```json
{
  "kind": "daily",
  "daily_time": "09:00",
  "weekdays": ["mon", "tue", "wed", "thu", "fri"]
}
```

### 单次任务

用户：“明天下午 3 点提醒我给客户回电话”

```json
{
  "name": "客户回电提醒",
  "instruction": "提醒我给客户回电话",
  "schedule": {
    "kind": "single",
    "run_at": "2026-04-29T15:00"
  }
}
```

用户说“今天、明天、下周”时，按当前本地时区理解。没有其他说明时使用上海时间。

### Cron

只有当用户明确给 cron 表达式，或 daily/interval 表达不了时才用：

```json
{
  "kind": "cron",
  "expr": "0 9 * * 1-5"
}
```

cron 必须是 5 段，且需要能被 UI 编辑：minute/hour 是单个整数，day-of-month 和 month 是 `*`。

## 复杂任务模板

用户：“每天 9 点总结这个 Room 昨天的进展并发回来”

先确认：

```text
你希望这个定时任务在哪个会话里执行？结果要回到哪里？
```

如果用户选择当前会话执行并回到当前会话：

```json
{
  "name": "每日进展总结",
  "instruction": "总结这个 Room 昨天的进展",
  "execution_mode": "existing",
  "reply_mode": "execution",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

当前会话存在时，`selected_session_key` 可不填，工具会使用当前会话。

## 出错处理

遇到这些错误时按固定方式处理：

- 缺 `execution_mode` / `reply_mode`: 说明这是复杂任务。问用户执行会话和结果回传，不要继续猜。
- `interval_value must be a positive integer`: 只检查一次是否把数字放进 `schedule.interval_value`，并确保是整数；修正一次后仍失败就报告错误。
- 需要 `selected_session_key`: 如果用户说“当前会话”，直接用当前会话；否则让用户选择已有会话。
- cron 无法翻译到 UI: 改用 `daily` 或 `interval`；如果表达不了，告诉用户当前 UI 不支持这种 cron。

## 常用工具顺序

1. 创建前通常不需要列任务，除非用户说“替换、修改、删除、同名任务、已有任务”。
2. 创建：`create_scheduled_task`
3. 查看：`list_scheduled_tasks`
4. 修改：`update_scheduled_task`
5. 停用/启用：`disable_scheduled_task` / `enable_scheduled_task`
6. 立即跑一次：`run_scheduled_task`
7. 查失败原因：`get_scheduled_task_runs`
8. 删除：先 `list_scheduled_tasks` 确认，再 `delete_scheduled_task`

## 参数对照

只在工具调用时使用这些字段：

- 任务名称: `name`
- 任务指令: `instruction`
- 目标智能体: `agent_id`，缺省当前智能体
- 单次: `schedule.kind="single"` + `run_at`
- 每天: `schedule.kind="daily"` + `daily_time` + 可选 `weekdays`
- 间隔: `schedule.kind="interval"` + `interval_value` + `interval_unit`
- cron: `schedule.kind="cron"` + `expr`
- 执行会话: `execution_mode` = `main` / `existing` / `temporary` / `dedicated`
- 结果回传: `reply_mode` = `none` / `execution` / `selected`
- 重叠策略: `overlap_policy` = `skip` / `allow`，缺省 `skip`
- 创建后启用: `enabled`，缺省 `true`
