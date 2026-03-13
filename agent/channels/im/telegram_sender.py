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
from agent.schema.model_message import EventMessage, Message, StreamMessage

TELEGRAM_MAX_LENGTH = 4096


class TelegramSender(MessageSender):
    """Telegram 消息发送器。"""

    def __init__(self, chat_id: int, application: Application):
        self._chat_id = chat_id
        self._app = application

    async def send_message(self, message: Message) -> None:
        """发送 Agent 消息到 Telegram。"""
        if message.role in ("system", "result"):
            return

        text = self._extract_text(message)
        if not text:
            return

        for chunk in self._split_message(text):
            await self._app.bot.send_message(
                chat_id=self._chat_id,
                text=chunk,
                parse_mode="Markdown",
            )

    async def send_stream_message(self, message: StreamMessage) -> None:
        """流式消息不推送到 Telegram。"""
        del message

    async def send_event_message(self, event: EventMessage) -> None:
        """事件消息不推送到 Telegram。"""
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
        """将超长文本分片。"""
        if len(text) <= TELEGRAM_MAX_LENGTH:
            return [text]

        chunks = []
        while text:
            chunks.append(text[:TELEGRAM_MAX_LENGTH])
            text = text[TELEGRAM_MAX_LENGTH:]
        return chunks
