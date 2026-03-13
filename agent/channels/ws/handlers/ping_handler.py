#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：ping_handler.py
# @Date   ：2026/3/13 18:10
# @Author ：leemysw
# 2026/3/13 18:10   Create
# =====================================================

"""WebSocket 心跳处理器。"""

from typing import Any, Dict

from agent.channels.ws.handlers.base_handler import BaseHandler
from agent.infra.agent.session_manager import session_manager
from agent.schema.model_message import EventMessage
from agent.utils.logger import logger


class PingHandler(BaseHandler):
    """心跳检测处理器。"""

    async def handle_ping(self, message: Dict[str, Any]) -> None:
        """处理心跳检测消息。"""
        session_key = message.get("session_key") or message.get("agent_id")
        if not session_key:
            logger.warning("⚠️ ping消息缺少session_key")
            return
        logger.debug(f"💗收到心跳检测: session_key={session_key}")
        event = EventMessage(
            event_type="pong",
            session_key=session_key,
            session_id=session_manager.get_session_id(session_key),
            data={"status": "ok"},
        )
        await self.send(event)
