# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_message_processor.py
# @Date   ：2026/3/14 16:20
# @Author ：leemysw
# 2026/3/14 16:20   Create
# =====================================================

"""单轮聊天消息处理器。"""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass, field
from typing import Any, Dict, Optional

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
from agent.service.message.sdk_message_processor import message_vis
from agent.service.session.session_manager import session_manager
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


@dataclass
class AssistantDraft:
    """维护单轮 assistant 消息的流式草稿。"""

    message_id: Optional[str] = None
    content: list[Dict[str, Any]] = field(default_factory=list)
    model: Optional[str] = None
    stop_reason: Optional[str] = None
    usage: Optional[Dict[str, Any]] = None
    partial_tool_inputs: Dict[int, str] = field(default_factory=dict)
    is_open: bool = False


class ChatMessageProcessor:
    """负责单轮消息转换、流式 patch 生成与最终落盘。"""

    def __init__(
            self,
            session_key: str,
            query: str,
            round_id: Optional[str] = None,
            agent_id: str = "main",
            session_id: Optional[str] = None,
    ):
        self.query = query
        self.session_key = session_key
        self.agent_id = agent_id or "main"
        self.subtype: Optional[str] = None
        self.round_id: Optional[str] = round_id
        self.session_id: Optional[str] = session_id
        self.message_count: int = 0
        self.is_save_user_message: bool = False
        self.draft = AssistantDraft()
        self.last_assistant_message_id: Optional[str] = None

    async def process_messages(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """处理响应消息并返回规范化消息。"""
        self._print_message(response_msg)
        message_vis.print_message(response_msg)
        self._set_subtype(response_msg)
        await self._set_session_id(response_msg)
        await self._save_user_message()

        processed_messages = self._build_messages(response_msg)
        for message in processed_messages:
            if isinstance(message, Message):
                await session_store.save_message(message)
            self.message_count += 1
        return processed_messages

    async def _set_session_id(self, response_msg: SDKMessage) -> Optional[str]:
        """处理 session 映射关系。"""
        if self.session_id is not None:
            return self.session_id

        if not isinstance(response_msg, SystemMessage):
            return None

        raw = self._to_plain_dict(response_msg)
        data = raw.get("data") or {}
        self.session_id = data.get("session_id")
        if not self.session_id:
            return None
        await session_manager.register_sdk_session(
            session_key=self.session_key,
            session_id=self.session_id,
        )
        logger.debug(f"🔗建立映射: key={self.session_key} ↔ sdk_session={self.session_id}")
        return self.session_id

    def _set_subtype(self, response_msg: SDKMessage) -> None:
        """同步记录 result subtype。"""
        raw = self._to_plain_dict(response_msg)
        subtype = raw.get("subtype")
        if subtype:
            self.subtype = str(subtype)
        if isinstance(response_msg, ResultMessage):
            self.subtype = "success" if self.subtype == "success" else "error"

    async def _save_user_message(self) -> None:
        """保存当前轮次的用户消息。"""
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
            content=self.query,
        )
        await session_store.save_message(user_message)
        self.is_save_user_message = True

    def _build_messages(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """将 SDK 消息转换为统一协议。"""
        if isinstance(response_msg, SystemMessage):
            return []
        if isinstance(response_msg, StreamEvent):
            return self._consume_stream_event(response_msg)
        if isinstance(response_msg, AssistantMessage):
            message = self._consume_assistant_message(response_msg)
            return [message] if message else []
        if isinstance(response_msg, UserMessage):
            return self._consume_tool_result_messages(response_msg)
        if isinstance(response_msg, ResultMessage):
            return [self._build_result_message(response_msg)]
        raise ValueError(f"Unsupported SDK message type: {type(response_msg)}")

    def _consume_stream_event(self, response_msg: StreamEvent) -> list[Message | StreamMessage]:
        """消费流式事件并产出标准化 patch。"""
        raw = self._to_plain_dict(response_msg)
        event = raw.get("event") if isinstance(raw.get("event"), dict) else raw
        event_type = str(event.get("type") or "")
        if not event_type:
            return []

        if event_type == "message_start":
            message_payload = event.get("message") or {}
            self._start_new_draft()
            self.draft.model = message_payload.get("model") or self.draft.model
            self.draft.usage = message_payload.get("usage") or self.draft.usage
            return [
                self._build_stream_message(
                    stream_type="message_start",
                    message={
                        "model": self.draft.model,
                    },
                    usage=self.draft.usage,
                )
            ]

        if event_type == "content_block_start":
            self._ensure_assistant_draft()
            index = event.get("index")
            if not isinstance(index, int):
                return []
            block = self._to_plain_block(event.get("content_block"))
            self._set_content_block(index, block)
            if block.get("type") == "tool_use":
                return []
            return [
                self._build_stream_message(
                    stream_type="content_block_start",
                    index=index,
                    content_block=block,
                )
            ]

        if event_type == "content_block_delta":
            self._ensure_assistant_draft()
            index = event.get("index")
            if not isinstance(index, int):
                return []
            delta = event.get("delta") or {}
            if not self._apply_content_delta(index, delta):
                return []
            current_block = self.draft.content[index]
            if current_block.get("type") == "tool_use":
                return []
            return [
                self._build_stream_message(
                    stream_type="content_block_delta",
                    index=index,
                    content_block=current_block,
                )
            ]

        if event_type == "message_delta":
            self._ensure_assistant_draft()
            delta = event.get("delta") or {}
            self.draft.stop_reason = delta.get("stop_reason") or self.draft.stop_reason
            self.draft.usage = event.get("usage") or self.draft.usage
            return [
                self._build_stream_message(
                    stream_type="message_delta",
                    message={"stop_reason": self.draft.stop_reason},
                    usage=self.draft.usage,
                )
            ]

        if event_type == "message_stop":
            self._ensure_assistant_draft()
            messages: list[Message | StreamMessage] = [
                self._build_stream_message(stream_type="message_stop"),
            ]
            finalized_message = self._finalize_current_draft()
            if finalized_message:
                messages.append(finalized_message)
            return messages

        return []

    def _consume_assistant_message(self, response_msg: AssistantMessage) -> Optional[Message]:
        """消费 assistant 快照，更新当前段的权威内容。"""
        raw = self._to_plain_dict(response_msg)
        self._ensure_assistant_draft()
        self.draft.content = self._merge_content_blocks(
            current_blocks=self.draft.content,
            incoming_blocks=self._normalize_content_blocks(raw.get("content")),
        )
        self.draft.model = raw.get("model") or self.draft.model
        self.draft.stop_reason = raw.get("stop_reason") or self.draft.stop_reason
        self.draft.usage = raw.get("usage") or self.draft.usage
        if not self.draft.content:
            return None
        return Message(
            message_id=self.draft.message_id or str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id or str(uuid.uuid4()),
            session_id=self.session_id,
            parent_id=self.round_id,
            role="assistant",
            content=list(self.draft.content),
            model=self.draft.model,
            stop_reason=self.draft.stop_reason,
            usage=self.draft.usage,
            is_complete=False,
        )

    def _consume_tool_result_messages(self, response_msg: UserMessage) -> list[Message]:
        """消费工具结果回灌，并通过消息快照更新 UI。"""
        raw = self._to_plain_dict(response_msg)
        content_blocks = self._normalize_content_blocks(raw.get("content"))
        if not content_blocks:
            return []
        if not all(block.get("type") == "tool_result" for block in content_blocks):
            return []

        message = Message(
            message_id=str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id or str(uuid.uuid4()),
            session_id=self.session_id,
            parent_id=self._current_parent_message_id(),
            role="assistant",
            content=content_blocks,
            is_complete=True,
        )
        self.last_assistant_message_id = message.message_id
        return [message]

    def _build_result_message(self, response_msg: ResultMessage) -> Message:
        """构建结果消息。"""
        raw = self._to_plain_dict(response_msg)
        subtype = str(raw.get("subtype") or "success")
        normalized_subtype = subtype if subtype in ("success", "error", "interrupted") else "error"
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id or str(uuid.uuid4()),
            session_id=self.session_id,
            parent_id=self._current_parent_message_id(),
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

    def _start_new_draft(self) -> None:
        """开始一段新的 assistant 草稿。"""
        if not self.round_id:
            self.round_id = str(uuid.uuid4())
        self.draft = AssistantDraft(
            message_id=str(uuid.uuid4()),
            is_open=True,
        )

    def _finalize_current_draft(self) -> Optional[Message]:
        """在 message_stop 时收束当前 assistant 段。"""
        if not self.draft.is_open or not self.draft.message_id or not self.draft.content:
            self.draft = AssistantDraft()
            return None

        message = Message(
            message_id=self.draft.message_id,
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id or str(uuid.uuid4()),
            session_id=self.session_id,
            parent_id=self.round_id,
            role="assistant",
            content=list(self.draft.content),
            model=self.draft.model,
            stop_reason=self.draft.stop_reason,
            usage=self.draft.usage,
            is_complete=True,
        )
        self.last_assistant_message_id = message.message_id
        self.draft = AssistantDraft()
        return message

    def _build_stream_message(
            self,
            stream_type: str,
            index: Optional[int] = None,
            content_block: Optional[Dict[str, Any]] = None,
            message: Optional[Dict[str, Any]] = None,
            usage: Optional[Dict[str, Any]] = None,
    ) -> StreamMessage:
        """构建统一流式消息。"""
        self._ensure_assistant_draft()
        return StreamMessage(
            message_id=self.draft.message_id or str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id or str(uuid.uuid4()),
            session_id=self.session_id,
            type=stream_type,
            index=index,
            content_block=content_block,
            message=message or {},
            usage=usage,
        )

    def _ensure_assistant_draft(self) -> None:
        """确保 assistant 草稿已初始化。"""
        if not self.round_id:
            self.round_id = str(uuid.uuid4())
        if not self.draft.message_id:
            self.draft.message_id = str(uuid.uuid4())
        self.draft.is_open = True

    def _current_parent_message_id(self) -> Optional[str]:
        """返回当前应关联的父助手消息 ID。"""
        if self.draft.message_id:
            return self.draft.message_id
        return self.last_assistant_message_id or self.round_id

    def _set_content_block(self, index: int, block: Dict[str, Any]) -> None:
        """按索引写入内容块。"""
        while len(self.draft.content) <= index:
            self.draft.content.append({"type": "text", "text": ""})
        self.draft.content[index] = block

    def _apply_content_delta(self, index: int, delta: Dict[str, Any]) -> bool:
        """将流式增量应用到指定内容块。"""
        while len(self.draft.content) <= index:
            self.draft.content.append({"type": "text", "text": ""})

        block = dict(self.draft.content[index])
        delta_type = delta.get("type")
        if block.get("type") == "text" and delta_type == "text_delta":
            block["text"] = f"{block.get('text', '')}{delta.get('text', '')}"
            self.draft.content[index] = block
            return True
        if block.get("type") == "thinking" and delta_type == "thinking_delta":
            block["thinking"] = f"{block.get('thinking', '')}{delta.get('thinking', '')}"
            self.draft.content[index] = block
            return True
        if block.get("type") == "thinking" and delta_type == "signature_delta":
            block["signature"] = f"{block.get('signature', '')}{delta.get('signature', '')}"
            self.draft.content[index] = block
            return True
        if block.get("type") == "tool_use" and delta_type == "input_json_delta":
            partial_json = f"{self.draft.partial_tool_inputs.get(index, '')}{delta.get('partial_json', '')}"
            self.draft.partial_tool_inputs[index] = partial_json
            try:
                block["input"] = json.loads(partial_json)
                self.draft.content[index] = block
                return True
            except json.JSONDecodeError:
                return False
        return False

    def _upsert_block(self, incoming_block: Dict[str, Any]) -> tuple[int, bool]:
        """将内容块幂等合入草稿，并返回索引和是否新建。"""
        for index, current_block in enumerate(self.draft.content):
            if self._is_same_block(current_block, incoming_block):
                self.draft.content[index] = incoming_block
                return index, False

        self.draft.content.append(incoming_block)
        return len(self.draft.content) - 1, True

    def _merge_content_blocks(
            self,
            current_blocks: list[Dict[str, Any]],
            incoming_blocks: list[Dict[str, Any]],
    ) -> list[Dict[str, Any]]:
        """合并最终 assistant 内容，避免流式块被最终态覆盖。"""
        merged_blocks = [dict(block) for block in current_blocks]
        text_indexes = [
            index
            for index, block in enumerate(merged_blocks)
            if block.get("type") == "text"
        ]
        next_text_index = 0

        for block in incoming_blocks:
            if block.get("type") == "text":
                if next_text_index < len(text_indexes):
                    merged_blocks[text_indexes[next_text_index]] = dict(block)
                    next_text_index += 1
                    continue
                merged_blocks.append(dict(block))
                continue
            self._merge_block_into_list(merged_blocks, block)

        merged_blocks = [
            block
            for block in merged_blocks
            if block.get("type") != "text" or bool(str(block.get("text", "")).strip())
        ]
        self._move_thinking_to_front(merged_blocks)
        return merged_blocks

    def _merge_block_into_list(
            self,
            blocks: list[Dict[str, Any]],
            incoming_block: Dict[str, Any],
    ) -> None:
        """将内容块合入指定列表。"""
        for index, current_block in enumerate(blocks):
            if self._is_same_block(current_block, incoming_block):
                blocks[index] = incoming_block
                return
        blocks.append(incoming_block)

    @staticmethod
    def _move_thinking_to_front(blocks: list[Dict[str, Any]]) -> None:
        """确保 thinking 位于首位。"""
        for index, block in enumerate(blocks):
            if block.get("type") != "thinking":
                continue
            if index == 0:
                return
            thinking_block = blocks.pop(index)
            blocks.insert(0, thinking_block)
            return

    @staticmethod
    def _is_same_block(left: Dict[str, Any], right: Dict[str, Any]) -> bool:
        """判断两个内容块是否代表同一逻辑块。"""
        left_type = left.get("type")
        right_type = right.get("type")
        if left_type != right_type:
            return False
        if left_type == "thinking":
            return True
        if left_type == "tool_use":
            return left.get("id") == right.get("id")
        if left_type == "tool_result":
            return left.get("tool_use_id") == right.get("tool_use_id")
        if left_type == "text":
            return False
        return left == right

    @staticmethod
    def _to_plain_dict(payload: Any) -> Dict[str, Any]:
        """将 SDK 对象转换为普通字典。"""
        if payload is None:
            return {}
        if isinstance(payload, dict):
            return dict(payload)
        if hasattr(payload, "model_dump"):
            return payload.model_dump(mode="json", exclude_none=True)
        if hasattr(payload, "__dict__"):
            return dict(payload.__dict__)
        return {}

    def _normalize_content_blocks(self, content: Any) -> list[Dict[str, Any]]:
        """统一规范化内容块。"""
        if isinstance(content, str):
            return [{"type": "text", "text": content}]
        if isinstance(content, list):
            return [self._to_plain_block(block) for block in content]
        if content is None:
            return []
        return [{"type": "text", "text": str(content)}]

    def _to_plain_block(self, block: Any) -> Dict[str, Any]:
        """将 SDK block 转换为普通字典。"""
        if isinstance(block, dict):
            return self._normalize_block_payload(dict(block))
        if isinstance(block, TextBlock):
            return {
                "type": "text",
                "text": block.text,
            }
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
        if hasattr(block, "model_dump"):
            return self._normalize_block_payload(
                block.model_dump(mode="json", exclude_none=True)
            )
        if hasattr(block, "__dict__"):
            return self._normalize_block_payload(dict(block.__dict__))
        return {"type": "text", "text": str(block)}

    @staticmethod
    def _normalize_block_payload(block: Dict[str, Any]) -> Dict[str, Any]:
        """将 SDK 新增块类型归一到现有消息协议。"""
        normalized_block = dict(block)
        block_type = normalized_block.get("type")
        if block_type == "server_tool_use":
            normalized_block["type"] = "tool_use"
        elif block_type == "server_tool_result":
            normalized_block["type"] = "tool_result"
        return normalized_block

    def _print_message(self, message: SDKMessage) -> None:
        """打印 SDK 消息，便于跟踪执行过程。"""

        if isinstance(message, StreamEvent):
            return

        logger.debug(
            "📨 SDK message: session=%s type=%s payload=%s",
            self.session_key,
            type(message).__name__,
            self._to_plain_dict(message),
        )
