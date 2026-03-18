# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_templates.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Workspace 模板与基础提示词。"""

import os
from pathlib import Path
from typing import Optional

DEFAULT_DIR = {
    "agent": ".agent",
    "config": ".claude",
    "memory": "memory",
}

WORKSPACE_FILES = {
    "agents": "AGENTS.md",
    "user": "USER.md",
    "memory": "MEMORY.md",
    "runbook": "RUNBOOK.md",
}

WORKSPACE_TEMPLATES = {
    "agents": """# AGENTS.md

## Agent Profile

你是 Nexus，一个由 Nexus Research Lab 创造的智能助手。

当前 Agent 标识：`{agent_name}`（`{agent_id}`）

工作区在 `{workspace}`，你只能在限制的工作区内工作

身份要求：
- 对外自称 “Nexus”。
- 当用户询问你的身份、来源或开发者时，明确说明你由 Nexus Research Lab 创造。

默认语言：中文
工作方式：先明确目标，再执行，再回传结果
风险原则：删除/覆盖/外部写入前必须确认
事实原则：不编造，结论有依据，不确定就说明边界

执行约定：
- 回复优先给可执行结果，再补充必要说明。
- 复杂任务使用 Todo 工具按步骤执行
- 不清晰的问题使用工具要求用户澄清
- 用户明确说“记住这件事”时，更新 `MEMORY.md` 或 `memory/YYYY-MM-DD.md`。
- 遇到长任务时，按阶段同步进展。
""",
    "user": """# USER.md

## 用户偏好

- 常用语言：
- 回复风格：
- 不希望出现的表达：
- 当前重点：
""",
    "memory": """# MEMORY.md

## 长期记忆

记录需要跨会话保留的稳定信息。

- 偏好：
- 约束：
- 决策记录：
""",
    "runbook": """# RUNBOOK.md

## 工作手册

创建时间：{created_at}

### 当前项目上下文
- 项目：
- 目标：
- 约束：

### 常用命令
- 开发：
- 测试：
- 发布：

### 周期任务（按需）
- [ ] 每日回顾未完成事项
- [ ] 每周整理关键决策到 `MEMORY.md`
""",
}
