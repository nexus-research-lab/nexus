---
name: nexus-manager
description: 管理 Nexus 的用户账号、Agent、Room、Workspace 与 Skill 系统操作。当用户提到注册或创建 Nexus 账号、列出用户、重置密码、创建 agent、创建 room、邀请成员、查看 room、读写工作区文件、安装或卸载 skill、删除成员或房间、查询系统协作结构时，使用此 skill，即使没有明确说“管理”二字。
---

# nexus-manager

管理 Nexus 平台的用户账号、Agent、Room、Workspace 与 Skill。

## 执行契约

- 主智能体直接调用宿主注入的 `"$NEXUSCTL_COMMAND_PATH" --json ...`。只有入口变量没有注入时才使用 `nexusctl`。
- 宿主注入的当前 owner 与 Agent workspace 是唯一作用域来源。命令前不要拼接环境变量，不要自行选择作用域，不要直接读写数据库、state 或 runtime 目录。
- 普通 Agent 在 `runtime isolation=enforce` 下不能调用控制面；改用对应的内置 Nexus 工具/宿主 API，或明确报告当前部署不可用。不要通过源码搜索、`go run` 或改环境变量绕过。
- 所有控制面调用都要求 `--json`。先检查 `success`，再读取 `item`/`items`；失败时读取 `error`、`message` 与退出码。不要从旧版字段或 stdout 猜结果。
- 低频命令或参数以当前二进制的 `--help` 为准，不要凭旧文档补参数。

密码是一次性写入的敏感值：能提供标准输入时使用 `--password-stdin`；否则遵循用户给出的安全输入方式。最终回复、日志摘要和表格只报告用户名、`user_id`、角色、状态与结果，不复述密码。

## 常用命令

以下示例中的 `nexusctl` 代表上面的宿主注入入口；实际执行仍须加 `--json`。

### 用户账号

```bash
nexusctl --json auth status
nexusctl --json user list
nexusctl --json user create --username "alice" --display-name "Alice" --role member --password-stdin
nexusctl --json user reset-password --username "alice" --password-stdin
```

- 创建或重置前先用 `user list` 检查目标是否已存在。
- `user create` 与 `user reset-password` 的用户名和密码参数按当前 `--help` 选择；用户名与 user ID 不要同时猜测。

### Agent 与会话

```bash
nexusctl --json agent list
nexusctl --json agent create --name "Research"
nexusctl --json agent get research
nexusctl --json session list --agent-id research
```

### Room

```bash
nexusctl --json room list
nexusctl --json room get <room_id>
nexusctl --json room contexts <room_id>
nexusctl --json room create --agent-id research --agent-id writer --name "内容团队" --title "Kickoff"
nexusctl --json room update <room_id> --name "内容团队" --title "本周计划"
nexusctl --json room add-member <room_id> --agent-id translator
nexusctl --json room remove-member <room_id> --agent-id translator
nexusctl --json room delete <room_id>
```

- `room create` 的 `--agent-id` 可重复传入。主智能体不能作为 Room 成员。
- 删除 Room、成员或其他资源前，先确认影响范围。
- Room 内的定向消息使用 `nexus_room.send_directed_message`，不要把它伪装成 CLI 命令。runtime 会注入 `room_id`、`conversation_id` 和 `source_agent_id`；`recipients` 使用成员的 `agent_id`。

### Workspace

```bash
nexusctl --json workspace list --agent-id research
nexusctl --json workspace get --agent-id research --path "RUNBOOK.md"
nexusctl --json workspace update --agent-id research --path "RUNBOOK.md" --content "# 新计划"
nexusctl --json workspace create --agent-id research --path "notes/todo.md" --type file --content "- kickoff"
nexusctl --json workspace rename --agent-id research --path "notes/todo.md" --new-path "notes/plan.md"
nexusctl --json workspace delete --agent-id research --path "notes/plan.md"
```

- 路径必须是 workspace 内的相对路径，不允许路径穿越。
- 修改前先读取现有内容，明确覆盖范围；删除前先确认。

### Skill

```bash
nexusctl --json skill list
nexusctl --json skill agent-list --agent-id research
nexusctl --json skill install --agent-id research --skill-name planner
nexusctl --json skill uninstall --agent-id research --skill-name planner
nexusctl --json skill search-external "pdf"
nexusctl --json skill import-git --url https://github.com/example/skills --path skills/demo
nexusctl --json skill install-external --agent-id research --item-file search-result.json
nexusctl --json skill update demo-skill
nexusctl --json skill update --all
```

- 平台内置 Skill 由平台库管理；只有 workspace-local 或外部 Skill 才涉及文件导入。
- 外部 Skill 通常先 `search-external`，再按搜索结果执行 `install-external`；参数细节以 `--help` 和结果 JSON 为准。
- 基础 Skill 不要手动卸载。卸载前确认 Agent 与 Skill 名称。

## 操作顺序

1. 先读取当前状态：账号用 `auth status`/`user list`，成员用 `agent list`，Room 用 `room list`，Skill 用 `skill list`。
2. 再执行单个变更；变更后重新读取结果，确认 `success` 和返回 ID。
3. 任何失败都如实报告，不要用直接改文件、数据库或环境变量的方式补救。
