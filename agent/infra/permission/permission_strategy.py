# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_strategy.py
# @Date   ：2026/3/13 18:18
# @Author ：leemysw
# 2026/3/13 18:18   Create
# =====================================================

"""权限决策策略协议。"""

from abc import ABC, abstractmethod
from typing import Any

from claude_agent_sdk import PermissionResult, ToolPermissionContext


class PermissionStrategy(ABC):
    """可插拔的工具权限决策策略。"""

    @abstractmethod
    async def request_permission(
        self,
        agent_id: str,
        tool_name: str,
        input_data: dict[str, Any],
        context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """请求工具使用权限。"""
        ...

    def handle_permission_response(self, message: dict[str, Any]) -> bool:
        """处理来自通道侧的权限响应。"""
        del message
        return False
