# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：telegram_sender.py
# @Date   ：2026/3/13 18:26
# @Author ：leemysw
# 2026/3/13 18:26   Create
# =====================================================

"""Telegram 消息发送器。"""

from typing import List, Optional

from telegram.ext import Application

from agent.channels.message_sender import MessageSender
from agent.schema.model_message import AError, AEvent, AMessage

TELEGRAM_MAX_LENGTH = 4096


class TelegramSender(MessageSender):
    """Telegram 消息发送器。"""

    def __init__(self, chat_id: int, application: Application):
        self._chat_id = chat_id
        self._app = application

    async def send_message(self, message: AMessage) -> None:
        """发送 Agent 消息到 Telegram。"""
        if message.message_type in ("stream", "system"):
            return

        text = self._extract_text(message)
        if not text or message.message_type == "result":
            return

        for chunk in self._split_message(text):
            await self._app.bot.send_message(
                chat_id=self._chat_id,
                text=chunk,
                parse_mode="Markdown",
            )

    async def send_event(self, event: AEvent) -> None:
        """事件消息不推送到 Telegram。"""
        del event

    async def send_error(self, error: AError) -> None:
        """发送错误到 Telegram。"""
        text = f"⚠️ Error: {error.message}"
        await self._app.bot.send_message(
            chat_id=self._chat_id,
            text=text[:TELEGRAM_MAX_LENGTH],
        )

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
        """将超长文本分片。"""
        if len(text) <= TELEGRAM_MAX_LENGTH:
            return [text]

        chunks = []
        while text:
            chunks.append(text[:TELEGRAM_MAX_LENGTH])
            text = text[TELEGRAM_MAX_LENGTH:]
        return chunks
