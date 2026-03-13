# session/ — 会话路由

L2 | 父级: agent/service/CLAUDE.md

## 成员清单

- `__init__.py`: 模块入口，导出 `build_session_key` / `parse_session_key` / `resolve_session`
- `session_router.py`: Session Key 构建（确定性路由）、解析、会话解析（查找或创建）

## 架构

```
session_key = agent:<agentId>:<channel>:<chatType>:<ref>[:topic:<threadId>]
```

- `build_session_key()` — 纯函数，无副作用，确定性构建路由键
- `parse_session_key()` — 反向解析回结构化字段
- `resolve_session()` — 查找活跃会话或创建新会话

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
