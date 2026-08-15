---
name: automation
title: Nexus 自动化
description: 在 Nexus 中创建、查询、修改、删除、立即运行和检查持久化定时任务，并管理 Agent heartbeat。用户要求提醒、延迟执行、周期检查、定时报表、补跑、结果重投递、持续关注或唤醒 Agent 时使用；不要用临时会话状态或系统 cron 代替。
scope: any
tags: [automation, scheduled-task, reminder, heartbeat]
---

# Nexus 自动化

使用 `nexus_automation` 的两个工具。`automation_query` 只读，`automation_update` 执行受权限保护的变更。工具 schema 是字段真相源，本 Skill 负责选择操作和组织工作流。

## 选择入口

`automation_query` 的 `operation`：

- `list`：查找当前或已删除任务。
- `get`：读取任务配置、健康摘要和最近观测。
- `runs`：读取运行历史。
- `events`：读取管理审计。
- `report`：按日期汇总运行与投递状态。
- `heartbeat`：读取 heartbeat 配置和运行态。

`automation_update` 的 `operation`：

- `create`：创建持久化任务。
- `update`：局部修改、启停任务，或中断当前运行。
- `delete`：删除任务。
- `run`：立即触发一次，不改变后续排程。
- `retry_delivery`：只重投递已完成 run 的结果，不重新执行任务。
- `set_heartbeat`：局部修改 heartbeat 配置。
- `wake`：立即唤醒，或把上下文留到下一次 heartbeat。

修改、删除、运行或补投递前，如果没有精确 `job_id`，先用 `automation_query operation=list` 定位。自然语言 `query` 只有在当前权限范围内唯一命中时才会执行变更；有多个候选时让用户确认。

## 范围与定位

- 在 DM、Room 或 IM 群中查询时，当前会话的任务优先匹配。用户明确说“这里”“当前会话”“这个群”或“当前频道”时，将范围限定到当前会话；用户给出具体任务名时仍定位唯一任务。
- `report` 未指定 `job_id`、`agent_id` 或具体任务时，默认汇总当前会话相关任务。不要为了查询当前群的发送情况猜测外部通道路由。
- 在定时任务自身的后台运行中，`get`、`runs`、`events` 和 `report` 可以省略 `job_id`/`query`，宿主会绑定当前任务。
- `runs` 和 `events` 可以检查已删除任务；`get`、修改和运行只面向当前未删除任务。追溯删除记录时先 `list` 并传 `include_deleted=true`。
- 普通 Agent、Room 和外部来源保持自身或当前会话范围。只有 owner main 在自己的可信 Nexus 私有 DM 中才可按 schema 指定其他 Agent 或真实会话。

## 创建任务

`create` 必须提供稳定的 `request_id`、`name`、`instruction` 和 `schedule`。同一次创建调用重试时复用原 `request_id`，避免重复任务。

调度格式：

- 单次：`{"kind":"single","run_at":"2026-08-16T09:00:00+08:00"}`
- 每日或指定星期：`{"kind":"daily","daily_time":"09:00","weekdays":["mo","tu","we","th","fr"],"timezone":"Asia/Shanghai"}`
- 固定间隔：`{"kind":"interval","interval_value":30,"interval_unit":"minutes"}`
- 标准五段 cron：`{"kind":"cron","expr":"0 9 * * 1-5","timezone":"Asia/Shanghai"}`

遵循这些默认值：

- `context_mode` 默认 `isolated`。只有任务确实需要当前聊天历史时才用 `current`。
- 有当前会话且用户期待结果时使用 `deliver_result=true`。外部 IM 的账号、目标、thread 和 session 由 Nexus 从可信上下文绑定，不要猜测或填写路由。
- 省略 `permission_mode` 会复制创建时 Session/Agent 的有效权限。只有用户明确要求时才选择 `bypassPermissions`。
- `instruction` 要自包含，写清目标、输入、输出和失败处理。任务无人值守执行，不能依赖 `AskUserQuestion` 临时补充信息。
- 有重叠风险时保留默认 `overlap_policy=skip`；只有并发运行确实安全时才用 `allow`。
- 用户给出截止时间时使用 `expires_at`。到期只停止后续触发，不中断已经开始的运行；更新时用 `clear_expires_at=true` 清除截止时间。
- 优先使用 `single`、`daily` 或 `interval`。`cron` 只接受可转换为 UI 日/周调度的标准五段表达式：分钟和小时是单个整数，日期和月份必须为 `*`；月度等其他周期改用 `interval` 或拆成多个任务，不擅自改写用户时间要求。

## 修改与恢复

- 小幅补充要求使用 `instruction_append`；明确重写时才使用 `instruction`，两者不要同时传。
- 暂停任务使用 `update` + `enabled=false`。
- 卡住的 active run 使用 `update` + `enabled=false` + `cancel_active_run=true`；需要继续时再显式启用或 `run`。
- `run` 只立即触发一次，不改变后续排程，也不会重新启用已暂停任务。用户要求恢复长期排程时先 `update enabled=true`。
- 执行失败需要重新计算结果时使用 `run`。结果已经生成、只有投递失败时使用 `retry_delivery`；优先传报告返回的 `run_id`，存在多个可补投递 run 时先列出候选让用户选择。
- 删除是不可逆的用户操作，只在用户明确要求删除时调用 `delete`。删除时若仍有 active run，服务会尝试把它标记为 cancelled。

## 解释报告

根据 `report` 或 `get` 返回的信号选择动作，不把“执行失败”和“投递失败”混为一谈：

- `execution_attention` 或失败执行需要重新计算时，修正配置后使用 `run`。
- `running` 且确认已经卡住时，使用 `update` + `cancel_active_run=true`；不要仅因运行时间较长就擅自中断。
- `delivery_attention` 且存在 `manual_redelivery_run_ids` 时，使用 `retry_delivery`，不重新执行任务。
- 已删除任务只解释历史和审计；不能补投递、修改或运行。

变更后用 `automation_query operation=get` 写后核验；关注发送情况时再用 `report` 或 `runs`。

## Heartbeat

Heartbeat 适合 Agent 持续自检、消费积累事件或进行宽松周期关注。精确时间的用户提醒、报表和延迟动作仍创建 scheduled task。

- `set_heartbeat` 可修改 `enabled`、`every_seconds`、`target_mode` 和 `ack_max_chars`。
- `wake` 的 `mode=now` 立即登记唤醒；`mode=next-heartbeat` 把可选 `text` 留到下一周期。
- `set_heartbeat` 会由工具读取版本、CAS 更新并重读核验；遇到并发冲突时重新查询，不覆盖较新的配置。`wake` 不修改 heartbeat 配置或版本。
- 纯 `HEARTBEAT_OK` 会按配置抑制投递，不要把 heartbeat 当成保证用户可见送达的提醒。

## 回复

创建或修改后简要说明任务名称、时间、是否回传结果和当前状态。查询报告时优先解释异常与下一步，不向用户倾倒内部路由、权限快照或完整审计载荷。
