#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_handler.py
# @Date   ：2026/3/13 18:10
# @Author ：leemysw
# 2026/3/13 18:10   Create
# =====================================================

"""WebSocket 入站处理器基类。"""

from abc import ABC
from typing import Any, Dict, Optional, Union

from agent.channels.message_sender import MessageSender
from agent.schema.model_message import AError, AEvent, AMessage


class BaseHandler(ABC):
    """WebSocket 处理器基类。"""

    def __init__(self, sender: MessageSender):
        self.sender = sender

    async def send(self, message: Union[AEvent, AError, AMessage]) -> None:
        """通过 MessageSender 协议发送消息，与传输层解耦。"""
        await self.sender.send(message)

    def create_error_response(
        self,
        error_type: str,
        message: str,
        agent_id: Optional[str] = None,
        session_id: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
    ) -> AError:
        """创建错误响应模型。"""
        return AError(
            error_type=error_type,
            message=message,
            session_id=session_id,
            agent_id=agent_id,
            details=details,
        )
