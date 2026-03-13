# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：message_formatter.py
# @Date   ：2026/3/14 11:42
# @Author ：leemysw
# 2026/3/14 11:42   Create
# =====================================================

"""Claude 消息格式转换与会话消息处理。"""

from __future__ import annotations

import json
import uuid
from datetime import datetime
from typing import Any, Dict, List, Optional

from claude_agent_sdk import Message as SDKMessage
from claude_agent_sdk import ResultMessage, SystemMessage
from claude_agent_sdk.types import AssistantMessage, StreamEvent, TextBlock, ThinkingBlock, ToolResultBlock, ToolUseBlock, UserMessage

from agent.infra.agent.session_manager import session_manager
from agent.schema.model_message import Message, StreamMessage
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class SDKMessageProcessor:
    """Claude Agent SDK 消息处理器。"""

    def process_message(
        self,
        message: SDKMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str] = None,
    ) -> List[Message | StreamMessage]:
        """将 Claude SDK 消息转换为统一协议。"""
        if isinstance(message, SystemMessage):
            return []

        if isinstance(message, StreamEvent):
            return [self._build_stream_message(message, session_key, agent_id, session_id, round_id)]

        if isinstance(message, AssistantMessage):
            return [self._build_assistant_message(message, session_key, agent_id, session_id, round_id, parent_id)]

        if isinstance(message, UserMessage):
            return [self._build_user_or_tool_result_message(message, session_key, agent_id, session_id, round_id, parent_id)]

        if isinstance(message, ResultMessage):
            return [self._build_result_message(message, session_key, agent_id, session_id, round_id, parent_id)]

        raise ValueError(f"Unsupported SDK message type: {type(message)}")

    @staticmethod
    def _to_plain_dict(payload: Any) -> Dict[str, Any]:
        """将任意对象转换为字典。"""
        if payload is None:
            return {}
        if isinstance(payload, dict):
            return dict(payload)
        if hasattr(payload, "model_dump"):
            return payload.model_dump(mode="json", exclude_none=True)
        if hasattr(payload, "__dict__"):
            return dict(payload.__dict__)
        return {}

    @staticmethod
    def _to_plain_block(block: Any) -> Dict[str, Any]:
        """将 SDK 内容块统一转换为字典。"""
        if isinstance(block, dict):
            return dict(block)
        if hasattr(block, "model_dump"):
            return block.model_dump(mode="json", exclude_none=True)
        if isinstance(block, TextBlock):
            return {"type": "text", "text": block.text}
        if isinstance(block, ThinkingBlock):
            return {"type": "thinking", "thinking": block.thinking, "signature": getattr(block, "signature", None)}
        if isinstance(block, ToolUseBlock):
            return {"type": "tool_use", "id": block.id, "name": block.name, "input": block.input or {}}
        if isinstance(block, ToolResultBlock):
            return {
                "type": "tool_result",
                "tool_use_id": block.tool_use_id,
                "content": block.content,
                "is_error": bool(getattr(block, "is_error", False)),
            }
        return {"type": "text", "text": str(block)}

    def _normalize_content_blocks(self, content: Any) -> List[Dict[str, Any]]:
        """统一规范化内容块列表。"""
        if isinstance(content, str):
            return [{"type": "text", "text": content}]
        if isinstance(content, list):
            return [self._to_plain_block(block) for block in content]
        if content is None:
            return []
        return [{"type": "text", "text": str(content)}]

    def _build_assistant_message(
        self,
        message: AssistantMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str],
    ) -> Message:
        """构建助手消息。"""
        raw = self._to_plain_dict(message)
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            parent_id=parent_id,
            role="assistant",
            content=self._normalize_content_blocks(raw.get("content")),
            model=raw.get("model"),
            stop_reason=raw.get("stop_reason"),
            usage=raw.get("usage"),
            parent_tool_use_id=raw.get("parent_tool_use_id"),
        )

    def _build_user_or_tool_result_message(
        self,
        message: UserMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str],
    ) -> Message:
        """构建用户消息或 tool_result 消息。"""
        raw = self._to_plain_dict(message)
        blocks = self._normalize_content_blocks(raw.get("content"))
        if blocks and all(block.get("type") == "tool_result" for block in blocks):
            return Message(
                message_id=str(uuid.uuid4()),
                session_key=session_key,
                agent_id=agent_id,
                round_id=round_id,
                session_id=session_id,
                parent_id=parent_id,
                role="assistant",
                content=blocks,
                parent_tool_use_id=raw.get("parent_tool_use_id"),
                is_tool_result=True,
            )

        content = raw.get("content")
        if not isinstance(content, str):
            content = next(
                (block.get("text", "") for block in blocks if block.get("type") == "text"),
                "",
            )
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            parent_id=parent_id,
            role="user",
            content=content,
            parent_tool_use_id=raw.get("parent_tool_use_id"),
        )

    def _build_result_message(
        self,
        message: ResultMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str],
    ) -> Message:
        """构建结果消息。"""
        raw = self._to_plain_dict(message)
        subtype = str(raw.get("subtype") or "success")
        normalized_subtype = subtype if subtype in ("success", "error", "interrupted") else "error"
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            parent_id=parent_id,
            role="result",
            subtype=normalized_subtype,
            duration_ms=int(raw.get("duration_ms") or 0),
            duration_api_ms=int(raw.get("duration_api_ms") or 0),
            num_turns=int(raw.get("num_turns") or 0),
            total_cost_usd=raw.get("total_cost_usd"),
            usage=raw.get("usage"),
            result=raw.get("result"),
            is_error=bool(raw.get("is_error", normalized_subtype != "success")),
        )

    def _build_stream_message(
        self,
        message: StreamEvent,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
    ) -> StreamMessage:
        """构建流式消息。"""
        raw = self._to_plain_dict(message)
        event = raw.get("event") if isinstance(raw.get("event"), dict) else raw
        return StreamMessage(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            type=str(event.get("type") or ""),
            index=event.get("index"),
            delta=event.get("delta"),
            message=event.get("message"),
            usage=event.get("usage"),
            content_block=self._to_plain_block(event["content_block"]) if event.get("content_block") else None,
        )

    def print_message(self, message: SDKMessage, session_id: Optional[str] = None) -> None:
        """打印 SDK 消息，便于跟踪执行过程。"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        prefix = f"🕐 [{timestamp}] "
        if session_id:
            prefix += f"📋 Session: {session_id} - "
        print(prefix, end="")

        if isinstance(message, AssistantMessage):
            print("AssistantMessage")
        elif isinstance(message, UserMessage):
            print("UserMessage")
        elif isinstance(message, SystemMessage):
            print("SystemMessage")
        elif isinstance(message, ResultMessage):
            print("ResultMessage")
        elif isinstance(message, StreamEvent):
            print("StreamEvent")
        else:
            print(type(message))

        raw = self._to_plain_dict(message)
        print(json.dumps(raw, ensure_ascii=False, indent=2))
        print("=" * 80)


sdk_message_processor = SDKMessageProcessor()


class ChatMessageProcessor:
    """单轮聊天消息处理器。"""

    def __init__(self, session_key: str, query: str, round_id: Optional[str] = None, agent_id: str = "main"):
        self.query = query
        self.session_key = session_key
        self.agent_id = agent_id or "main"
        self.subtype: Optional[str] = None
        self.round_id: Optional[str] = round_id
        self.parent_id: Optional[str] = None
        self.session_id: Optional[str] = None
        self.message_count: int = 0
        self.is_streaming: bool = False
        self.is_streaming_tool: bool = False
        self.is_save_user_message: bool = False
        self.stream_message_id: Optional[str] = None
        self.accumulated_thinking: str = ""
        self.accumulated_signature: str = ""
        self.accumulated_content_blocks: List[Dict[str, Any]] = []

    async def process_messages(self, response_msg: SDKMessage) -> List[Message | StreamMessage]:
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

        processed_messages: List[Message | StreamMessage] = []
        for message in messages:
            self.update_stream_state(message)

            if isinstance(message, StreamMessage) and self.is_streaming_tool:
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

        raw = sdk_message_processor._to_plain_dict(response_msg)
        data = raw.get("data") or {}
        self.session_id = data.get("session_id")
        await session_manager.register_sdk_session(session_key=self.session_key, session_id=self.session_id)
        logger.debug(f"🔗建立映射: key={self.session_key} ↔ sdk_session={self.session_id}")
        return self.session_id

    def set_subtype(self, response_msg: SDKMessage) -> None:
        """设置消息子类型。"""
        raw = sdk_message_processor._to_plain_dict(response_msg)
        subtype = raw.get("subtype")
        if subtype:
            self.subtype = str(subtype)
        if isinstance(response_msg, ResultMessage):
            self.subtype = "success" if self.subtype == "success" else "error"

    def update_stream_state(self, message: Message | StreamMessage) -> None:
        """更新流式处理状态。"""
        if isinstance(message, StreamMessage) and message.type == "message_start":
            self.is_streaming = True
            self.stream_message_id = message.message_id
            self.accumulated_thinking = ""
            self.accumulated_signature = ""
            self.accumulated_content_blocks = []

        if self.is_streaming:
            if isinstance(message, StreamMessage):
                if self.stream_message_id:
                    message.message_id = self.stream_message_id

                if message.type == "content_block_start":
                    block = sdk_message_processor._to_plain_block(message.content_block) if message.content_block else {}
                    if block.get("type") == "tool_use":
                        self.is_streaming_tool = True
                elif message.type == "content_block_delta":
                    delta = message.delta or {}
                    if delta.get("type") == "thinking_delta":
                        self.accumulated_thinking += delta.get("thinking", "")
                    elif delta.get("type") == "signature_delta":
                        self.accumulated_signature += delta.get("signature", "")

                if self.is_streaming_tool and message.type == "content_block_stop":
                    self.is_streaming_tool = False

            elif message.role == "assistant":
                if self.stream_message_id:
                    message.message_id = self.stream_message_id
                self.parent_id = message.message_id

                if isinstance(message.content, list):
                    message.content = self._merge_assistant_stream_content(message.content)

        if isinstance(message, StreamMessage) and message.type == "message_stop":
            self.is_streaming = False
            self.stream_message_id = None
            self.accumulated_content_blocks = []

    def _merge_assistant_stream_content(self, incoming_blocks: List[Any]) -> List[Dict[str, Any]]:
        """合并同一条流式 assistant 消息的内容块。"""
        merged_blocks = list(self.accumulated_content_blocks)

        for block in incoming_blocks:
            self._upsert_content_block(merged_blocks, block)

        if self.accumulated_thinking:
            self._upsert_content_block(
                merged_blocks,
                {
                    "type": "thinking",
                    "thinking": self.accumulated_thinking,
                    "signature": self.accumulated_signature,
                },
            )

        self._move_thinking_to_front(merged_blocks)
        self.accumulated_content_blocks = merged_blocks
        return list(merged_blocks)

    @staticmethod
    def _upsert_content_block(content_blocks: List[Dict[str, Any]], new_block: Any) -> None:
        """按块类型做幂等更新。"""
        normalized_block = sdk_message_processor._to_plain_block(new_block)
        block_type = normalized_block.get("type")

        if block_type == "thinking":
            for index, block in enumerate(content_blocks):
                if block.get("type") == "thinking":
                    content_blocks[index] = normalized_block
                    return
            content_blocks.insert(0, normalized_block)
            return

        if block_type == "tool_use":
            for index, block in enumerate(content_blocks):
                if block.get("type") == "tool_use" and block.get("id") == normalized_block.get("id"):
                    content_blocks[index] = normalized_block
                    return
            content_blocks.append(normalized_block)
            return

        if block_type == "tool_result":
            for index, block in enumerate(content_blocks):
                if block.get("type") == "tool_result" and block.get("tool_use_id") == normalized_block.get("tool_use_id"):
                    content_blocks[index] = normalized_block
                    return
            content_blocks.append(normalized_block)
            return

        if block_type == "text":
            for block in content_blocks:
                if block.get("type") == "text" and block.get("text") == normalized_block.get("text"):
                    return
            content_blocks.append(normalized_block)
            return

        content_blocks.append(normalized_block)

    @staticmethod
    def _move_thinking_to_front(content_blocks: List[Dict[str, Any]]) -> None:
        """确保 thinking 始终位于首位。"""
        thinking_index: Optional[int] = None
        for index, block in enumerate(content_blocks):
            if block.get("type") == "thinking":
                thinking_index = index
                break
        if thinking_index is None or thinking_index == 0:
            return
        thinking_block = content_blocks.pop(thinking_index)
        content_blocks.insert(0, thinking_block)

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
