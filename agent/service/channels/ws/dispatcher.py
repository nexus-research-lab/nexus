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

from agent.service.channels.ws.handlers.error_handler import ErrorHandler
from agent.service.channels.ws.handlers.interrupt_handler import InterruptHandler
from agent.service.channels.ws.handlers.permission_handler import PermissionHandler
from agent.service.channels.ws.handlers.ping_handler import PingHandler
from agent.service.channels.ws.websocket_sender import WebSocketSender
from agent.service.channels.ws.ws_connection_registry import ws_connection_registry
from agent.service.chat.chat_service import ChatService
from agent.service.chat.room_chat_service import RoomChatService, build_room_session_key
from agent.service.room.room_route_guard import room_route_guard
from agent.service.session.session_router import build_session_key


class ChannelDispatcher:
    """负责将通道入站消息路由到对应处理器。"""

    def __init__(
        self,
        sender: WebSocketSender,
        chat_service: ChatService,
        room_chat_service: RoomChatService,
        interrupt_handler: InterruptHandler,
        permission_handler: PermissionHandler,
        ping_handler: PingHandler,
        error_handler: ErrorHandler,
        chat_tasks: Dict[str, Any],
    ) -> None:
        self._sender = sender
        self._chat_service = chat_service
        self._room_chat_service = room_chat_service
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
            # Room 消息路由到 RoomChatService
            if self._is_room_message(message):
                self._normalize_room_session_key(message)
                await self._room_chat_service.handle_room_message_with_task(
                    message, self._chat_tasks,
                )
            else:
                await self._chat_service.handle_chat_message_with_task(
                    message, self._chat_tasks,
                )
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

        if msg_type == "subscribe_room":
            room_id = message.get("room_id", "")
            conversation_id = message.get("conversation_id")
            last_seen_room_seq = message.get("last_seen_room_seq")
            if room_id:
                try:
                    await room_route_guard.validate_subscription(
                        room_id=room_id,
                        conversation_id=conversation_id if isinstance(conversation_id, str) else None,
                    )
                except ValueError as exc:
                    await self._sender.send_event_message(
                        self._error_handler.create_error_response(
                            error_type="invalid_room_subscription",
                            message=str(exc),
                            details={
                                "room_id": room_id,
                                "conversation_id": conversation_id,
                            },
                        )
                    )
                    return
                await ws_connection_registry.subscribe_room(
                    self._sender,
                    room_id,
                    conversation_id=conversation_id if isinstance(conversation_id, str) else None,
                    last_seen_room_seq=(
                        last_seen_room_seq if isinstance(last_seen_room_seq, int) else None
                    ),
                )
            return

        if msg_type == "unsubscribe_room":
            room_id = message.get("room_id", "")
            if room_id:
                ws_connection_registry.unsubscribe_room(self._sender, room_id)
            return

        if msg_type == "ping":
            await self._ping_handler.handle_ping(message)
            return

        await self._error_handler.handle_unknown_message_type(message)

    @staticmethod
    def _is_room_message(message: Dict[str, Any]) -> bool:
        """判断是否为 Room 消息。"""
        if message.get("chat_type") == "group":
            return True
        if message.get("room_id"):
            return True
        session_key = message.get("session_key", "")
        if isinstance(session_key, str) and session_key.startswith("room:"):
            return True
        return False

    @staticmethod
    def _normalize_room_session_key(message: Dict[str, Any]) -> None:
        """为 Room 消息构建共享 session_key。"""
        conversation_id = message.get("conversation_id", "")
        if not message.get("session_key") and conversation_id:
            message["session_key"] = build_room_session_key(conversation_id)

    @staticmethod
    def _normalize_session_key(message: Dict[str, Any]) -> None:
        """将前端 agent_id 标准化为内部 session_key（仅 DM）。"""
        if "agent_id" not in message or "session_key" in message:
            return

        message["session_key"] = build_session_key(
            channel="ws",
            chat_type="dm",
            ref=message["agent_id"],
            agent_id=message["agent_id"],
        )
