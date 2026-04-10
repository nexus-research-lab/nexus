# Nexus 定时任务与 Heartbeat 自动化设计

日期：2026-04-09

## 背景

当前仓库已经存在定时任务相关的前端占位页面与少量类型定义：

- `web/src/pages/scheduled-tasks/`
- `web/src/lib/scheduled-task-api.ts`
- `web/src/types/scheduled-task.ts`

但后端实际能力尚未形成闭环：

- 没有可用的定时任务 API、持久化、调度循环、运行记录
- 没有类似 OpenClaw 的 heartbeat 主会话轮询机制
- 没有统一的跨通道主动投递能力，无法稳定支持 WebSocket、Telegram、Discord 的定时回复
- 现有 `agent/service/capability/scheduled/` 只有空壳，无法承载完整运行时

本设计目标是让 Nexus 在能力范围与使用体验上对齐 OpenClaw 的 cron + heartbeat 体系，支持：

- 精确定时任务：`at / every / cron`
- 主会话 heartbeat：周期性主会话 turn
- 定时回复：回到 WebSocket、Telegram、Discord 会话或频道
- 主会话任务注入：`sessionTarget=main` 通过 system event + wake 驱动
- 隔离任务：`isolated` 独立执行并记录运行历史
- 绑定会话任务：围绕已有会话持续自动执行
- 命名会话任务：围绕一个长期自动流程保留上下文
- 运行审计：cron 每次执行都有 run ledger

## 目标

### 产品目标

1. 用户可以像 OpenClaw 一样配置精确定时任务与 heartbeat。
2. 用户可以创建定时提醒、定时回复、周期巡检、隔离报告型任务。
3. 用户可以查看任务列表、启停状态、下次执行时间、运行历史与错误。
4. heartbeat 可以按 Agent 独立配置 cadence、active hours、target、prompt。
5. 支持 `websocket + Telegram + Discord` 三类现有会话入口的触发与投递。

### 架构目标

1. 调度逻辑与产品入口解耦。
2. 复用现有 Agent 执行与消息落库链路，不复制一套聊天执行器。
3. heartbeat 与 cron 共享统一的后台 agent turn 执行底座。
4. 主动投递统一走 Delivery Router，不让不同 runtime 各自发消息。
5. 运行记录独立于会话消息历史，避免审计与聊天内容混淆。

## 非目标

本次设计不要求同时引入 OpenClaw 其他自动化子系统的完整能力，例如：

- webhook hooks
- Gmail PubSub
- Task Flow / Clawflow
- standing orders 独立配置系统

上述能力的架构接口会预留，但不纳入本次实现范围。

## 核心设计结论

### 1. 目录分层

不把完整运行时全部塞进 `agent/service/capability/scheduled/`。

采用两层结构：

- `agent/service/automation/`
  - 放真正的自动化运行时内核
  - 包含 heartbeat、cron、runtime、delivery、runs
- `agent/service/capability/scheduled/`
  - 放“定时任务”产品入口 façade
  - 负责把前端/API 请求编排到底层 automation 服务

原因：

- `capability/scheduled` 适合表达产品能力入口
- heartbeat、cron、wake、delivery、run ledger 已经是横跨多通道的基础设施
- 按仓库约束，Python 单文件不能过大，拆到 automation 更利于长期维护

### 2. heartbeat 与 cron 的职责边界

严格对齐 OpenClaw 语义：

- heartbeat
  - 是周期性主会话 agent turn
  - 默认跑在主会话
  - 不产生日志任务
  - 适合近似周期的监控、检查、提醒
- cron
  - 是精确调度器
  - 支持 `at / every / cron`
  - 每次执行都产生日志 run 记录
  - 适合一次性提醒、日报、周期报告、定时回复

### 3. 会话语义

在现有 `session_key` 体系之上扩展，不引入第二套会话模型。

- 主会话：`agent:<agent_id>:system:dm:main`
- 绑定会话：使用现有真实会话键，例如 `agent:<agent_id>:tg:group:<chat_id>`
- 隔离会话：
  - 逻辑基键：`agent:<agent_id>:system:dm:cron:<job_id>`
  - 实际 run 键：`agent:<agent_id>:system:dm:cron:<job_id>:run:<run_id>`
- 命名会话：`agent:<agent_id>:system:dm:session:<name>`

### 4. 主会话 cron 不直接跑独立 agent

`sessionTarget=main` 的 cron 任务不直接开 isolated run。

它的真实行为是：

1. 写入主会话 system event 队列
2. 按 `wake_mode=now|next-heartbeat` 决定是否立即唤醒 heartbeat
3. 由 heartbeat / main-session runtime 在主会话上下文中消费该事件

这样才能保持与 OpenClaw 一致的“主脑上下文感知”体验。

## 现有仓库能力复用

### 复用的执行链路

后台 agent turn 不直接走 `ChatService.handle_chat_message()` 入口，而是复用以下组件：

- `agent/service/agent/agent_runtime.py`
- `agent/service/message/chat_message_processor.py`
- `agent/service/session/session_store.py`
- `agent/service/session/session_manager.py`

理由：

- `ChatService` 目前强耦合 websocket sender、ws chat task registry 与前台消息生命周期
- 自动化运行需要脱离“当前前端连接是否在线”的限制
- 但消息落库和 SDK 消息转换应保持与现有聊天链路一致

### 复用的会话存储

继续复用：

- `session_repository` 的 workspace 文件存储
- `session_store` 的消息/会话业务门面

自动化运行只新增：

- 主会话键的持久化策略
- 隔离/命名会话键的生成与清理策略

### 复用的数据库模式

参考 connectors 的做法：

- SQLAlchemy model + async sqlite repository
- API 层走 `resp.ok()` / `resp.fail()`

## 模块设计

### A. Heartbeat 模块

建议目录：

- `agent/service/automation/heartbeat/heartbeat_service.py`
- `agent/service/automation/heartbeat/heartbeat_scheduler.py`
- `agent/service/automation/heartbeat/heartbeat_prompt.py`
- `agent/service/automation/heartbeat/heartbeat_delivery_filter.py`
- `agent/service/automation/heartbeat/heartbeat_dispatcher.py`
- `agent/service/automation/heartbeat/heartbeat_state_store.py`

职责：

- cadence 计算
- active hours 判断
- `HEARTBEAT.md` 读取
- `tasks:` due-only 解析
- `HEARTBEAT_OK` ack 过滤
- 主会话 turn 调度
- heartbeat 配置状态持久化

### B. Cron 模块

建议目录：

- `agent/service/automation/cron/cron_service.py`
- `agent/service/automation/cron/cron_schedule.py`
- `agent/service/automation/cron/cron_timer.py`
- `agent/service/automation/cron/cron_runner.py`
- `agent/service/automation/cron/cron_normalizer.py`
- `agent/service/automation/cron/cron_run_log.py`
- `agent/service/automation/cron/cron_store_service.py`

职责：

- `at / every / cron` 调度
- timer 重臂
- 手动 run
- 重启恢复
- catch-up
- 主会话 / 隔离 / 绑定 / 命名会话执行分流
- run ledger 更新

### C. 共用 runtime 模块

建议目录：

- `agent/service/automation/runtime/agent_run_orchestrator.py`
- `agent/service/automation/runtime/run_context.py`
- `agent/service/automation/runtime/run_result.py`
- `agent/service/automation/runtime/system_event_queue.py`
- `agent/service/automation/runtime/wake_service.py`

职责：

- 统一执行 heartbeat / cron 触发的 agent turn
- system event 入队与消费
- `wake now / next-heartbeat`
- 统一 run result 结构

### D. Delivery 模块

建议目录：

- `agent/service/automation/delivery/delivery_router.py`
- `agent/service/automation/delivery/delivery_target.py`
- `agent/service/automation/delivery/delivery_memory.py`
- `agent/service/automation/delivery/delivery_sender.py`

职责：

- 解析投递目标
- 支持 `none / last / explicit`
- 支持 WebSocket / Telegram / Discord 投递
- 持久化 last route

### E. Run Ledger 模块

建议目录：

- `agent/service/automation/runs/run_ledger_service.py`
- `agent/service/automation/runs/run_ledger_repository.py`

职责：

- 记录 cron 每次执行
- 提供 runs 列表、详情、审计

### F. Capability façade

建议目录：

- `agent/service/capability/scheduled/scheduled_task_service.py`

职责：

- 面向产品的“定时任务”能力入口
- 调用 cron service / run ledger
- 不承载 heartbeat 核心逻辑

## 会话与运行语义

### 主会话

- 每个 Agent 一个固定主会话：`agent:<agent_id>:system:dm:main`
- heartbeat 默认运行在这里
- `sessionTarget=main` 的 cron 通过 system event 注入到这里
- 需要持久化：
  - session history
  - sdk session_id
  - 最近一次 heartbeat 状态
  - 最近一次外部投递目标

### 绑定会话

- 直接使用已有真实 `session_key`
- 创建定时任务时把当前会话固化到 job
- 适合：
  - 定时回复当前群聊
  - 在当前 DM 周期跟进

### 隔离会话

- 每次 run 一个 fresh session
- 不污染主会话
- 适合日报、巡检、分析报告

### 命名会话

- 持久保留上下文
- 不依赖某个外部 channel
- 适合长期自动流程

## heartbeat 语义

1. 到点后读取 heartbeat 配置
2. 读取 `HEARTBEAT.md`
3. 解析 `tasks:` block
4. 若无任务或文件有效内容为空，则 skip
5. 构造 prompt
6. 在主会话执行 turn
7. 用 `HEARTBEAT_OK` 规则过滤 ack-only 响应
8. 若需提醒，则通过 delivery router 投递

### `HEARTBEAT.md`

支持两种模式：

- 普通 checklist
- `tasks:` due-only block

### `HEARTBEAT_OK`

行为对齐 OpenClaw：

- 仅 heartbeat 场景下按 ack token 处理
- start/end 出现且剩余内容不超过 `ack_max_chars` 时，视为 silent ack
- 不应把 silent ack 外发

## cron 语义

### 支持的 schedule kind

- `at`
- `every`
- `cron`

### 支持的 session target

- `main`
- `isolated`
- `bound`
- `named`

### `main`

- 写 system event
- 走 wake 机制
- 最终由主会话 turn 处理

### `isolated`

- 直接独立执行
- 产生日志 run
- 可显式投递

### `bound`

- 在现有会话中执行
- 保留上下文
- 可实现定时回复

### `named`

- 在命名自动会话中执行
- 保留长期上下文

## 数据模型

### `automation_cron_job`

字段：

- `job_id`
- `name`
- `description`
- `agent_id`
- `schedule_kind`
- `run_at`
- `interval_seconds`
- `cron_expression`
- `timezone`
- `session_target_kind`
- `bound_session_key`
- `named_session_key`
- `wake_mode`
- `payload_kind`
- `instruction`
- `model`
- `thinking`
- `timeout_seconds`
- `delivery_mode`
- `delivery_channel`
- `delivery_to`
- `delivery_account_id`
- `delivery_thread_id`
- `enabled`
- `delete_after_run`
- `last_run_at`
- `next_run_at`
- `last_status`
- `last_error`
- `created_at`
- `updated_at`

### `automation_cron_run`

字段：

- `run_id`
- `job_id`
- `agent_id`
- `session_target_kind`
- `session_key`
- `status`
- `summary`
- `error`
- `delivery_mode`
- `delivery_status`
- `delivery_error`
- `delivery_channel`
- `delivery_to`
- `delivery_account_id`
- `delivery_thread_id`
- `model`
- `usage_json`
- `started_at`
- `ended_at`
- `created_at`

### `automation_heartbeat_state`

字段：

- `agent_id`
- `enabled`
- `every_seconds`
- `target_mode`
- `target_channel`
- `target_to`
- `target_account_id`
- `target_thread_id`
- `direct_policy`
- `prompt_override`
- `ack_max_chars`
- `light_context`
- `isolated_session`
- `include_reasoning`
- `active_hours_json`
- `task_state_json`
- `last_run_at`
- `next_run_at`
- `last_status`
- `last_error`
- `updated_at`

### `automation_delivery_route`

字段：

- `agent_id`
- `session_key`
- `channel`
- `to`
- `account_id`
- `thread_id`
- `updated_at`

## API 设计

### Scheduled Tasks

- `GET /agent/v1/scheduled-tasks`
- `GET /agent/v1/scheduled-tasks/{task_id}`
- `POST /agent/v1/scheduled-tasks`
- `PATCH /agent/v1/scheduled-tasks/{task_id}`
- `DELETE /agent/v1/scheduled-tasks/{task_id}`
- `POST /agent/v1/scheduled-tasks/{task_id}/run`
- `PATCH /agent/v1/scheduled-tasks/{task_id}/status`
- `GET /agent/v1/scheduled-tasks/{task_id}/runs`
- `GET /agent/v1/scheduled-task-runs/{run_id}`

### Heartbeat

- `GET /agent/v1/automation/heartbeat`
- `GET /agent/v1/automation/heartbeat/{agent_id}`
- `PUT /agent/v1/automation/heartbeat/{agent_id}`
- `POST /agent/v1/automation/heartbeat/{agent_id}/wake`
- `GET /agent/v1/automation/heartbeat/{agent_id}/status`

### Automation Overview

- `GET /agent/v1/automation/overview`

## 关键请求模型

### 创建定时任务

- `name`
- `description`
- `agent_id`
- `schedule`
  - `kind=at|every|cron`
  - `run_at`
  - `interval_seconds`
  - `cron_expression`
  - `timezone`
- `session_target`
  - `kind=main|isolated|bound|named`
  - `bound_session_key`
  - `named_session_key`
  - `wake_mode`
- `payload`
  - `kind=system_event|agent_turn`
  - `instruction`
  - `model`
  - `thinking`
  - `timeout_seconds`
- `delivery`
  - `mode=none|last|explicit`
  - `channel`
  - `to`
  - `account_id`
  - `thread_id`
- `enabled`
- `delete_after_run`

### 更新 heartbeat 配置

- `enabled`
- `every_seconds`
- `target_mode`
- `target_channel`
- `target_to`
- `target_account_id`
- `target_thread_id`
- `direct_policy`
- `prompt_override`
- `ack_max_chars`
- `light_context`
- `isolated_session`
- `include_reasoning`
- `active_hours`
  - `start`
  - `end`
  - `timezone`
- `show_ok`
- `show_alerts`
- `use_indicator`

## 四条核心执行链路

### 1. heartbeat 主会话链路

1. heartbeat 到点
2. 读取 heartbeat 配置与 `HEARTBEAT.md`
3. 解析 due tasks
4. 生成 prompt
5. 在主会话执行一轮 agent turn
6. 结果经 `HEARTBEAT_OK` 过滤
7. 需要提醒时走 delivery router

### 2. cron main-session 链路

1. cron 到点
2. job 为 `sessionTarget=main`
3. 写 system event queue
4. `wake now` 则立刻触发 heartbeat / 主会话 dispatch
5. `next-heartbeat` 则等待下次 heartbeat tick
6. 主会话消费事件并决定是否投递

### 3. cron isolated / named / bound 链路

1. cron 到点
2. 创建 run ledger
3. 按 target 类型生成 session key
4. orchestrator 执行一轮
5. 消息落到对应 session
6. delivery router 发回目标
7. 更新 run ledger 与 job 状态

### 4. 定时回复链路

本质上是 cron + delivery：

- Web 会话：落消息到对应 session，在线即推送，不在线保历史
- Telegram：主动发回 chat/topic，并更新 last route
- Discord：主动发回 channel/thread，并更新 last route

## 产品能力面

首版必须支持：

1. 创建一次性提醒
2. 创建 `every` 周期任务
3. 创建 cron 表达式任务
4. 创建绑定当前会话的定时回复
5. 创建 isolated 报告任务并指定投递目标
6. 配置 heartbeat cadence、active hours、target
7. 手动 wake heartbeat
8. 查看任务运行历史
9. 查看最近错误与投递失败

## 实现顺序

### 阶段 1：基础数据层

- automation models / repositories / migration
- heartbeat 与 cron 基础 schema

### 阶段 2：统一执行与投递底座

- agent run orchestrator
- system event queue
- wake service
- delivery router

### 阶段 3：heartbeat runtime

- heartbeat scheduler
- `HEARTBEAT.md`
- `tasks:` due-only
- `HEARTBEAT_OK`
- active hours

### 阶段 4：cron runtime

- `at / every / cron`
- `main / isolated / bound / named`
- run ledger
- pause / resume / manual run

### 阶段 5：API 产品面

- scheduled tasks API
- heartbeat API
- overview API

### 阶段 6：前端页面

- Scheduled Tasks 管理页
- 创建 / 编辑表单
- run history
- heartbeat 设置面板

### 阶段 7：稳定性与回归

- 重启恢复
- catch-up
- active hours 边界
- 三通道投递
- delivery 失败与 run 失败分离

## 风险与约束

### 1. 现有 channel outbound 能力不足

需要补一层真正的主动投递接口，否则 heartbeat 和 cron 无法对 Telegram / Discord 稳定发回。

### 2. 主会话 system event 目前不存在

需要新增持久化 event queue，否则 `sessionTarget=main` 只能退化成伪 isolated run。

### 3. heartbeat 与现有 WebSocket heartbeat 名称冲突

前端当前有 websocket transport heartbeat，后端新增的是 automation heartbeat。
文档与代码命名必须明确区分：

- websocket heartbeat
- automation heartbeat

### 4. 需要严格控制文件大小

所有 Python 文件保持 300 行以内，优先拆分，避免再次形成大而杂的运行时文件。

## 最终结论

Nexus 不应把完整自动化运行时塞进 `capability/scheduled`。

正确结构是：

- `capability/scheduled` 负责“产品入口”
- `automation/*` 负责“运行时内核”

heartbeat 与 cron 是并列自动化机制：

- heartbeat 负责近似周期的主会话感知
- cron 负责精确定时与隔离任务

两者共用：

- session 体系
- agent run orchestrator
- delivery router
- delivery memory

cron 额外拥有：

- run ledger
- 精确调度
- 手动运行
- 运行历史

按本设计落地后，Nexus 将具备与 OpenClaw 对齐的定时任务与 heartbeat 自动化能力范围，并且能覆盖现有 `websocket + Telegram + Discord` 的使用体验。
