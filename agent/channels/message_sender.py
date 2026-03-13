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

from agent.schema.model_message import EventMessage, Message, StreamMessage


class MessageSender(ABC):
    """消息发送协议。"""

    async def send(self, message: Union[Message, StreamMessage, EventMessage]) -> None:
        """统一发送入口，自动分发到具体方法。"""
        if isinstance(message, Message):
            await self.send_message(message)
            return
        if isinstance(message, StreamMessage):
            await self.send_stream_message(message)
            return
        if isinstance(message, EventMessage):
            await self.send_event_message(message)

    @abstractmethod
    async def send_message(self, message: Message) -> None:
        """发送 Agent 消息。"""
        ...

    @abstractmethod
    async def send_stream_message(self, message: StreamMessage) -> None:
        """发送流式消息。"""
        ...

    @abstractmethod
    async def send_event_message(self, event: EventMessage) -> None:
        """发送事件消息。"""
        ...
