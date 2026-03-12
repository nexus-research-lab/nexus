#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：ping_handler.py
# @Date   ：2025/12/06
# @Author ：leemysw
#
# 2025/12/06   Create
# =====================================================

from typing import Any, Dict

from agent.service.handler.base_handler import BaseHandler
from agent.service.schema.model_message import AEvent, AStatus
from agent.infra.runtime.session_manager import session_manager
from agent.utils.logger import logger


class PingHandler(BaseHandler):
    """心跳检测处理器"""

    async def handle_ping(self, message: Dict[str, Any]) -> None:
        """
        处理心跳检测消息
        """
        agent_id = message.get("agent_id")
        if not agent_id:
            logger.warning("⚠️ ping消息缺少agent_id")
            return
        logger.debug(f"💗收到心跳检测: agent_id={agent_id}")
        event = AEvent(
            event_type="pong",
            agent_id=agent_id,
            session_id=session_manager.get_session_id(agent_id),
            data=AStatus().model_dump()
        )
        await self.send(event)
