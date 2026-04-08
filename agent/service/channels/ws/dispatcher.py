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

from agent.schema.model_message import EventMessage
from agent.service.channels.ws.handlers.error_handler import ErrorHandler
from agent.service.channels.ws.handlers.interrupt_handler import InterruptHandler
from agent.service.channels.ws.handlers.permission_handler import PermissionHandler
from agent.service.channels.ws.handlers.ping_handler import PingHandler
from agent.service.channels.ws.websocket_sender import WebSocketSender
from agent.service.channels.ws.ws_session_replay_registry import (
    ws_session_replay_registry,
)
from agent.service.channels.ws.ws_chat_task_registry import ws_chat_task_registry
from agent.service.channels.ws.ws_connection_registry import ws_connection_registry
from agent.service.chat.chat_service import ChatService
from agent.service.chat.room_chat_service import RoomChatService
from agent.service.permission.strategy.permission_interactive import (
    InteractivePermissionStrategy,
)
from agent.service.room.room_message_store import room_message_store
from agent.service.room.room_route_guard import room_route_guard
from agent.service.room.room_session_keys import build_room_shared_session_key
from agent.service.session.session_router import (
    get_session_key_validation_error,
    is_room_session_key,
)


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
        msg_type = message.get("type")
        if await self._reject_invalid_browser_session_key(message):
            return
        self._register_active_session(message)

        if msg_type == "chat":
            # Room 消息路由到 RoomChatService
            if self._is_room_message(message):
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

        if msg_type == "bind_session":
            # 中文注释：前端重连后需要知道后台是否仍在生成，以恢复 loading 态。
            session_key = message.get("session_key", "")
            if isinstance(session_key, str) and session_key:
                last_seen_session_seq = message.get("last_seen_session_seq")
                if isinstance(last_seen_session_seq, int) and last_seen_session_seq > 0:
                    await ws_session_replay_registry.replay_session_events(
                        sender=self._sender,
                        session_key=session_key,
                        last_seen_session_seq=last_seen_session_seq,
                    )
                await self._push_session_status(session_key)
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
                await self._restore_room_pending_slots(
                    room_id=room_id,
                    conversation_id=conversation_id if isinstance(conversation_id, str) else None,
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

    def _register_active_session(self, message: Dict[str, Any]) -> None:
        """把当前连接登记为该 session 的活跃 sender。"""
        session_key = message.get("session_key")
        if not isinstance(session_key, str) or not session_key:
            return
        InteractivePermissionStrategy.register_session_sender(
            session_key=session_key,
            sender=self._sender,
        )

    @staticmethod
    def _is_room_message(message: Dict[str, Any]) -> bool:
        """判断是否为 Room 消息。"""
        if message.get("chat_type") == "group":
            return True
        if message.get("room_id"):
            return True
        session_key = message.get("session_key", "")
        if isinstance(session_key, str) and is_room_session_key(session_key):
            return True
        return False

    async def _restore_room_pending_slots(
        self,
        room_id: str,
        conversation_id: str | None,
    ) -> None:
        """在 Room 重连后恢复仍在执行的 slot。"""
        if not conversation_id:
            return

        room_session_key = build_room_shared_session_key(conversation_id)
        running_round_id = ws_chat_task_registry.get_running_round_id(room_session_key)
        if not running_round_id:
            return

        active_slots = await room_message_store.get_active_slots(conversation_id)
        if not active_slots:
            return

        slots_by_round: dict[str, list[dict[str, Any]]] = {}
        for slot in active_slots:
            round_id = str(slot.get("round_id") or "")
            base_round_id = round_id.split(":", 1)[0] if round_id else ""
            # 中文注释：Room 重连恢复只允许补发“当前仍在运行的主 round”。
            # 这样可以把 SQL 中历史遗留的 inflight 记录挡在外面，避免旧消息重新被点亮成执行中。
            if not base_round_id or base_round_id != running_round_id:
                continue
            slots_by_round.setdefault(base_round_id, []).append(slot)

        for base_round_id, pending in slots_by_round.items():
            await self._sender.send_event_message(
                EventMessage(
                    event_type="chat_ack",
                    delivery_mode="ephemeral",
                    session_key=room_session_key,
                    room_id=room_id,
                    conversation_id=conversation_id,
                    caused_by=base_round_id,
                    data={
                        "req_id": base_round_id,
                        "round_id": base_round_id,
                        "pending": pending,
                    },
                )
            )

    async def _reject_invalid_browser_session_key(
        self,
        message: Dict[str, Any],
    ) -> bool:
        """拒绝浏览器直接发送的非法 session_key。"""
        msg_type = message.get("type")
        if msg_type not in {"chat", "interrupt", "permission_response", "bind_session"}:
            return False

        raw_session_key = message.get("session_key")
        if not isinstance(raw_session_key, str):
            raw_session_key = ""

        error_message = get_session_key_validation_error(raw_session_key)
        if error_message is None:
            message["session_key"] = raw_session_key.strip()
            return False

        await self._sender.send_event_message(
            self._error_handler.create_error_response(
                error_type=(
                    "validation_error"
                    if error_message == "session_key is required"
                    else "invalid_session_key"
                ),
                message=error_message,
                session_key=raw_session_key or None,
                details={"type": msg_type},
            )
        )
        return True

    async def _push_session_status(self, session_key: str) -> None:
        """向当前连接推送 session 运行状态，供前端恢复 loading 态。"""
        is_generating = ws_chat_task_registry.is_running(session_key)
        await self._sender.send_event_message(
            EventMessage(
                event_type="session_status",
                session_key=session_key,
                data={"is_generating": is_generating},
            )
        )
