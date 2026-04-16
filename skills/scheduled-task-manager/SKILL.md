---
name: scheduled-task-manager
description: 管理 Agent 与 Room 会话上的定时任务。当需要创建、查看、启停、立即执行、查看运行记录或删除定时任务时，使用此 skill。
---

# scheduled-task-manager

管理 Nexus 平台中的定时任务。优先使用主智能体内置的定时任务工具，不要把这项能力当成 shell 命令包装器来使用。

## 使用原则

- 涉及创建任务时，先明确四件事：
  - 归属对象：`Agent` 还是 `Room`
  - 执行方式：`主会话 / 现有会话 / 临时会话 / 专用长期会话`
  - 调度规则：`every / cron / at`
  - 执行内容：任务要做什么
- 涉及删除或覆盖现有任务时，先列出当前任务，确认目标 `job_id`。
- 默认使用正常模式读取紧凑 JSON；只有排查异常时才加 `--verbose`。

## UI 字段对照

- UI 的“使用主会话” = `session_target.kind = main`
- UI 的“使用现有会话” = `session_target.kind = bound`
- UI 的“每次新建临时会话” = `session_target.kind = isolated`
- UI 的“使用专用长期会话” = `session_target.kind = named`
- UI 的“结果回传”
  - “不回传” = `delivery.mode = none`
  - “回到执行会话” = 优先回到本次选中的执行/上下文会话；若执行目标是 Agent 主会话，则结果留在主会话内，不额外投递
  - “回到指定会话” = `delivery.mode = explicit`

如果用户是从 UI 理解需求，优先用 UI 语义复述一遍，再落到结构化字段，避免把 `main / bound / named / isolated` 直接甩给用户。

## 可用动作

- `list_scheduled_tasks`
- `create_scheduled_task`
- `update_scheduled_task`
- `delete_scheduled_task`
- `enable_scheduled_task`
- `disable_scheduled_task`
- `run_scheduled_task`
- `get_scheduled_task_runs`

## 建议工作流

1. 先 `list_scheduled_tasks` 看当前状态
2. 创建前确认归属对象、执行方式和调度规则
3. 创建后根据需要 `run_scheduled_task` 做一次验证
4. 观察异常时使用 `get_scheduled_task_runs`
5. 调整配置时优先 `update_scheduled_task`
6. 不再需要时再执行 `delete_scheduled_task`
