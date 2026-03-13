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
from agent.schema.model_message import AError, AEvent, AMessage
from agent.utils.logger import logger

DISCORD_MAX_LENGTH = 2000


class DiscordSender(MessageSender):
    """Discord 消息发送器。"""

    def __init__(self, discord_channel: discord.abc.Messageable):
        self._channel = discord_channel
        self._last_sent_text: Optional[str] = None

    async def send_message(self, message: AMessage) -> None:
        """发送 Agent 消息到 Discord。"""
        if message.message_type == "stream":
            return

        if message.message_type == "system":
            return

        text = self._extract_text(message)
        if not text or message.message_type == "result":
            return

        if text == self._last_sent_text:
            logger.debug(f"🔄 跳过重复消息: {text[:50]}...")
            return
        self._last_sent_text = text

        for chunk in self._split_message(text):
            await self._channel.send(chunk)

    async def send_event(self, event: AEvent) -> None:
        """事件消息不推送到 Discord。"""
        del event

    async def send_error(self, error: AError) -> None:
        """发送错误到 Discord。"""
        text = f"⚠️ Error: {error.message}"
        await self._channel.send(text[:DISCORD_MAX_LENGTH])

    @staticmethod
    def _extract_text(message: AMessage) -> Optional[str]:
        """从 AMessage 提取可读文本。"""
        msg = message.message
        if message.message_type == "assistant" and message.block_type == "text":
            if hasattr(msg, "content") and msg.content:
                block = msg.content[0]
                if hasattr(block, "text"):
                    return block.text
            return None

        if message.message_type == "result":
            if hasattr(msg, "result") and msg.result:
                return f"✅ {msg.result}"
            return None

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
