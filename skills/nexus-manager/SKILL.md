---
name: nexus-manager
description: 管理 Nexus 的 Agent、Room、Workspace 与 Skill 系统操作。当用户提到创建 agent、创建 room、邀请成员、查看 room、读写工作区文件、安装或卸载 skill、删除成员或房间、查询系统协作结构时，使用此 skill，即使没有明确说“管理”二字。
---

# nexus-manager

管理 Nexus 平台的 Agent、Room、Workspace 与 Skill。通过 Nexus 控制面工具执行系统操作。

CLI 工具：优先使用环境变量 `NEXUSCTL_COMMAND_PATH` 指向的命令；示例里的 `nexusctl` 只是简写。
不要搜索 `cmd/nexusctl`，也不要手写 `go run ./cmd/nexusctl`，运行时入口已经注入。

运行时目录约定：

- `NEXUS_CONFIG_DIR` 与 `CLAUDE_CONFIG_DIR` 始终是当前 owner 的
  `~/.nexus/users/<owner>/runtime`，不是宿主的 `.nexus` 根。
- `NEXUSCTL_WORKSPACE_PATH` 是当前 Agent workspace 或其子目录。
  `nexusctl` 启动时会根据这两个受宿主注入的路径解析
  `.nexus/app/data/nexus.db` 与用户 workspace 基址；不要自行设置
  `NEXUS_STATE_ROOT`、`WORKSPACE_PATH`，也不要通过路径拼接访问其他 owner。
- `app/` 与 `users/<owner>/state/rooms/` 是宿主控制面目录，runtime 不应直接读写；
  workspace 文件通过本 Skill 的命令操作。

如果部署启用了 Linux `runtime isolation=enforce`，普通 Agent runtime 内的
`nexusctl` 控制面调用会被拒绝。不要通过 `go run`、搜索源码入口或修改环境变量绕过；
普通 Agent 改用对应的内置 Nexus 工具/宿主 API，或把该操作报告为当前部署不可用。
Nexus 主智能体属于宿主控制面主体，可以使用宿主注入的
`"$NEXUSCTL_COMMAND_PATH"`；仍必须保持当前 owner scope，不得使用
`--global-scope`、`--scope-user-id` 或覆盖 `NEXUSCTL_USER_ID`。

## CLI 输出约定

- Agent 正常调用统一加 `--json`，让 stdout 始终返回单行 JSON，便于直接提字段。
- `stdout` 只读数据，`stderr` 只读诊断；失败时不要从 stdout 猜错误。
- 参数用法错误返回 `64`，执行错误返回 `1`。遇到 `64` 先修参数，遇到 `1` 再判断是否重试或换方案。
- 默认不要加 `--verbose`，只有在排查异常、确认 skill 部署过程或追踪系统初始化问题时才显式打开。
- 子命令按领域拆分：`agent`、`room`、`conversation`、`workspace`、`skill`、`launcher`。
- 失败时优先读 stderr 里的 JSON 错误，不要假设命令名仍旧是旧 Python 风格。
- 成功响应通常包含 `success: true` 以及 `item` 或 `items`；失败响应读取
  `error`、`message` 和退出码，不要按旧版 `ok/data` 字段猜测结果。
- 多用户部署下不要把 `user_id` 写进命令模板；运行时会自动注入当前用户作用域。只有手工在终端直跑 `nexusctl` 时，才需要显式传 `--scope-user-id` 或设置 `NEXUSCTL_USER_ID`。

```bash
# Agent 正常调用
nexusctl --json agent list

# 排查问题时再打开诊断
nexusctl --json --verbose agent list

# 人工查看时使用格式化输出
nexusctl --pretty agent list
```

## 核心概念

- **Agent（成员）**：具有独立工作空间的智能体，可被邀请加入 Room 协作。
- **Room（群组空间）**：多个 Agent 共处的对话空间，支持创建后追加成员。
- **Workspace（工作区）**：每个 Agent 独立拥有的文件空间，可读写业务文件与运行资料。
- **Skill（技能）**：平台内置 Skill 由全局平台库提供，Agent 只记录启用的 ID；外部或 workspace-local Skill 才可能以文件形式部署到 Agent 工作区。
- **主智能体**：系统内置的保留 Agent，不能作为 Room 成员，所有 Room 操作由它发起。
- 每个成员创建后自动获得独立工作空间（workspace），用于存放工具配置、记忆和业务文件；不要把平台内置 Skill 当作 workspace 文件维护。

## 命令参考

### Agent 管理

#### 列出成员

```bash
nexusctl agent list
```

#### 创建成员

```bash
nexusctl agent create --name "Research"
```

#### 读取成员详情

```bash
nexusctl agent get research
```

#### 读取成员会话

```bash
nexusctl session list --agent-id research
```

### Room 管理

#### 查看 Room 列表

```bash
nexusctl room list
```

#### 读取 Room

```bash
nexusctl room get abc123
```

#### 读取 Room 上下文

```bash
nexusctl room contexts abc123
```

#### 创建 Room

```bash
nexusctl room create --agent-id research --agent-id writer --name "内容团队" --title "Kickoff" --description "内容生产协作空间"
```

#### 更新 Room

```bash
nexusctl room update abc123 --name "内容团队" --title "本周计划"
```

#### 向 Room 追加成员

```bash
nexusctl room add-member abc123 --agent-id translator
```

- Room ID 使用第一个位置参数，成员使用必填的 `--agent-id`。
- 仅支持群组类型 Room（`room`），不支持私聊（`dm`）。
- 成功结果位于 `item`，其中包含 Room、conversation 与成员状态。

#### 移除 Room 成员

```bash
nexusctl room remove-member abc123 --agent-id translator
```

#### 创建 Room directed message

Room runtime 内会自动注入当前 `room_id`、`conversation_id` 和 `source_agent_id`；Agent 正常调用时不要额外传这些字段。`recipients` 填 Room 成员的 `agent_id`，不是成员名。

```json
{"tool":"nexus_room.send_directed_message","arguments":{"recipients":["0ed5434a8c13"],"wake_policy":"none","reply_route":{"mode":"none"},"content":"私下提醒"}}
{"tool":"nexus_room.send_directed_message","arguments":{"recipients":["0ed5434a8c13"],"wake_policy":"immediate","reply_route":{"mode":"private","recipients":["<host-agent-id>"],"wake_policy":"immediate"},"content":"请处理后私下回给主持人"}}
{"tool":"nexus_room.send_directed_message","arguments":{"recipients":["0ed5434a8c13"],"wake_policy":"immediate","reply_route":{"mode":"private","recipients":["<host-agent-id>"],"wake_policy":"immediate","next_reply_route":{"mode":"public"}},"content":"请私下回答主持人，主持人随后公开推进"}}
{"tool":"nexus_room.send_directed_message","arguments":{"recipients":["<self-agent-id>"],"wake_policy":"none","reply_route":{"mode":"none"},"content":"只写给自己的上下文"}}
```

#### 删除 Room

```bash
nexusctl room delete abc123
```

### Workspace 操作

#### 列出工作区文件

```bash
nexusctl workspace list --agent-id research
```

#### 读取工作区文件

```bash
nexusctl workspace get --agent-id research --path "RUNBOOK.md"
```

#### 更新工作区文件

```bash
nexusctl workspace update --agent-id research --path "RUNBOOK.md" --content "# 新计划"
```

#### 创建工作区条目

```bash
nexusctl workspace create --agent-id research --path "notes/todo.md" --type file --content "- kickoff"
nexusctl workspace create --agent-id research --path "notes" --type directory
```

#### 重命名工作区条目

```bash
nexusctl workspace rename --agent-id research --path "notes/todo.md" --new-path "notes/plan.md"
```

#### 删除工作区条目

```bash
nexusctl workspace delete --agent-id research --path "notes/plan.md"
```

### Skill 管理

#### 列出 Skill

```bash
nexusctl skill list
```

#### 读取成员 Skill 状态

```bash
nexusctl skill agent-list --agent-id research
```

#### 安装 Skill

```bash
nexusctl skill install --agent-id research --skill-name planner
```

#### 搜索社区 Skill

```bash
nexusctl skill search-external "pdf"
```

#### 从 Git 导入 Skill

```bash
nexusctl skill import-git --url https://github.com/example/skills --path skills/demo
```

#### 导入并安装外部 Skill

```bash
nexusctl skill install-external --agent-id research --item-file search-result.json
nexusctl skill install-external --agent-id research --import-mode skills_sh --package-spec owner/repo/skill --skill-slug skill
```

#### 更新导入 Skill

```bash
nexusctl skill update demo-skill
nexusctl skill update --all
```

#### 卸载 Skill

```bash
nexusctl skill uninstall --agent-id research --skill-name planner
```

## Workspace 规则

每个成员创建后自动分配独立工作空间。系统用户位于 `~/.nexus/users/__system__/workspace/<agent_slug>/`；登录用户位于 `~/.nexus/users/<user_id>/workspace/<agent_slug>/`。

### 目录结构

```
~/.nexus/
  app/                              # 宿主控制面：数据库、配置、日志（runtime 不直接访问）
  users/<owner>/
    workspace/<agent_slug>/         # Agent 可工作的业务目录
    runtime/                        # NEXUS_CONFIG_DIR 与 CLAUDE_CONFIG_DIR
      projects/                     # nxs/Claude transcript
      home/ cache/ logs/ tmp/       # runtime 私有运行时文件
    state/rooms/                    # 宿主托管 Room ledger（runtime 不直接访问）

<workspace>/
  AGENTS.md          # Agent 身份与行为规则
  USER.md            # 用户偏好
  RUNBOOK.md         # 运维手册与任务清单
```

### 文件操作约束

- **受保护目录**：`.agents/`、`.claude/` 属于内部运行时目录，不要把它们当作普通
  workspace 文档直接维护；平台或导入 Skill 的绑定通过 `skill` 命令管理。
- **路径安全**：不允许路径穿越（`../`），所有操作限定在工作空间根目录内。
- **命名文件**：`AGENTS.md`、`USER.md`、`RUNBOOK.md` 可通过名称直接读写，也可通过相对路径操作。
- **文件大小限制**：实时快照推送上限 128KB，超出部分不推送。

### 模板初始化规则

- 创建成员时自动初始化目录结构和模板文件。
- 已存在的文件不会被覆盖，保证用户修改不丢失。
- 主智能体和普通成员使用不同的模板（主智能体模板包含系统级职责定义）。

### 技能部署

- 基础 skill 与主智能体专属 skill 由系统管理，不能手动卸载。
- 普通 skill 可通过 `skill install` / `skill uninstall` 管理。
- 平台内置 skill 由全局平台库统一提供；安装到 Agent 只记录 `options.skill_ids`，不会复制一份到 Agent workspace。
- nxs 与 Claude 通过同一个平台兼容根读取平台库：根下分别有 `.agents/skills/<skill_id>/` 和 `.claude/skills/<skill_id>/` 两个入口。
- 外部 skill 仍可导入后安装到指定 Agent；外部安装态在兼容旧版的 workspace 文件中维护，平台内置 skill 不要直接改全局目录。
- 只有 workspace-local skill 才属于 workspace 的 `.agents/skills/` / `.claude/skills/` 文件范围。

## 操作流程

1. 查询结构：`agent list` / `room get` / `room contexts`
2. 管理成员：`agent create` / `agent get`
3. 管理协作：`room create` / `room update` / `room add-member` / `room remove-member` / `room delete`
4. 管理工作区：`workspace list` → `workspace get` → `workspace update`
5. 管理技能：`skill list` → `skill agent-list` → `skill install` / `skill uninstall`；
   外部来源使用 `skill search-external` → `skill install-external`

## 使用规则

- **主智能体不能作为 Room 成员**，创建 Room 时不要把主智能体的 `agent_id`
  传给任何 `--agent-id`。
- 创建成员前先读取 `agent list` 确认当前作用域，随后使用 `agent create`；
  创建失败时读取 CLI 错误并告知用户原因。
- 创建多人 Room 时，先向用户确认成员列表，再执行创建。
- 涉及文件修改时，先读再写；对路径和覆盖范围说清楚。
- 涉及删除成员、删除房间、删除文件、卸载技能时，默认先确认影响范围。
- 工具统一返回 JSON：先检查 `success` 字段，为 `true` 时读 `item` 或 `items`，
  为 `false` 时读 `error`、`message` 并直接告知用户。
- 工具执行失败时不要假装成功，根据 `error` 内容给出明确反馈。
