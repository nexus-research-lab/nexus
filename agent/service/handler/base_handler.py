#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：error_handler.py
# @Date   ：2025/12/06
# @Author ：leemysw
#
# 2025/12/06   Create
# =====================================================

from abc import ABC
from typing import Any, Dict, Optional, Union

from agent.service.channel.channel import MessageSender
from agent.service.schema.model_message import AError, AEvent, AMessage


class BaseHandler(ABC):

    def __init__(self, sender: MessageSender):
        self.sender = sender

    async def send(self, message: Union[AEvent, AError, AMessage]) -> None:
        """通过 MessageSender 协议发送消息，与传输层解耦"""
        await self.sender.send(message)

    def create_error_response(
            self, error_type: str, message: str,
            agent_id: Optional[str] = None,
            session_id: Optional[str] = None,
            details: Optional[Dict[str, Any]] = None
    ) -> AError:
        """创建错误响应模型"""
        return AError(
            error_type=error_type,
            message=message,
            session_id=session_id,
            agent_id=agent_id,
            details=details
        )
