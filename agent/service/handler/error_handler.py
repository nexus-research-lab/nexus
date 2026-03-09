#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：error_handler.py
# @Date   ：2025/12/06
# @Author ：leemysw
#
# 2025/12/06   Create
# =====================================================

import traceback
from typing import Any, Dict

from agent.service.handler.base_handler import BaseHandler
from agent.utils.logger import logger


class ErrorHandler(BaseHandler):
    """错误处理器"""

    async def handle_unknown_message_type(self, message: Dict[str, Any]) -> None:
        """
        处理未知消息类型

        Args:
            message: 原始消息
        """

        agent_id = message.get("agent_id")
        msg_type = message.get("type")
        logger.warning(f"❓未知消息类型: {msg_type}")
        error_response = self.create_error_response(
            error_type="unknown_message_type",
            message=f"Unknown message type: {msg_type}",
            session_id=agent_id,
            details={"original_message": message}
        )
        await self.send(error_response)

    async def handle_websocket_error(self, error: Exception) -> None:
        """处理WebSocket错误"""
        logger.error(f"❌WebSocket错误: {error}")
        traceback.print_exc()

        # 发送错误响应
        error_response = self.create_error_response(
            error_type="websocket_error",
            message=str(error),
            session_id=None
        )
        await self.send(error_response)
