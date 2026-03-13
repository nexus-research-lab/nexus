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
from agent.schema.model_message import AEvent, AStatus
from agent.utils.logger import logger


class PingHandler(BaseHandler):
    """心跳检测处理器。"""

    async def handle_ping(self, message: Dict[str, Any]) -> None:
        """处理心跳检测消息。"""
        agent_id = message.get("agent_id")
        if not agent_id:
            logger.warning("⚠️ ping消息缺少agent_id")
            return
        logger.debug(f"💗收到心跳检测: agent_id={agent_id}")
        event = AEvent(
            event_type="pong",
            agent_id=agent_id,
            session_id=session_manager.get_session_id(agent_id),
            data=AStatus().model_dump(),
        )
        await self.send(event)
