---
name: automation
title: Nexus 自动化
description: 通过宿主提供的 nexus_runtime 结构化工具查询、规划、确认并管理 scheduled task。用户要求提醒、延迟执行、周期检查、定时报表、补跑、结果重投递、持续关注、修改或删除任务、处理任务权限阻塞时使用；同时适用于定时任务后台 run 检查自身状态。
scope: any
tags: [automation, scheduled-task, reminder]
---

# Nexus 自动化

Automation 使用宿主提供的 `nexus_runtime.command`，并绑定当前 owner、Agent、DM/Room/IM Session 与可选 job/run。不要声明或覆盖这些 identity，不调用 `nexusctl automation`，也不要通过 shell 或文件构造命令。

## 固定生命周期

1. 每个新意图先调用 current contract：

   ```json
   {"domain":"automation","action":"contract"}
   ```

   严格遵守返回的 contract。不要探测命令路径、临时文件或环境变量。
2. 只选择 current contract 实际列出的 operation。每次新输入前重新读取该 operation 的 exact contract，并把完整 closed object 直接放在 `input` 字段中；不要落盘或通过 shell 转码。
3. query 使用 `inspect`。mutation 固定走 `inspect → plan → apply → verify`：先定位唯一任务与 revision；检查 plan 的 normalized input、target、summary、risk、`current_revision` 和 `plan_digest`；apply 时复用同一结构化 input。
4. apply 使用 plan 的 `current_revision` 和稳定 request ID，并由当前 Nexus/Room/IM 会话发起原生真人确认。没有真实 allow 就没有写入；plan 本身不代表用户批准或状态已改变。
5. apply 后用 fresh input inspect `get`；执行/投递问题读取 runs、events 或 report。只按返回的 typed object 判断结果。

## 按当前动作读取参考

- list/get/runs/events/report 与 scheduled-run 只读范围：[references/queries.md](references/queries.md)
- create/update/delete/run/retry_delivery、schedule 与投递语义：[references/scheduled-tasks.md](references/scheduled-tasks.md)

只读取当前动作需要的参考；字段目录以 current contract 的 operation definition 为准。

## 权限边界

- 普通 Agent 只能管理自身任务；Room 与外部 IM 自动绑定当前可信会话。只有主智能体自己的 Nexus 私有 DM 且 `cross_agent_allowed=true` 时，才能选择其他真实 Agent/Session。
- channel、account、target、thread、Session、DeliveryGrant、job/run runtime identity 由宿主固定；不要猜、回显或写进输入。外部 IM 的空查询默认只覆盖当前会话。
- 后台 scheduled run 只有宿主绑定 job/run 的查询权限，不得创建、修改、运行或删除任务。
- apply 的确认只批准管理命令。任务运行时仍服从其 permission mode、工具 allow/deny 快照与持久审批；页面或 IM 的 `/y`、`/a`、`/d` 只恢复对应 logical run，`nexus_runtime` 不直接批准工具 permission request。
- `script` task 属于人工控制面，Agent 不创建、修改、删除、修复或立即运行。

回复只说明真实变更、作用域、排程、投递和验证结果，不输出 capability、内部路由、权限快照或完整审计载荷。
