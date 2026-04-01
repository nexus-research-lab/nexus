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
from agent.service.permission.permission_route_context import PermissionRouteContext


class PermissionStrategy(ABC):
    """可插拔的工具权限决策策略。"""

    @abstractmethod
    async def request_permission(
        self,
        session_key: str,
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

    def bind_session_route(
        self,
        session_key: str,
        route_context: PermissionRouteContext,
    ) -> None:
        """为某个运行时 session 绑定前端路由上下文。"""
        del session_key, route_context

    def unbind_session_route(self, session_key: str) -> None:
        """移除运行时 session 的前端路由上下文。"""
        del session_key

    def cancel_requests_for_session(
        self,
        session_key: str,
        message: str = "Permission request cancelled",
    ) -> int:
        """取消指定 session 下仍在等待的权限请求。"""
        del session_key, message
        return 0
