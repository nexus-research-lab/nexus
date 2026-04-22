---
name: nexus-manager
description: 管理 Nexus 的 Agent、Room、Workspace 与 Skill 系统操作。当用户提到创建 agent、创建 room、邀请成员、查看 room、读写工作区文件、安装或卸载 skill、删除成员或房间、查询系统协作结构时，使用此 skill，即使没有明确说“管理”二字。
---

# nexus-manager

管理 Nexus 平台的 Agent、Room、Workspace 与 Skill。通过 CLI 工具执行系统操作。

CLI 工具路径：`go run "{project_root}/cmd/nexusctl"`

## CLI 输出约定

- `nexusctl` 默认输出 JSON，可以直接作为推理上下文输入。
- 子命令按领域拆分：`agent`、`room`、`conversation`、`workspace`、`skill`、`launcher`。
- 失败时优先读 JSON 中的错误，不要假设命令名仍旧是旧 Python 风格。
- 多用户部署下不要把 `user_id` 写进命令模板；运行时会自动注入当前用户作用域。只有手工在终端直跑 `nexusctl` 时，才需要显式传 `--scope-user-id` 或设置 `NEXUSCTL_USER_ID`。

## 核心概念

- **Agent（成员）**：具有独立工作空间的智能体，可被邀请加入 Room 协作。
- **Room（群组空间）**：多个 Agent 共处的对话空间，支持创建后追加成员。
- **Workspace（工作区）**：每个 Agent 独立拥有的文件空间，可读写业务文件与记忆文件。
- **Skill（技能）**：部署到 Agent 工作区中的能力包，决定其可用专业动作。
- **主智能体**：系统内置的保留 Agent，不能作为 Room 成员，所有 Room 操作由它发起。
- 每个成员创建后自动获得独立工作空间（workspace），用于存放技能、工具配置和文件。

## 命令参考

### Agent 管理

#### 列出成员

```bash
go run "{project_root}/cmd/nexusctl" agent list
```

#### 创建成员

```bash
go run "{project_root}/cmd/nexusctl" agent create --name "Research"
```

#### 读取成员详情

```bash
go run "{project_root}/cmd/nexusctl" agent get research
```

#### 读取成员会话

```bash
go run "{project_root}/cmd/nexusctl" session list --agent-id research
```

### Room 管理

#### 查看 Room 列表

```bash
go run "{project_root}/cmd/nexusctl" room list
```

#### 读取 Room

```bash
go run "{project_root}/cmd/nexusctl" room get abc123
```

#### 读取 Room 上下文

```bash
go run "{project_root}/cmd/nexusctl" room contexts abc123
```

#### 创建 Room

```bash
go run "{project_root}/cmd/nexusctl" room create --agent-id research --agent-id writer --name "内容团队" --title "Kickoff" --description "内容生产协作空间"
```

#### 更新 Room

```bash
go run "{project_root}/cmd/nexusctl" room update abc123 --name "内容团队" --title "本周计划"
```

#### 向 Room 追加成员

```bash
go run "{project_root}/cmd/nexusctl" room add-member abc123 --agent-id translator
```

- `--room_id` 和 `--agent_id` 均必填。
- 仅支持群组类型 Room（`room`），不支持私聊（`dm`）。
- 返回字段：`room_id`、`room_name`、`conversation_id`、`member_agent_ids`

#### 移除 Room 成员

```bash
go run "{project_root}/cmd/nexusctl" room remove-member abc123 --agent-id translator
```

#### 删除 Room

```bash
go run "{project_root}/cmd/nexusctl" room delete abc123
```

### Workspace 操作

#### 列出工作区文件

```bash
go run "{project_root}/cmd/nexusctl" workspace list --agent-id research
```

#### 读取工作区文件

```bash
go run "{project_root}/cmd/nexusctl" workspace get --agent-id research --path "RUNBOOK.md"
```

#### 更新工作区文件

```bash
go run "{project_root}/cmd/nexusctl" workspace update --agent-id research --path "RUNBOOK.md" --content "# 新计划"
```

#### 创建工作区条目

```bash
go run "{project_root}/cmd/nexusctl" workspace create --agent-id research --path "notes/todo.md" --type file --content "- kickoff"
go run "{project_root}/cmd/nexusctl" workspace create --agent-id research --path "notes" --type directory
```

#### 重命名工作区条目

```bash
go run "{project_root}/cmd/nexusctl" workspace rename --agent-id research --path "notes/todo.md" --new-path "notes/plan.md"
```

#### 删除工作区条目

```bash
go run "{project_root}/cmd/nexusctl" workspace delete --agent-id research --path "notes/plan.md"
```

### Skill 管理

#### 列出 Skill

```bash
go run "{project_root}/cmd/nexusctl" skill list
```

#### 读取成员 Skill 状态

```bash
go run "{project_root}/cmd/nexusctl" skill agent-list --agent-id research
```

#### 安装 Skill

```bash
go run "{project_root}/cmd/nexusctl" skill install --agent-id research --skill-name planner
```

#### 卸载 Skill

```bash
go run "{project_root}/cmd/nexusctl" skill uninstall --agent-id research --skill-name planner
```

## Workspace 规则

每个成员创建后自动分配独立工作空间。单用户模式位于 `~/.nexus/workspace/<agent_slug>/`；多用户模式位于 `~/.nexus/workspace/<user_id>/<agent_slug>/`。

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
- **memory/ 目录**：统一用于按天日志、摘要和资产文件，通过 `nexusctl memory` 维护。
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

1. 查询结构：`agent list` / `room get` / `room contexts`
2. 管理成员：`agent create` / `agent get`
3. 管理协作：`room create` / `room update` / `room add-member` / `room remove-member` / `room delete`
4. 管理工作区：`workspace list` → `workspace get` → `workspace update`
5. 管理技能：`list_skills` → `get_agent_skills` → `install_skill` / `uninstall_skill`

## 使用规则

- **主智能体不能作为 Room 成员**，创建 Room 时不要把主智能体的 `agent_id` 放进 `agent_ids`。
- 创建成员前，先 `validate_agent_name` 再 `create_agent`，名称不通过时告知用户原因。
- 创建多人 Room 时，先向用户确认成员列表，再执行创建。
- 涉及文件修改时，先读再写；对路径和覆盖范围说清楚。
- 涉及删除成员、删除房间、删除文件、卸载技能时，默认先确认影响范围。
- 工具统一返回 JSON：先检查 `ok` 字段，为 `true` 时读 `data`，为 `false` 时读 `error` 并直接告知用户。
- 工具执行失败时不要假装成功，根据 `error` 内容给出明确反馈。
