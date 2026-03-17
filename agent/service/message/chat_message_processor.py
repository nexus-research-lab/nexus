# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_message_processor.py
# @Date   ：2026/03/17 20:08
# @Author ：leemysw
# 2026/03/17 20:08   Create
# =====================================================

"""单轮聊天消息处理器。"""

from __future__ import annotations

import uuid
from typing import Optional

from claude_agent_sdk import Message as SDKMessage
from claude_agent_sdk import ResultMessage, SystemMessage
from claude_agent_sdk.types import AssistantMessage, StreamEvent, UserMessage

from agent.schema.model_message import Message, StreamMessage
from agent.service.message.assistant_segment import AssistantSegment
from agent.service.message.sdk_message_mapper import SdkMessageMapper
from agent.service.session.session_manager import session_manager
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class ChatMessageProcessor:
    """负责将 SDK 消息转换为前后端统一消息协议。"""

    def __init__(
        self,
        session_key: str,
        query: str,
        round_id: Optional[str] = None,
        agent_id: str = "main",
        session_id: Optional[str] = None,
    ) -> None:
        self.query = query
        self.session_key = session_key
        self.agent_id = agent_id or "main"
        self.round_id = round_id or str(uuid.uuid4())
        self.session_id = session_id
        self.subtype: Optional[str] = None
        self.message_count = 0
        self._is_user_message_saved = False
        self._last_assistant_message_id: Optional[str] = None
        self._segment = AssistantSegment()

    async def process_messages(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """处理单条 SDK 消息。"""
        if not isinstance(response_msg, StreamEvent):
            logger.debug(
                "📨 SDK message: session=%s type=%s payload=%s",
                self.session_key,
                type(response_msg).__name__,
                SdkMessageMapper.to_plain_dict(response_msg),
            )
        await self._register_session_id(response_msg)
        if not isinstance(response_msg, SystemMessage):
            await self._save_user_message()

        messages = self._dispatch(response_msg)
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        subtype = payload.get("subtype")
        if subtype:
            self.subtype = str(subtype)
        if isinstance(response_msg, ResultMessage):
            self.subtype = "success" if self.subtype == "success" else "error"

        for message in messages:
            if isinstance(message, Message):
                await session_store.save_message(message)
            self.message_count += 1
        return messages

    async def _register_session_id(self, response_msg: SDKMessage) -> None:
        """在 system message 中建立 session 映射。"""
        if self.session_id or not isinstance(response_msg, SystemMessage):
            return
        session_id = (SdkMessageMapper.to_plain_dict(response_msg).get("data") or {}).get("session_id")
        if not session_id:
            return
        self.session_id = session_id
        await session_manager.register_sdk_session(session_key=self.session_key, session_id=session_id)
        logger.debug("🔗建立映射: key=%s ↔ sdk_session=%s", self.session_key, session_id)

    async def _save_user_message(self) -> None:
        """只在当前轮次首次收到非 system 消息时写入用户消息。"""
        if self._is_user_message_saved:
            return
        user_message = Message(
            message_id=self.round_id,
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            role="user",
            content=self.query,
        )
        await session_store.save_message(user_message)
        self._is_user_message_saved = True

    def _dispatch(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """按 SDK 消息类型分发。"""
        if isinstance(response_msg, SystemMessage):
            return []
        if isinstance(response_msg, StreamEvent):
            return self._handle_stream_event(response_msg)
        if isinstance(response_msg, AssistantMessage):
            return self._handle_assistant_message(response_msg)
        if isinstance(response_msg, UserMessage):
            return self._handle_tool_result_message(response_msg)
        if isinstance(response_msg, ResultMessage):
            return [self._build_result_message(response_msg)]
        raise ValueError(f"Unsupported SDK message type: {type(response_msg)}")

    def _handle_stream_event(self, response_msg: StreamEvent) -> list[Message | StreamMessage]:
        """处理流式事件，仅负责实时 patch。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        event = payload.get("event") if isinstance(payload.get("event"), dict) else payload
        event_type = str(event.get("type") or "")
        if not event_type:
            return []

        if event_type == "message_start":
            message = event.get("message") or {}
            self._segment.start(
                message_id=message.get("id"),
                model=message.get("model"),
                usage=message.get("usage"),
            )
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="message_start",
                    message={"model": self._segment.model},
                    usage=self._segment.usage,
                )
            ]

        if event_type == "content_block_start":
            index = event.get("index")
            if not isinstance(index, int):
                return []
            block = SdkMessageMapper.to_plain_block(event.get("content_block"))
            self._segment.apply_block(index, block)
            if block.get("type") == "tool_use":
                return []
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="content_block_start",
                    index=index,
                    content_block=block,
                )
            ]

        if event_type == "content_block_delta":
            index = event.get("index")
            if not isinstance(index, int) or not self._segment.apply_delta(index, event.get("delta") or {}):
                return []
            current_block = self._segment.content[index]
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="content_block_delta",
                    index=index,
                    content_block=current_block,
                )
            ]

        if event_type == "message_delta":
            delta = event.get("delta") or {}
            self._segment.update_message_meta(
                stop_reason=delta.get("stop_reason"),
                usage=event.get("usage"),
            )
            messages: list[Message | StreamMessage] = [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="message_delta",
                    message={"stop_reason": self._segment.stop_reason},
                    usage=self._segment.usage,
                )
            ]
            if self._segment.has_content() and self._segment.stop_reason:
                assistant_message = self._segment.build_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    parent_id=self.round_id,
                    is_complete=True,
                )
                self._remember_assistant_message(assistant_message)
                messages.append(assistant_message)
            return messages

        if event_type == "message_stop":
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="message_stop",
                )
            ]

        return []

    def _handle_assistant_message(self, response_msg: AssistantMessage) -> list[Message]:
        """使用 SDK assistant 快照直接覆盖当前段内容。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        content = SdkMessageMapper.normalize_content_blocks(payload.get("content"))
        if not content:
            return []
        self._segment.replace_from_snapshot(
            content=content,
            model=payload.get("model"),
            usage=payload.get("usage"),
            stop_reason=payload.get("stop_reason"),
        )
        assistant_message = self._segment.build_message(
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self.round_id,
            is_complete=bool(self._segment.stop_reason),
        )
        self._remember_assistant_message(assistant_message)
        return [assistant_message]

    def _handle_tool_result_message(self, response_msg: UserMessage) -> list[Message]:
        """将工具结果回灌转换为 assistant 消息。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        content = SdkMessageMapper.normalize_content_blocks(payload.get("content"))
        if not content or not all(block.get("type") == "tool_result" for block in content):
            return []
        message = Message(
            message_id=str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self._current_parent_id(),
            role="assistant",
            content=content,
            is_complete=True,
        )
        self._remember_assistant_message(message)
        return [message]

    def _build_result_message(self, response_msg: ResultMessage) -> Message:
        """构建结果消息。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        subtype = str(payload.get("subtype") or "success")
        normalized_subtype = subtype if subtype in ("success", "error", "interrupted") else "error"
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self._current_parent_id(),
            role="result",
            subtype=normalized_subtype,
            duration_ms=int(payload.get("duration_ms") or 0),
            duration_api_ms=int(payload.get("duration_api_ms") or 0),
            num_turns=int(payload.get("num_turns") or 0),
            total_cost_usd=payload.get("total_cost_usd"),
            usage=payload.get("usage"),
            result=payload.get("result"),
            is_error=bool(payload.get("is_error", normalized_subtype != "success")),
        )

    def _current_parent_id(self) -> str:
        """返回当前消息应该关联的父节点。"""
        return self._segment.message_id or self._last_assistant_message_id or self.round_id
    def _remember_assistant_message(self, message: Message) -> None:
        """记录最近一条 assistant 消息 ID。"""
        self._last_assistant_message_id = message.message_id
