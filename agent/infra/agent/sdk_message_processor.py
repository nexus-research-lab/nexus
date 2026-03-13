# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：sdk_message_processor.py
# @Date   ：2026/3/14 11:58
# @Author ：leemysw
# 2026/3/14 11:58   Create
# =====================================================

"""Claude SDK 消息转换器。"""

from __future__ import annotations

import json
import uuid
from datetime import datetime
from typing import Any, Dict, List, Optional

from claude_agent_sdk import Message as SDKMessage
from claude_agent_sdk import ResultMessage, SystemMessage
from claude_agent_sdk.types import (
    AssistantMessage,
    StreamEvent,
    TextBlock,
    ThinkingBlock,
    ToolResultBlock,
    ToolUseBlock,
    UserMessage,
)

from agent.schema.model_message import Message, StreamMessage


class SDKMessageProcessor:
    """负责将 Claude SDK 消息映射为统一协议。"""

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
            return [self.build_stream_message(message, session_key, agent_id, session_id, round_id)]
        if isinstance(message, AssistantMessage):
            return [
                self.build_assistant_message(
                    message,
                    session_key,
                    agent_id,
                    session_id,
                    round_id,
                    parent_id,
                )
            ]
        if isinstance(message, UserMessage):
            return [
                self.build_user_or_tool_result_message(
                    message,
                    session_key,
                    agent_id,
                    session_id,
                    round_id,
                    parent_id,
                )
            ]
        if isinstance(message, ResultMessage):
            return [
                self.build_result_message(
                    message,
                    session_key,
                    agent_id,
                    session_id,
                    round_id,
                    parent_id,
                )
            ]
        raise ValueError(f"Unsupported SDK message type: {type(message)}")

    @staticmethod
    def to_plain_dict(payload: Any) -> Dict[str, Any]:
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
    def to_plain_block(block: Any) -> Dict[str, Any]:
        """将 SDK 内容块统一转换为字典。"""
        if isinstance(block, dict):
            return dict(block)
        if hasattr(block, "model_dump"):
            return block.model_dump(mode="json", exclude_none=True)
        if isinstance(block, TextBlock):
            return {"type": "text", "text": block.text}
        if isinstance(block, ThinkingBlock):
            return {
                "type": "thinking",
                "thinking": block.thinking,
                "signature": getattr(block, "signature", None),
            }
        if isinstance(block, ToolUseBlock):
            return {
                "type": "tool_use",
                "id": block.id,
                "name": block.name,
                "input": block.input or {},
            }
        if isinstance(block, ToolResultBlock):
            return {
                "type": "tool_result",
                "tool_use_id": block.tool_use_id,
                "content": block.content,
                "is_error": bool(getattr(block, "is_error", False)),
            }
        return {"type": "text", "text": str(block)}

    def normalize_content_blocks(self, content: Any) -> List[Dict[str, Any]]:
        """统一规范化内容块列表。"""
        if isinstance(content, str):
            return [{"type": "text", "text": content}]
        if isinstance(content, list):
            return [self.to_plain_block(block) for block in content]
        if content is None:
            return []
        return [{"type": "text", "text": str(content)}]

    def build_assistant_message(
        self,
        message: AssistantMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str],
    ) -> Message:
        """构建助手消息。"""
        raw = self.to_plain_dict(message)
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            parent_id=parent_id,
            role="assistant",
            content=self.normalize_content_blocks(raw.get("content")),
            model=raw.get("model"),
            stop_reason=raw.get("stop_reason"),
            usage=raw.get("usage"),
            parent_tool_use_id=raw.get("parent_tool_use_id"),
        )

    def build_user_or_tool_result_message(
        self,
        message: UserMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str],
    ) -> Message:
        """构建用户消息或工具结果消息。"""
        raw = self.to_plain_dict(message)
        blocks = self.normalize_content_blocks(raw.get("content"))
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

    def build_result_message(
        self,
        message: ResultMessage,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: Optional[str],
    ) -> Message:
        """构建结果消息。"""
        raw = self.to_plain_dict(message)
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

    def build_stream_message(
        self,
        message: StreamEvent,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
    ) -> StreamMessage:
        """构建流式消息。"""
        raw = self.to_plain_dict(message)
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
            content_block=self.to_plain_block(event["content_block"]) if event.get("content_block") else None,
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

        raw = self.to_plain_dict(message)
        print(json.dumps(raw, ensure_ascii=False, indent=2))
        print("=" * 80)


sdk_message_processor = SDKMessageProcessor()
