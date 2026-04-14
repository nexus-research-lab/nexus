---
name: scheduled-task-manager
description: 管理 Agent 与 Room 会话上的定时任务。当需要创建、查看、启停、立即执行、查看运行记录或删除定时任务时，使用此 skill。
---

# scheduled-task-manager

管理 Nexus 平台中的定时任务。通过 CLI 工具执行任务创建、查询和运行管理。

CLI 工具路径：`python3 "{project_root}/agent/cli.py"`

## 使用原则

- 涉及创建任务时，先明确四件事：
  - 归属对象：`Agent` 还是 `Room`
  - 目标会话：具体哪个 session
  - 调度规则：`every / cron / at`
  - 执行内容：任务要做什么
- 涉及删除或覆盖现有任务时，先列出当前任务，确认目标 `job_id`。
- 默认使用正常模式读取紧凑 JSON；只有排查异常时才加 `--verbose`。

## 常用命令

### 列出任务

```bash
python3 "{project_root}/agent/cli.py" list_scheduled_tasks
python3 "{project_root}/agent/cli.py" list_scheduled_tasks --agent_id "research"
```

### 创建任务

主会话：

```bash
python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "晨间简报" \
  --agent-id "research" \
  --main \
  --instruction "整理今天要关注的 3 个重点" \
  --schedule-kind "every" \
  --interval-seconds 86400 \
  --timezone "Asia/Shanghai"
```

绑定现有会话：

```bash
python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "会话提醒" \
  --agent-id "research" \
  --session-key "agent:research:ws:dm:launcher-app-research" \
  --instruction "提醒我检查发版状态" \
  --schedule-kind "at" \
  --run-at "2026-04-20T09:00" \
  --timezone "Asia/Shanghai"
```

命名会话：

```bash
python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "工作日晨报" \
  --agent-id "research" \
  --named-session-key "morning-brief" \
  --instruction "输出工作日早间简报" \
  --schedule-kind "cron" \
  --cron-expression "0 8 * * 1-5" \
  --timezone "Asia/Shanghai"
```

独立会话：

```bash
python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "隔离巡检" \
  --agent-id "research" \
  --isolated \
  --instruction "检查近期错误并输出摘要" \
  --schedule-kind "every" \
  --interval-seconds 1800 \
  --timezone "Asia/Shanghai"
```

### 启用 / 禁用

```bash
python3 "{project_root}/agent/cli.py" enable_scheduled_task --job-id "job_xxx"
python3 "{project_root}/agent/cli.py" disable_scheduled_task --job-id "job_xxx"
```

### 立即运行

```bash
python3 "{project_root}/agent/cli.py" run_scheduled_task --job-id "job_xxx"
```

### 查看运行记录

```bash
python3 "{project_root}/agent/cli.py" get_scheduled_task_runs --job-id "job_xxx"
```

### 删除任务

```bash
python3 "{project_root}/agent/cli.py" delete_scheduled_task --job-id "job_xxx"
```

## 建议工作流

1. 先 `list_scheduled_tasks` 看当前状态
2. 创建前确认归属对象、会话和调度规则
3. 创建后根据需要 `run_scheduled_task` 做一次验证
4. 观察异常时使用 `get_scheduled_task_runs`
5. 不再需要时再执行 `delete_scheduled_task`
