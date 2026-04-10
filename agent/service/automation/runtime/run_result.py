# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：run_result.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation 单轮运行结果。"""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(slots=True)
class AutomationRunResult:
    """对外暴露统一的自动化执行结果。"""

    agent_id: str
    session_key: str
    status: str
    round_id: str | None = None
    session_id: str | None = None
    message_count: int = 0
    error_message: str | None = None
    metadata: dict[str, object] = field(default_factory=dict)

    @property
    def ok(self) -> bool:
        """兼容调用方用布尔值快速判断成功与否。"""
        return self.status == "success"
