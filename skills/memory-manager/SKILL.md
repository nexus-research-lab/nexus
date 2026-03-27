---
name: memory-manager
description: 管理和检索 Agent 的长期记忆（MEMORY.md）和日记（diary/）。当需要查找过去的信息、回顾决策、或者在重要任务前进行自我提升时，使用此 skill。
---

# memory-manager

负责管理 Agent 的记忆系统。通过分层存储和检索工具，平衡上下文长度与记忆深度。

CLI 工具路径：`python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}"`

## 记忆分层规则

- **MEMORY.md**：跨会话的长期、高信号记忆（用户偏好、重大决策、项目里程碑）。
- **diary/YYYY-MM-DD.md**：每日进展、错误记录、即时感悟（建议每天创建一个）。
- **memory/**：存储会话的历史摘要或详细资产。

## 检索工具参考

### search — 语义/关键词检索

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" search --query "用户偏好 饮食"
```

- 返回匹配的文件路径、行号和内容。
- 建议在回复涉及“以前”、“上次”、“记得吗”等字眼时，先检索再回答。

### get — 获取文件片段

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" get --path "diary/2026-03-27.md" --from_line 1 --lines 50
```

- 当 `search` 给出的片段不够完整时，使用 `get` 获取上下文内容。

## 自改进机制（Self-Improvement）

- **当任务失败或被用户纠正时**：主动将原因和纠正措施写入今日日记 `diary/YYYY-MM-DD.md`。
- **当积累了重要经验时**：将提炼后的结论更新到 `MEMORY.md`。
- **在执行重大决策前**：建议先 `search` 相关关键词，避免重复错误。

## 约束

- `MEMORY.md` 注入 context 的部分应保持精简（通常由系统自动截取前 1000 字符）。
- 详细信息应存在 `diary/` 或 `memory/` 中，通过检索获取。
