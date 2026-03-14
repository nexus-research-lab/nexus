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

from agent.service.channels.message_sender import MessageSender
from agent.schema.model_message import EventMessage, Message, StreamMessage, build_error_event


class BaseHandler(ABC):
    """WebSocket 处理器基类。"""

    def __init__(self, sender: MessageSender):
        self.sender = sender

    async def send(self, message: Union[Message, StreamMessage, EventMessage]) -> None:
        """通过 MessageSender 协议发送消息，与传输层解耦。"""
        await self.sender.send(message)

    def create_error_response(
        self,
        error_type: str,
        message: str,
        agent_id: Optional[str] = None,
        session_id: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        session_key: Optional[str] = None,
    ) -> EventMessage:
        """创建错误响应模型。"""
        return build_error_event(
            error_type=error_type,
            message=message,
            session_key=session_key,
            session_id=session_id,
            agent_id=agent_id,
            details=details,
        )
