# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_interactive.py
# @Date   ：2026/04/03 10:33
# @Author ：leemysw
# 2026/04/03 10:33   Create
# =====================================================

"""交互式权限策略。"""

from typing import Any

from claude_agent_sdk import PermissionResult, ToolPermissionContext

from agent.service.channels.message_sender import MessageSender
from agent.service.permission.permission_route_context import PermissionRouteContext
from agent.service.permission.permission_runtime_context import permission_runtime_context
from agent.service.permission.strategy.permission_strategy import PermissionStrategy


class InteractivePermissionStrategy(PermissionStrategy):
    """把 WebSocket 连接接到全局权限运行时上下文。"""

    def __init__(self, sender: MessageSender):
        self.sender = sender

    async def request_permission(
        self,
        session_key: str,
        tool_name: str,
        input_data: dict[str, Any],
        context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """委托全局上下文处理权限确认。"""
        return await permission_runtime_context.request_permission(
            session_key=session_key,
            tool_name=tool_name,
            input_data=input_data,
            context=context,
        )

    def handle_permission_response(self, message: dict[str, Any]) -> bool:
        """处理前端权限确认结果。"""
        return permission_runtime_context.handle_permission_response(message)

    def bind_session_route(
        self,
        session_key: str,
        route_context: PermissionRouteContext,
    ) -> None:
        """绑定运行时 session 的前端路由。"""
        permission_runtime_context.bind_session_route(session_key, route_context)

    def unbind_session_route(self, session_key: str) -> None:
        """移除运行时 session 的前端路由。"""
        permission_runtime_context.unbind_session_route(session_key)

    def cancel_requests_for_session(
        self,
        session_key: str,
        message: str = "Permission request cancelled",
    ) -> int:
        """取消指定 session 的待确认权限请求。"""
        return permission_runtime_context.cancel_requests_for_session(
            session_key=session_key,
            message=message,
        )

    def has_pending_request_for_session(self, session_key: str) -> bool:
        """判断指定 session 是否仍有待确认请求。"""
        return permission_runtime_context.has_pending_request_for_session(session_key)

    def close(self) -> None:
        """连接关闭时，权限状态由运行时上下文继续托管。"""
        return None

    @classmethod
    def register_session_sender(
        cls,
        session_key: str,
        sender: MessageSender,
        client_id: str,
        request_control: bool,
    ) -> dict[str, Any]:
        """绑定某个前端 session 的发送通道。"""
        return permission_runtime_context.bind_session_sender(
            session_key=session_key,
            sender=sender,
            client_id=client_id,
            request_control=request_control,
        )

    @classmethod
    def unregister_sender(cls, sender: MessageSender) -> tuple[str, ...]:
        """注销某个连接持有的全部前端 session。"""
        return permission_runtime_context.unregister_sender(sender)

    @classmethod
    def unregister_session_sender(
        cls,
        session_key: str,
        sender: MessageSender,
    ) -> dict[str, Any]:
        """注销某个连接持有的单个前端 session。"""
        return permission_runtime_context.unbind_session_sender(session_key, sender)
