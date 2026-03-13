# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：dispatcher.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""通道消息分发器。"""

from typing import Any, Dict

from agent.channels.ws.handlers.error_handler import ErrorHandler
from agent.channels.ws.handlers.interrupt_handler import InterruptHandler
from agent.channels.ws.handlers.permission_handler import PermissionHandler
from agent.channels.ws.handlers.ping_handler import PingHandler
from agent.channels.ws.websocket_sender import WebSocketSender
from agent.infra.session.session_router import build_session_key
from agent.service.chat.chat_service import ChatService


class ChannelDispatcher:
    """负责将通道入站消息路由到对应处理器。"""

    def __init__(
        self,
        sender: WebSocketSender,
        chat_service: ChatService,
        interrupt_handler: InterruptHandler,
        permission_handler: PermissionHandler,
        ping_handler: PingHandler,
        error_handler: ErrorHandler,
        chat_tasks: Dict[str, Any],
    ) -> None:
        self._sender = sender
        self._chat_service = chat_service
        self._interrupt_handler = interrupt_handler
        self._permission_handler = permission_handler
        self._ping_handler = ping_handler
        self._error_handler = error_handler
        self._chat_tasks = chat_tasks

    async def dispatch(self, message: Dict[str, Any]) -> None:
        """根据消息类型路由到对应处理器。"""
        self._normalize_session_key(message)
        msg_type = message.get("type")

        if msg_type == "chat":
            await self._chat_service.handle_chat_message_with_task(message, self._chat_tasks)
            return

        if msg_type == "interrupt":
            await self._interrupt_handler.handle_interrupt(message, self._chat_tasks)
            return

        if msg_type == "permission_response":
            await self._permission_handler.handle_permission_response(message)
            return

        if msg_type == "subscribe_workspace":
            agent_id = message.get("agent_id", "")
            if agent_id:
                self._sender.subscribe_workspace(agent_id)
            return

        if msg_type == "unsubscribe_workspace":
            agent_id = message.get("agent_id", "")
            if agent_id:
                self._sender.unsubscribe_workspace(agent_id)
            return

        if msg_type == "ping":
            await self._ping_handler.handle_ping(message)
            return

        await self._error_handler.handle_unknown_message_type(message)

    @staticmethod
    def _normalize_session_key(message: Dict[str, Any]) -> None:
        """将前端 agent_id 标准化为内部 session_key。"""
        if "agent_id" not in message or "session_key" in message:
            return

        message["session_key"] = build_session_key(
            channel="ws",
            chat_type="dm",
            ref=message["agent_id"],
            agent_id=message["agent_id"],
        )
