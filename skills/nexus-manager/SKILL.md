---
name: nexus-manager
description: 管理 Nexus 的 Agent、Room、Workspace 与 Skill 系统操作。当用户提到创建 agent、创建 room、邀请成员、查看 room、读写工作区文件、安装或卸载 skill、删除成员或房间、查询系统协作结构时，使用此 skill，即使没有明确说“管理”二字。
---

# nexus-manager

管理 Nexus 平台的 Agent、Room、Workspace 与 Skill。通过 CLI 工具执行系统操作。

CLI 工具路径：`python3 "{project_root}/agent/cli.py"`

## CLI 输出约定

- 默认调用不要加 `--verbose`，CLI 正常模式只读取最终 JSON 结果，避免把过程日志灌回上下文。
- 只有在排查异常、确认 skill 部署过程或追踪系统初始化问题时，才显式加 `--verbose`。
- 需要人工阅读结构化结果时，再加 `--pretty`；正常推理链路优先使用默认紧凑 JSON。
- 如果命令执行失败，先看返回的 `error`；只有 `error` 信息不足以定位问题时，再用 `--verbose` 重跑一次。

```bash
# 正常调用：默认只读结果 JSON
python3 "{project_root}/agent/cli.py" list_agents

# 排查问题：显式打开过程日志
python3 "{project_root}/agent/cli.py" --verbose list_agents

# 人工查看结果：格式化输出
python3 "{project_root}/agent/cli.py" --pretty list_agents
```

## 核心概念

- **Agent（成员）**：具有独立工作空间的智能体，可被邀请加入 Room 协作。
- **Room（群组空间）**：多个 Agent 共处的对话空间，支持创建后追加成员。
- **Workspace（工作区）**：每个 Agent 独立拥有的文件空间，可读写业务文件与记忆文件。
- **Skill（技能）**：部署到 Agent 工作区中的能力包，决定其可用专业动作。
- **主智能体**：系统内置的保留 Agent，不能作为 Room 成员，所有 Room 操作由它发起。
- 每个成员创建后自动获得独立工作空间（workspace），用于存放技能、工具配置和文件。

## 命令参考

### Agent 管理

#### list_agents — 列出所有成员

```bash
python3 "{project_root}/agent/cli.py" list_agents
python3 "{project_root}/agent/cli.py" list_agents --include_main
```

- 默认不包含主智能体；加 `--include_main` 可包含。
- 返回字段：`agent_id`、`name`、`status`、`workspace_path`、`provider`、`skills_enabled`

#### validate_agent_name — 校验成员名称

```bash
python3 "{project_root}/agent/cli.py" validate_agent_name --name "Research"
```

- 创建成员前应先校验名称，避免冲突或非法字符。
- 返回校验结果，包含是否通过及原因。

#### create_agent — 创建成员

```bash
python3 "{project_root}/agent/cli.py" create_agent --name "Research"
python3 "{project_root}/agent/cli.py" create_agent --name "Research" --provider "glm"
```

- `--name` 必填；`--provider` 可选，值为 Provider key，来源于 Settings -> Providers。
- 返回字段：`agent_id`、`name`、`workspace_path`、`provider`、`skills_enabled`、`status`

#### get_agent — 读取成员详情

```bash
python3 "{project_root}/agent/cli.py" get_agent --agent_id "research"
```

#### get_agent_sessions — 读取成员会话

```bash
python3 "{project_root}/agent/cli.py" get_agent_sessions --agent_id "research"
```

#### delete_agent — 删除成员

```bash
python3 "{project_root}/agent/cli.py" delete_agent --agent_id "research"
```

### Room 管理

#### list_rooms — 查看最近 Room 列表

```bash
python3 "{project_root}/agent/cli.py" list_rooms
python3 "{project_root}/agent/cli.py" list_rooms --limit 10
```

- 返回字段：`room_id`、`room_type`、`name`、`description`、`member_agent_ids`、`updated_at`

#### get_room — 读取 Room

```bash
python3 "{project_root}/agent/cli.py" get_room --room_id "abc123"
```

#### get_room_contexts — 读取 Room 上下文

```bash
python3 "{project_root}/agent/cli.py" get_room_contexts --room_id "abc123"
```

#### create_room — 创建 Room

```bash
python3 "{project_root}/agent/cli.py" create_room --agent_ids "research,writer" --name "内容团队" --title "Kickoff" --description "内容生产协作空间"
```

- `--agent_ids` 必填，逗号分隔的成员 ID 列表。
- `--name`、`--title`、`--description` 可选，不指定则自动生成。
- 单成员时类型为 `dm`（私聊），多成员时类型为 `room`（群组）。
- 返回字段：`room_id`、`room_type`、`room_name`、`conversation_id`、`conversation_title`、`member_agent_ids`

#### update_room — 更新 Room

```bash
python3 "{project_root}/agent/cli.py" update_room --room_id "abc123" --name "内容团队" --title "本周计划"
```

#### add_room_member — 向 Room 追加成员

```bash
python3 "{project_root}/agent/cli.py" add_room_member --room_id "abc123" --agent_id "translator"
```

- `--room_id` 和 `--agent_id` 均必填。
- 仅支持群组类型 Room（`room`），不支持私聊（`dm`）。
- 返回字段：`room_id`、`room_name`、`conversation_id`、`member_agent_ids`

#### remove_room_member — 移除 Room 成员

```bash
python3 "{project_root}/agent/cli.py" remove_room_member --room_id "abc123" --agent_id "translator"
```

#### delete_room — 删除 Room

```bash
python3 "{project_root}/agent/cli.py" delete_room --room_id "abc123"
```

### Workspace 操作

#### list_workspace_files — 列出工作区文件

```bash
python3 "{project_root}/agent/cli.py" list_workspace_files --agent_id "research"
```

#### read_workspace_file — 读取工作区文件

```bash
python3 "{project_root}/agent/cli.py" read_workspace_file --agent_id "research" --path "RUNBOOK.md"
```

#### update_workspace_file — 更新工作区文件

```bash
python3 "{project_root}/agent/cli.py" update_workspace_file --agent_id "research" --path "RUNBOOK.md" --content "# 新计划"
```

#### create_workspace_entry — 创建工作区条目

```bash
python3 "{project_root}/agent/cli.py" create_workspace_entry --agent_id "research" --path "notes/todo.md" --entry_type "file" --content "- kickoff"
python3 "{project_root}/agent/cli.py" create_workspace_entry --agent_id "research" --path "notes" --entry_type "dir"
```

#### rename_workspace_entry — 重命名工作区条目

```bash
python3 "{project_root}/agent/cli.py" rename_workspace_entry --agent_id "research" --path "notes/todo.md" --new_path "notes/plan.md"
```

#### delete_workspace_entry — 删除工作区条目

```bash
python3 "{project_root}/agent/cli.py" delete_workspace_entry --agent_id "research" --path "notes/plan.md"
```

### Skill 管理

#### list_skills — 列出可用 Skill

```bash
python3 "{project_root}/agent/cli.py" list_skills
```

#### get_agent_skills — 读取成员 Skill 状态

```bash
python3 "{project_root}/agent/cli.py" get_agent_skills --agent_id "research"
```

#### install_skill — 安装 Skill

```bash
python3 "{project_root}/agent/cli.py" install_skill --agent_id "research" --skill_name "planner"
```

#### uninstall_skill — 卸载 Skill

```bash
python3 "{project_root}/agent/cli.py" uninstall_skill --agent_id "research" --skill_name "planner"
```

### 定时任务管理

#### list_scheduled_tasks — 列出定时任务

```bash
python3 "{project_root}/agent/cli.py" list_scheduled_tasks
python3 "{project_root}/agent/cli.py" list_scheduled_tasks --agent_id "research"
```

- 可选按 `agent_id` 过滤。
- 返回任务名称、调度规则、绑定会话、启用状态与下次执行时间。

#### create_scheduled_task — 创建定时任务

```bash
python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "晨间简报" \
  --agent-id "research" \
  --main \
  --instruction "整理今天要关注的 3 个重点" \
  --schedule-kind "every" \
  --interval-seconds 86400 \
  --timezone "Asia/Shanghai"

python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "会话内提醒" \
  --agent-id "research" \
  --session-key "agent:research:ws:dm:launcher-app-research" \
  --instruction "提醒我检查发版状态" \
  --schedule-kind "at" \
  --run-at "2026-04-20T09:00" \
  --timezone "Asia/Shanghai"

python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "工作日晨报" \
  --agent-id "research" \
  --named-session-key "morning-brief" \
  --instruction "输出工作日早间简报" \
  --schedule-kind "cron" \
  --cron-expression "0 8 * * 1-5" \
  --timezone "Asia/Shanghai"

python3 "{project_root}/agent/cli.py" create_scheduled_task \
  --name "隔离巡检" \
  --agent-id "research" \
  --isolated \
  --instruction "执行一次独立巡检并写出摘要" \
  --schedule-kind "every" \
  --interval-seconds 1800 \
  --timezone "Asia/Shanghai"
```

- 会话目标支持 4 种：`main`、`bound`、`named`、`isolated`。
- `--main`：发送到目标 Agent 的主会话；可配 `--wake-mode now|next-heartbeat`。
- `--session-key <key>`：绑定到一个已有会话；如果只传这个参数，CLI 会自动推断为 `bound`。
- `--named-session-key <key>`：发送到命名会话；如果只传这个参数，CLI 会自动推断为 `named`，其中 `main` 是保留字，不能作为命名 key。
- `--isolated`：使用隔离自动化会话；如果不传任何会话目标参数，CLI 默认也是 `isolated`。
- `--session-target-kind bound|named|main|isolated`：可选显式指定目标类型；通常只在你想把模式表达得更清楚时再传。
- `--main`、`--isolated`、`--session-target-kind` 互斥；`--session-key` 与 `--named-session-key` 也不能同时传。
- `schedule-kind` 取值：`every / at / cron`
- `at` 模式请传未来时间；如果时间早于当前时间，任务不会再触发。

#### run_scheduled_task — 立即运行定时任务

```bash
python3 "{project_root}/agent/cli.py" run_scheduled_task --job-id "job_xxx"
```

#### enable_scheduled_task / disable_scheduled_task — 启用或禁用定时任务

```bash
python3 "{project_root}/agent/cli.py" enable_scheduled_task --job-id "job_xxx"
python3 "{project_root}/agent/cli.py" disable_scheduled_task --job-id "job_xxx"
```

#### get_scheduled_task_runs — 查看运行记录

```bash
python3 "{project_root}/agent/cli.py" get_scheduled_task_runs --job-id "job_xxx"
```

#### delete_scheduled_task — 删除定时任务

```bash
python3 "{project_root}/agent/cli.py" delete_scheduled_task --job-id "job_xxx"
```

## Workspace 规则

每个成员创建后自动分配独立工作空间，位于 `~/.nexus/workspace/<agent_slug>/`。

### 目录结构

```
<workspace>/
  .agents/skills/    # 内部技能目录（不可直接操作）
  .claude/           # Claude 配置目录（不可直接操作）
  memory/            # 按天日志、摘要、调研片段和记忆资产
  AGENTS.md          # Agent 身份与行为规则
  USER.md            # 用户偏好
  MEMORY.md          # 跨会话持久记忆
  RUNBOOK.md         # 运维手册与任务清单
```

### 文件操作约束

- **受保护目录**：`.agents/`、`.claude/` 禁止直接读写，属于内部运行时目录。
- **路径安全**：不允许路径穿越（`../`），所有操作限定在工作空间根目录内。
- **命名文件**：`AGENTS.md`、`USER.md`、`MEMORY.md`、`RUNBOOK.md` 可通过名称直接读写，也可通过相对路径操作。
- **memory/ 目录**：统一用于按天日志、摘要和资产文件，通过 `save_memory_file` 写入。
- **文件大小限制**：实时快照推送上限 128KB，超出部分不推送。

### 模板初始化规则

- 创建成员时自动初始化目录结构和模板文件。
- 已存在的文件不会被覆盖，保证用户修改不丢失。
- 主智能体和普通成员使用不同的模板（主智能体模板包含系统级职责定义）。

### 技能部署

- 基础 skill 与主智能体专属 skill 由系统管理，不能手动卸载。
- 普通 skill 可通过 `install_skill` / `uninstall_skill` 管理。
- 技能部署到 `.agents/skills/<skill_name>/`。
- `.claude/skills/<skill_name>` 是指向 `.agents/skills/` 的相对符号链接。

## 操作流程

1. 查询结构：`list_agents` / `get_room` / `get_room_contexts`
2. 管理成员：`validate_agent_name` → `create_agent` / `delete_agent`
3. 管理协作：`create_room` / `update_room` / `add_room_member` / `remove_room_member` / `delete_room`
4. 管理工作区：`list_workspace_files` → `read_workspace_file` → `update_workspace_file`
5. 管理技能：`list_skills` → `get_agent_skills` → `install_skill` / `uninstall_skill`
6. 管理定时任务：`list_scheduled_tasks` → `create_scheduled_task` → `enable_scheduled_task` / `disable_scheduled_task` / `run_scheduled_task` / `get_scheduled_task_runs` / `delete_scheduled_task`

## 使用规则

- **主智能体不能作为 Room 成员**，创建 Room 时不要把主智能体的 `agent_id` 放进 `agent_ids`。
- 创建成员前，先 `validate_agent_name` 再 `create_agent`，名称不通过时告知用户原因。
- 创建多人 Room 时，先向用户确认成员列表，再执行创建。
- 涉及文件修改时，先读再写；对路径和覆盖范围说清楚。
- 涉及删除成员、删除房间、删除文件、卸载技能时，默认先确认影响范围。
- 涉及创建定时任务时，先明确三件事：目标智能体、会话目标（`main / bound / named / isolated`）、调度规则，再执行命令。
- 工具统一返回 JSON：先检查 `ok` 字段，为 `true` 时读 `data`，为 `false` 时读 `error` 并直接告知用户。
- 工具执行失败时不要假装成功，根据 `error` 内容给出明确反馈。
