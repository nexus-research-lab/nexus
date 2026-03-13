# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_message_processor.py
# @Date   ：2026/3/14 12:00
# @Author ：leemysw
# 2026/3/14 12:00   Create
# =====================================================

"""单轮聊天消息处理器。"""

from __future__ import annotations

import uuid
from typing import Optional

from claude_agent_sdk import Message as SDKMessage
from claude_agent_sdk import ResultMessage, SystemMessage

from agent.infra.agent.sdk_message_processor import sdk_message_processor
from agent.infra.agent.session_manager import session_manager
from agent.infra.agent.stream_message_state import StreamMessageState
from agent.schema.model_message import Message, StreamMessage
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class ChatMessageProcessor:
    """负责单轮消息转换、落盘与流式状态维护。"""

    def __init__(self, session_key: str, query: str, round_id: Optional[str] = None, agent_id: str = "main"):
        self.query = query
        self.session_key = session_key
        self.agent_id = agent_id or "main"
        self.subtype: Optional[str] = None
        self.round_id: Optional[str] = round_id
        self.parent_id: Optional[str] = None
        self.session_id: Optional[str] = None
        self.message_count: int = 0
        self.is_save_user_message: bool = False
        self.stream_state = StreamMessageState()

    async def process_messages(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """处理响应消息并管理消息状态。"""
        sdk_message_processor.print_message(response_msg, self.session_key)
        self.set_subtype(response_msg)
        await self.set_session_id(response_msg)
        await self.save_user_message(self.query)

        messages = sdk_message_processor.process_message(
            message=response_msg,
            session_key=self.session_key,
            agent_id=self.agent_id,
            session_id=self.session_id or "",
            round_id=self.round_id or "",
            parent_id=self.parent_id,
        )

        processed_messages: list[Message | StreamMessage] = []
        for message in messages:
            self.stream_state.apply(message)

            if isinstance(message, StreamMessage) and self.stream_state.is_streaming_tool:
                continue

            if isinstance(message, Message):
                self.parent_id = message.message_id
                await session_store.save_message(message)

            processed_messages.append(message)
            self.message_count += 1

        return processed_messages

    async def set_session_id(self, response_msg: SDKMessage) -> Optional[str]:
        """处理 session 映射关系。"""
        if self.session_id is not None:
            return self.session_id

        if not isinstance(response_msg, SystemMessage):
            raise ValueError("When session_id is None, response_msg must be a SystemMessage")

        raw = sdk_message_processor.to_plain_dict(response_msg)
        data = raw.get("data") or {}
        self.session_id = data.get("session_id")
        await session_manager.register_sdk_session(session_key=self.session_key, session_id=self.session_id)
        logger.debug(f"🔗建立映射: key={self.session_key} ↔ sdk_session={self.session_id}")
        return self.session_id

    def set_subtype(self, response_msg: SDKMessage) -> None:
        """设置消息子类型。"""
        raw = sdk_message_processor.to_plain_dict(response_msg)
        subtype = raw.get("subtype")
        if subtype:
            self.subtype = str(subtype)
        if isinstance(response_msg, ResultMessage):
            self.subtype = "success" if self.subtype == "success" else "error"

    async def save_user_message(self, content: str) -> None:
        """保存用户消息。"""
        if self.is_save_user_message:
            return
        if not self.round_id:
            self.round_id = str(uuid.uuid4())

        user_message = Message(
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            message_id=self.round_id,
            session_id=self.session_id,
            role="user",
            content=content,
        )
        await session_store.save_message(user_message)
        self.is_save_user_message = True
