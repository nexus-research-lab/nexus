# main-agent-orchestration

当用户要求你创建 agent、创建 room、邀请成员、查看最近 room 或校验成员命名时，优先使用这个 skill，而不是只给口头建议。

## 可用命令

命令行工具路径：
`python3 "{project_root}/agent/cli.py"`

### 1. 列出当前成员
```bash
python3 "{project_root}/agent/cli.py" list_agents
```

### 2. 校验成员名称
```bash
python3 "{project_root}/agent/cli.py" validate_agent_name --name "Research"
```

### 3. 创建成员
```bash
python3 "{project_root}/agent/cli.py" create_agent --name "Research"
```

### 4. 查看最近 room
```bash
python3 "{project_root}/agent/cli.py" list_rooms --limit 10
```

### 5. 创建 room
```bash
python3 "{project_root}/agent/cli.py" create_room --agent_ids "agent_a,agent_b" --name "市场研究" --title "Kickoff"
```

### 6. 向已有 room 追加成员
```bash
python3 "{project_root}/agent/cli.py" add_room_member --room_id "room_id" --agent_id "agent_id"
```

## 使用规则

- `main` 不能作为 room 成员。
- 创建成员前，优先先校验名称。
- 创建多人 room 时，先确认成员列表，再创建。
- 工具返回 JSON，先读 `ok`，再读取 `data`。
- 工具执行失败时，不要假装成功，直接根据 `error` 给用户明确反馈。
