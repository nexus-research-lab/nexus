---
name: nexus-manager
description: 管理 Nexus 的 Agent 成员和 Room（群组空间）。当用户提到创建 agent、创建 room、邀请成员加入 room、查看 room 列表、管理团队成员、多 agent 协作、校验 agent 名称、列出所有 agent 或往群组里加人时，使用此 skill，即使没有明确说"管理"二字。
---

# nexus-manager

管理 Nexus 平台的 Agent 成员和 Room 群组空间。通过 CLI 工具执行所有操作。

CLI 工具路径：`python3 "{project_root}/agent/cli.py"`

## 核心概念

- **Agent（成员）**：具有独立工作空间的智能体，可被邀请加入 Room 协作。
- **Room（群组空间）**：多个 Agent 共处的对话空间，支持创建后追加成员。
- **nexus**：系统内置的主 Agent，不能作为 Room 成员，所有 Room 操作由它发起。
- 每个成员创建后自动获得独立工作空间（workspace），用于存放技能、工具配置和文件。

## 命令参考

### list_agents — 列出所有成员

```bash
python3 "{project_root}/agent/cli.py" list_agents
python3 "{project_root}/agent/cli.py" list_agents --include_main
```

- 默认不包含 main agent；加 `--include_main` 可包含。
- 返回字段：`agent_id`、`name`、`status`、`workspace_path`、`model`、`skills_enabled`

### validate_agent_name — 校验成员名称

```bash
python3 "{project_root}/agent/cli.py" validate_agent_name --name "Research"
```

- 创建成员前应先校验名称，避免冲突或非法字符。
- 返回校验结果，包含是否通过及原因。

### create_agent — 创建成员

```bash
python3 "{project_root}/agent/cli.py" create_agent --name "Research"
python3 "{project_root}/agent/cli.py" create_agent --name "Research" --model "glm-5"
```

- `--name` 必填；`--model` 可选，不指定则使用默认模型。
- 返回字段：`agent_id`、`name`、`workspace_path`、`model`、`skills_enabled`、`status`

### list_rooms — 查看最近 Room 列表

```bash
python3 "{project_root}/agent/cli.py" list_rooms
python3 "{project_root}/agent/cli.py" list_rooms --limit 10
```

- 返回字段：`room_id`、`room_type`、`name`、`description`、`member_agent_ids`、`updated_at`

### create_room — 创建 Room

```bash
python3 "{project_root}/agent/cli.py" create_room --agent_ids "research,writer" --name "内容团队" --title "Kickoff" --description "内容生产协作空间"
```

- `--agent_ids` 必填，逗号分隔的成员 ID 列表。
- `--name`、`--title`、`--description` 可选，不指定则自动生成。
- 单成员时类型为 `dm`（私聊），多成员时类型为 `room`（群组）。
- 返回字段：`room_id`、`room_type`、`room_name`、`conversation_id`、`conversation_title`、`member_agent_ids`

### add_room_member — 向 Room 追加成员

```bash
python3 "{project_root}/agent/cli.py" add_room_member --room_id "abc123" --agent_id "translator"
```

- `--room_id` 和 `--agent_id` 均必填。
- 仅支持群组类型 Room（`room`），不支持私聊（`dm`）。
- 返回字段：`room_id`、`room_name`、`conversation_id`、`member_agent_ids`

## Workspace 规则

每个成员创建后自动分配独立工作空间，位于 `~/.nexus-core/workspace/<agent_slug>/`。

### 目录结构

```
<workspace>/
  .agents/skills/    # 内部技能目录（不可直接操作）
  .claude/           # Claude 配置目录（不可直接操作）
  memory/            # 持久化记忆文件
  AGENTS.md          # Agent 身份与行为规则
  USER.md            # 用户偏好
  MEMORY.md          # 跨会话持久记忆
  RUNBOOK.md         # 运维手册与任务清单
```

### 文件操作约束

- **受保护目录**：`.agents/`、`.claude/` 禁止直接读写，属于内部运行时目录。
- **路径安全**：不允许路径穿越（`../`），所有操作限定在工作空间根目录内。
- **命名文件**：`AGENTS.md`、`USER.md`、`MEMORY.md`、`RUNBOOK.md` 可通过名称直接读写，也可通过相对路径操作。
- **memory/ 目录**：用于存放会话记忆，通过 `save_memory` 写入。
- **文件大小限制**：实时快照推送上限 128KB，超出部分不推送。

### 模板初始化规则

- 创建成员时自动初始化目录结构和模板文件。
- 已存在的文件不会被覆盖，保证用户修改不丢失。
- main agent 和普通成员使用不同的模板（main 的模板包含系统级职责定义）。

### 技能部署

- 技能仅部署给 main agent，部署到 `.agents/skills/<skill_name>/`。
- `.claude/skills/<skill_name>` 是指向 `.agents/skills/` 的相对符号链接。
- 新增技能后重新部署即可生效，已有技能自动跳过。

## 操作流程

1. 查看可用成员 → `list_agents`
2. 校验新成员名称 → `validate_agent_name`
3. 创建成员 → `create_agent`
4. 创建 Room 时确认成员列表后再创建 → `create_room`
5. 需要时向已有 Room 追加成员 → `add_room_member`

## 使用规则

- **main 不能作为 Room 成员**，创建 Room 时不要把 main 放进 `agent_ids`。
- 创建成员前，先 `validate_agent_name` 再 `create_agent`，名称不通过时告知用户原因。
- 创建多人 Room 时，先向用户确认成员列表，再执行创建。
- 工具统一返回 JSON：先检查 `ok` 字段，为 `true` 时读 `data`，为 `false` 时读 `error` 并直接告知用户。
- 工具执行失败时不要假装成功，根据 `error` 内容给出明确反馈。
