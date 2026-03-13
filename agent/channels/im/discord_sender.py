# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：discord_sender.py
# @Date   ：2026/3/13 18:26
# @Author ：leemysw
# 2026/3/13 18:26   Create
# =====================================================

"""Discord 消息发送器。"""

from typing import List, Optional

import discord

from agent.channels.message_sender import MessageSender
from agent.schema.model_message import EventMessage, Message, StreamMessage
from agent.utils.logger import logger

DISCORD_MAX_LENGTH = 2000


class DiscordSender(MessageSender):
    """Discord 消息发送器。"""

    def __init__(self, discord_channel: discord.abc.Messageable):
        self._channel = discord_channel
        self._last_sent_text: Optional[str] = None

    async def send_message(self, message: Message) -> None:
        """发送 Agent 消息到 Discord。"""
        if message.role in ("system", "result"):
            return

        text = self._extract_text(message)
        if not text:
            return

        if text == self._last_sent_text:
            logger.debug(f"🔄 跳过重复消息: {text[:50]}...")
            return
        self._last_sent_text = text

        for chunk in self._split_message(text):
            await self._channel.send(chunk)

    async def send_stream_message(self, message: StreamMessage) -> None:
        """流式消息不推送到 Discord。"""
        del message

    async def send_event_message(self, event: EventMessage) -> None:
        """事件消息不推送到 Discord。"""
        del event

    @staticmethod
    def _extract_text(message: Message) -> Optional[str]:
        """从 Message 提取可读文本。"""
        if message.role == "assistant" and isinstance(message.content, list):
            texts = [block.text for block in message.content if getattr(block, "type", None) == "text"]
            return "\n".join(filter(None, texts)) or None

        if message.role == "user" and isinstance(message.content, str):
            return message.content

        return None

    @staticmethod
    def _split_message(text: str) -> List[str]:
        """将超长文本分片为 Discord 可发送的长度。"""
        if len(text) <= DISCORD_MAX_LENGTH:
            return [text]

        chunks = []
        while text:
            chunks.append(text[:DISCORD_MAX_LENGTH])
            text = text[DISCORD_MAX_LENGTH:]
        return chunks
