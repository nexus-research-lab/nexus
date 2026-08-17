---
name: automation
description: 通过宿主签发的 Nexus runtime CLI 查询、规划、确认并管理 scheduled task 与 Agent heartbeat。用户要求提醒、延迟执行、周期检查、定时报表、补跑、结果重投递、持续关注、修改或删除任务、处理任务权限阻塞或唤醒 Agent 时使用；同时适用于定时任务后台 run 检查自身状态。
---

# Nexus 自动化

使用宿主注入的 `NEXUS_COMMAND_PATH`。该命令通过当前 physical round capability 绑定 owner、Agent、DM/Room/IM Session 和可选的当前 job/run；不要自行声明或覆盖这些身份。

所有调用都加 `--json`。不要搜索源码入口、手写 `go run ./cmd/nexus`、调用 `nexusctl automation`，也不要覆盖 `NEXUS_COMMAND_*` 环境变量。

固定流程是 `inspect → plan → apply → verify`。

## 工作流

1. 首次操作或边界不确定时读取当前 contract。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json automation contract
   ```

2. 用 `inspect` 查询现状。修改、删除、运行或补投递前，先定位唯一任务并取得 `job_id`、`configuration_version`、running run 和健康状态。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json automation inspect \
     --operation list \
     --input '{"query":"工作日报"}'
   ```

3. 对任何变更先运行 `plan`。检查 `summary`、`risk`、`target`、`current_revision` 和 `plan_digest`，不要把 plan 当成已执行。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json automation plan \
     --operation update \
     --input '{"job_id":"JOB_ID","enabled":false}'
   ```

4. 用户意图与 plan 一致后运行 `apply`。每项新意图使用一个稳定 `request_id`；同一调用重试必须复用。`apply` 会重新 plan，并通过当前 Nexus/Room/IM 会话发起原生真人确认；没有真实 allow 不会写入。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json automation apply \
     --operation update \
     --input '{"job_id":"JOB_ID","enabled":false}' \
     --expected-revision 'task:JOB_ID:VERSION' \
     --request-id 'automation-update-UNIQUE'
   ```

5. apply 成功后重新 `inspect operation=get`。需要核对执行或发送时，读取 `runs`、`events` 或 `report`。

操作字段、schedule 形状、创建/修改语义和 heartbeat 输入见 [references/operations.md](references/operations.md)。只有在当前动作需要时完整读取该文件。

## 权限与范围

- 普通 Agent 只能管理自身任务；Room 和外部 IM 继续绑定当前可信会话。只有主智能体自己的 Nexus WebSocket 私有 DM，且 contract 返回 `cross_agent_allowed=true` 时，才能指定其他 Agent 或已存在的真实 Session。
- 外部 IM 的 channel、account、target、thread、Session 和 `DeliveryGrant` 由宿主绑定。不要猜测、回显或传入底层路由。
- 后台 scheduled run 只有查询权限，并自动绑定当前 `job_id/run_id`；不得创建、修改、运行或删除任务。
- `apply` 的原生确认只批准管理命令。任务到点后调用工具仍服从任务自己的 permission mode、allow/deny 快照和持久审批；页面或 IM `/y`、`/a`、`/d` 决定是否恢复同一 logical run。
- 不使用 CLI 直接批准任务工具请求，不伪造 permission request、policy revision 或 run identity。
- `script` 任务属于人类控制面，Agent CLI 不创建、编辑、删除、修复或立即运行。

## 选择动作

- 执行失败、需要重新计算：修正任务后使用 `run`。
- 结果已经产生、仅投递失败：使用 `retry_delivery`，不要重新执行。
- active run 确认卡住：`update` 同时传 `enabled=false`、`cancel_active_run=true` 和当前 `run_id`。
- 精确时间提醒或报表：scheduled task。宽松周期自检或消费积累事件：heartbeat。
- 删除不可恢复；只在用户明确要求删除并核对唯一任务后执行。

回复只说明变更内容、作用域、排程、结果投递和验证状态，不输出 capability、内部路由、权限快照或完整审计载荷。
