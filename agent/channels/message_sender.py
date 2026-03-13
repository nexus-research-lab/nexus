# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：message_sender.py
# @Date   ：2026/3/13 18:24
# @Author ：leemysw
# 2026/3/13 18:24   Create
# =====================================================

"""消息发送协议。"""

from abc import ABC, abstractmethod
from typing import Union

from agent.schema.model_message import AError, AEvent, AMessage


class MessageSender(ABC):
    """消息发送协议。"""

    async def send(self, message: Union[AMessage, AEvent, AError]) -> None:
        """统一发送入口，自动分发到具体方法。"""
        if isinstance(message, AMessage):
            await self.send_message(message)
        elif isinstance(message, AEvent):
            await self.send_event(message)
        elif isinstance(message, AError):
            await self.send_error(message)

    @abstractmethod
    async def send_message(self, message: AMessage) -> None:
        """发送 Agent 消息。"""
        ...

    @abstractmethod
    async def send_event(self, event: AEvent) -> None:
        """发送事件消息。"""
        ...

    @abstractmethod
    async def send_error(self, error: AError) -> None:
        """发送错误消息。"""
        ...
