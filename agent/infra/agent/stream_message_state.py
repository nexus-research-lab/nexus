# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：stream_message_state.py
# @Date   ：2026/3/14 11:59
# @Author ：leemysw
# 2026/3/14 11:59   Create
# =====================================================

"""流式消息状态管理。"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from agent.schema.model_message import Message, StreamMessage


@dataclass
class StreamMessageState:
    """维护单轮流式消息的累积状态。"""

    is_streaming: bool = False
    is_streaming_tool: bool = False
    stream_message_id: Optional[str] = None
    accumulated_thinking: str = ""
    accumulated_signature: str = ""
    accumulated_content_blocks: List[Dict[str, Any]] = field(default_factory=list)

    def apply(self, message: Message | StreamMessage) -> None:
        """根据消息更新流式状态。"""
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
                    block = self._to_dict(message.content_block) if message.content_block else {}
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

            elif message.role == "assistant" and isinstance(message.content, list):
                if self.stream_message_id:
                    message.message_id = self.stream_message_id
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

    def _upsert_content_block(self, content_blocks: List[Dict[str, Any]], new_block: Any) -> None:
        """按块类型做幂等更新。"""
        normalized_block = self._to_dict(new_block)
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

    @staticmethod
    def _to_dict(payload: Any) -> Dict[str, Any]:
        """将内容块统一转为字典。"""
        if payload is None:
            return {}
        if isinstance(payload, dict):
            return dict(payload)
        if hasattr(payload, "model_dump"):
            return payload.model_dump(mode="json", exclude_none=True)
        if hasattr(payload, "__dict__"):
            return dict(payload.__dict__)
        return {"type": "text", "text": str(payload)}
