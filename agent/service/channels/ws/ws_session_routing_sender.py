# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：ws_session_routing_sender.py
# @Date   ：2026/04/07 13:02
# @Author ：leemysw
# 2026/04/07 13:02   Create
# =====================================================

"""按 session_key 路由到当前活跃连接的发送器。"""

import uuid

from agent.schema.model_message import (
    EventMessage,
    Message,
    StreamMessage,
    TextContent,
    build_transport_event,
)
from agent.service.channels.message_sender import MessageSender
from agent.service.channels.ws.websocket_sender import WebSocketSender
from agent.service.channels.ws.ws_session_replay_registry import (
    ws_session_replay_registry,
)
from agent.service.permission.permission_runtime_context import (
    permission_runtime_context,
)
from agent.utils.logger import logger


class WsSessionRoutingSender(MessageSender):
    """将消息转发到某个 session 当前活跃的 WebSocket 连接。"""

    def __init__(self, fallback_sender: WebSocketSender) -> None:
        self._fallback_sender = fallback_sender

    async def send_message(self, message: Message) -> None:
        """发送完整消息。"""
        await self._forward_event(build_transport_event(message))

    async def send_stream_message(self, message: StreamMessage) -> None:
        """发送流式消息。"""
        await self._forward_event(build_transport_event(message))

    async def send_event_message(self, event: EventMessage) -> None:
        """发送事件消息。"""
        await self._forward_event(event)

    async def send_text(self, session_key: str, text: str) -> None:
        """向指定 session 推送自动化文本。"""
        from agent.service.session.session_store import session_store
        from agent.service.session.session_repository import session_repository

        session_info = await session_store.get_session_info(session_key)
        if session_info is None:
            raise LookupError(f"websocket delivery target session is not available: {session_key}")

        round_id = str(uuid.uuid4())
        assistant_message = Message(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=session_info.agent_id,
            round_id=round_id,
            session_id=session_info.session_id,
            role="assistant",
            content=[TextContent(text=text)],
        )
        result_message = Message(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=session_info.agent_id,
            round_id=round_id,
            session_id=session_info.session_id,
            parent_id=assistant_message.message_id,
            role="result",
            subtype="success",
            duration_ms=0,
            duration_api_ms=0,
            num_turns=0,
            total_cost_usd=0,
            usage={"input_tokens": 0, "output_tokens": 0},
            result=text,
            is_error=False,
        )

        persisted_assistant = await session_repository.create_message(assistant_message)
        persisted_result = await session_repository.create_message(result_message)
        if not persisted_assistant or not persisted_result:
            raise LookupError(f"websocket delivery target session is not available: {session_key}")

        for message in (assistant_message, result_message):
            try:
                await self.send_message(message)
            except Exception as exc:
                # 中文注释：消息已先落库，某条实时推送失败不应阻断后续消息进入 replay 管线。
                logger.warning("⚠️ 自动化实时推送失败，消息已落库: session=%s error=%s", session_key, exc)

    async def _forward_event(self, event: EventMessage) -> None:
        """把消息发给当前活跃连接。"""
        session_key = event.session_key
        prepared_event = ws_session_replay_registry.prepare_session_event(event)

        if not session_key:
            await self._fallback_sender.send_event_message(prepared_event)
            return

        active_sender = permission_runtime_context.resolve_session_sender(session_key)
        if active_sender is None:
            # 中文注释：断线期间不应把后台运行链打断。
            # 当前没有活跃连接时直接跳过实时推送，等待前端重连后继续接收增量，
            # 并依靠前端重拉补齐断线期间已落库的完整消息。
            logger.debug("📭 当前无活跃连接，跳过实时推送: session=%s", session_key)
            return

        await active_sender.send(prepared_event)
