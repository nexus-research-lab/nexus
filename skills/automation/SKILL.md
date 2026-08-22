---
name: automation
title: Nexus 自动化
description: 通过宿主签发的 Nexus runtime CLI 查询、规划、确认并管理 scheduled task。用户要求提醒、延迟执行、周期检查、定时报表、补跑、结果重投递、持续关注、修改或删除任务、处理任务权限阻塞时使用；同时适用于定时任务后台 run 检查自身状态。
scope: any
tags: [automation, scheduled-task, reminder]
---

# Nexus 自动化

Automation 使用宿主注入的 `NEXUS_COMMAND_PATH`，并绑定当前 owner、Agent、DM/Room/IM Session 与可选 job/run。不要声明或覆盖这些 identity，不调用 `nexusctl automation`，也不要从源码 `go run` 构建入口。

## 固定生命周期

1. 每个新意图先独立读取 current contract：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json automation contract
   ```

   PowerShell 使用 `& "${env:NEXUS_COMMAND_PATH}" ...`。严格遵守返回的顶层 `command_usage`、`contract` 与 `input_staging`；不要探测或覆盖 `NEXUS_COMMAND_*`。
2. 只选择 current contract 实际列出的 operation。每次新 input 写入前重新读取 contract，使用 fresh `input_staging.path`；第一次 Write 前先 Read，然后覆盖为一个完整 closed JSON object。不要用 shell、heredoc、`cat`、命令替换或重定向生成 JSON。
3. query 使用 `inspect`。mutation 固定走 `inspect → plan → apply → verify`：先定位唯一任务与 revision；检查 plan 的 normalized input、target、summary、risk、`current_revision` 和 `plan_digest`；保持同一输入文件内容不变再 apply。
4. apply 使用 plan 的 `current_revision` 和稳定 request ID，并由当前 Nexus/Room/IM 会话发起原生真人确认。没有真实 allow 就没有写入；plan 本身不代表用户批准或状态已改变。
5. apply 后写入 fresh verify 查询并 inspect `get`；执行/投递问题读取 runs、events 或 report。只按返回的 typed object 判断结果，不用 shell 后处理猜状态。

## 按当前动作读取参考

- list/get/runs/events/report 与 scheduled-run 只读范围：[references/queries.md](references/queries.md)
- create/update/delete/run/retry_delivery、schedule 与投递语义：[references/scheduled-tasks.md](references/scheduled-tasks.md)

只读取当前动作需要的参考；字段目录以 current contract 的 operation definition 为准。

## 权限边界

- 普通 Agent 只能管理自身任务；Room 与外部 IM 自动绑定当前可信会话。只有主智能体自己的 Nexus 私有 DM 且 `cross_agent_allowed=true` 时，才能选择其他真实 Agent/Session。
- channel、account、target、thread、Session、DeliveryGrant、job/run runtime identity 由宿主固定；不要猜、回显或写进输入。外部 IM 的空查询默认只覆盖当前会话。
- 后台 scheduled run 只有宿主绑定 job/run 的查询权限，不得创建、修改、运行或删除任务。
- apply 的确认只批准管理命令。任务运行时仍服从其 permission mode、工具 allow/deny 快照与持久审批；页面或 IM 的 `/y`、`/a`、`/d` 只恢复对应 logical run，CLI 不直接批准工具 permission request。
- `script` task 属于人工控制面，Agent CLI 不创建、修改、删除、修复或立即运行。

回复只说明真实变更、作用域、排程、投递和验证结果，不输出 capability、内部路由、权限快照或完整审计载荷。
