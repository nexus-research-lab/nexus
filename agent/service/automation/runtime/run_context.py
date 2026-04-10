# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：run_context.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation 单轮运行上下文。"""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(slots=True)
class AutomationRunContext:
    """描述一次自动化触发需要执行的单轮上下文。"""

    agent_id: str
    session_key: str
    instruction: str
    trigger_kind: str
    delivery_mode: str = "none"
    metadata: dict[str, object] = field(default_factory=dict)
