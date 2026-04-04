---
name: memory-manager
description: 管理和检索 Agent 的长期记忆（MEMORY.md）和日记（diary/）。当需要查找过去的信息、回顾决策、或者在重要任务前进行自我提升时，使用此 skill。
---

# memory-manager

负责管理 Agent 的记忆系统。它不只保存长期记忆，还负责把失败、纠正、需求和复盘沉淀到日记，再把稳定规则提升到长期文件。

`memory-manager` 只是使用入口；真正的记忆能力已经沉到内部模块 `agent.service.memory`。  
也就是说：

- **内部模块** 负责条目建模、相似归并、次数累计、状态流转、长期提升
- **skill** 负责告诉模型何时该查、何时该记、何时该提升

CLI 工具路径：`python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}"`

## 记忆分层规则

- **MEMORY.md**：跨会话的长期、高信号记忆（用户偏好、重大决策、项目里程碑）。
- **SOUL.md**：行为准则、沟通偏好、长期执行原则。
- **TOOLS.md**：工具坑点、命令经验、外部接口注意事项。
- **diary/YYYY-MM-DD.md**：每日进展、错误记录、即时感悟、自我复盘。
- **memory/**：存储任务摘要、调研片段、临时结论等资产文件，不承担按天记日志职责。

## 检索工具参考

### search — 语义/关键词检索

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" search --query "用户偏好 饮食"
```

- 返回匹配的文件路径、行号和内容。
- 建议在回复涉及“以前”、“上次”、“记得吗”、返工、排错时，先检索再回答。

### get — 获取文件片段

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" get --path "diary/2026-03-27.md" --from_line 1 --lines 50
```

- 当 `search` 给出的片段不够完整时，使用 `get` 获取上下文内容。

### review — 回顾近期日记

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" review --days 3 --limit 6
```

- 在复杂任务、返工任务、重大决策前先执行。
- 优先阅读与当前任务最接近的标题，再决定是否继续 `get` 深读。

## 记录机制

### log — 写入今日日记

记录日记时，统一使用四种类型：

- `LRN`：学到的新规则、用户纠正、更优做法
- `ERR`：命令失败、接口失败、工具异常
- `FEAT`：用户提出但当前没有的能力
- `REF`：任务完成后的自我复盘

学习条目示例：

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" log \
  --kind LRN \
  --category correction \
  --title "用户要求注释使用中文" \
  --field "详情=当前仓库要求所有非平凡注释使用中文" \
  --field "行动=后续新增注释全部使用中文" \
  --field "来源=user_feedback" \
  --field "标签=comment,style"
```

错误条目示例：

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" log \
  --kind ERR \
  --title "list_skills 因死引用失败" \
  --field "错误=No module named 'agent.infra.database.models.skill'" \
  --field "上下文=执行 agent/cli.py list_skills" \
  --field "修复=删除合并残留的 skill_sql_repository" \
  --field "可复现=yes"
```

### promote — 提升为长期规则

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" promote \
  --target soul \
  --title "回复风格" \
  --content "默认简洁直接，不写冗长导语"
```

### resolve — 标记已解决

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" resolve \
  --entry-id "LRN-20260404-010203-123456789" \
  --note "已经把规范写入 AGENTS.md"
```

### set-status — 更新条目状态

```bash
python3 "{project_root}/agent/memory_cli.py" --workspace "{workspace}" set-status \
  --entry-id "LRN-20260404-010203-123456789" \
  --status case_by_case \
  --note "只在当前仓库生效"
```

目标选择：

- `memory`：项目事实、长期约束、稳定决策
- `soul`：行为准则、沟通偏好、用户长期要求
- `tools`：工具坑点、命令经验、接口注意事项
- `agents`：流程规则、执行顺序、工作约定

## 生命周期规则

- 新条目会自动生成 `ID`
- 相似条目会自动补 `关联`
- 重复纠正会自动累计 `次数`
- 当纠正类学习累计到 `3` 次时，状态会变成 `needs_confirmation`
- 用户明确表达的长期偏好，不走累计，直接提升到长期文件

常见状态：

- `pending`：已记录，待后续处理
- `needs_confirmation`：重复出现，应该确认是否升格为长期规则
- `resolved`：问题已经处理完
- `promoted`：已经提升到长期文件
- `case_by_case`：不固化为长期规则，只按场景处理
- `archived`：历史归档，回顾时可忽略

## 自动提醒

- workspace 初始化时会自动写入 Claude hooks。
- 每次新任务开始时会提醒是否需要先回顾近期 diary。
- Bash 命令失败时会提醒是否要记录 `[ERR]` 条目。

## 约束

- 长期文件保持短、准、稳，不把大段过程直接堆进 `MEMORY.md`。
- 详细过程优先写到 `diary/`，摘要和附件放到 `memory/`，需要时再提升到长期文件。
- 用户明确表达的长期偏好，不需要累计三次，直接提升。
- 记忆模块是内部核心能力，不要把关键语义只写在 skill 文本里。
