# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_message_processor.py
# @Date   ：2026/3/14 13:45
# @Author ：leemysw
# 2026/3/14 13:45   Create
# =====================================================

"""单轮聊天消息处理器。"""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass, field
from typing import Any, Dict, Optional

from claude_agent_sdk import Message as SDKMessage
from claude_agent_sdk import ResultMessage, SystemMessage
from claude_agent_sdk.types import AssistantMessage, StreamEvent, UserMessage

from agent.schema.model_message import Message
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


class ChatMessageProcessor:
    """负责单轮消息转换、聚合与落盘。"""

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

    async def process_messages(self, response_msg: SDKMessage) -> list[Message]:
        """处理响应消息并返回可直接消费的消息快照。"""
        self._print_message(response_msg)
        self._set_subtype(response_msg)
        await self._set_session_id(response_msg)
        await self._save_user_message()

        message = self._build_message(response_msg)
        if message is None:
            return []

        await session_store.save_message(message)
        self.message_count += 1
        return [message]

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

    def _build_message(self, response_msg: SDKMessage) -> Optional[Message]:
        """将 SDK 消息聚合为统一 Message。"""
        if isinstance(response_msg, SystemMessage):
            return None
        if isinstance(response_msg, StreamEvent):
            return self._consume_stream_event(response_msg)
        if isinstance(response_msg, AssistantMessage):
            return self._consume_assistant_message(response_msg)
        if isinstance(response_msg, UserMessage):
            return self._consume_tool_result_message(response_msg)
        if isinstance(response_msg, ResultMessage):
            return self._build_result_message(response_msg)
        raise ValueError(f"Unsupported SDK message type: {type(response_msg)}")

    def _consume_stream_event(self, response_msg: StreamEvent) -> Optional[Message]:
        """消费流式事件并产出 assistant 快照。"""
        raw = self._to_plain_dict(response_msg)
        event = raw.get("event") if isinstance(raw.get("event"), dict) else raw
        event_type = str(event.get("type") or "")
        if not event_type:
            return None

        if event_type == "message_start":
            message_payload = event.get("message") or {}
            self._ensure_assistant_draft()
            self.draft.model = message_payload.get("model") or self.draft.model
            self.draft.usage = message_payload.get("usage") or self.draft.usage
            return self._build_assistant_snapshot()

        if event_type == "content_block_start":
            self._ensure_assistant_draft()
            index = event.get("index")
            block = self._to_plain_block(event.get("content_block"))
            if isinstance(index, int):
                self._set_content_block(index, block)
                return self._build_assistant_snapshot()
            return None

        if event_type == "content_block_delta":
            self._ensure_assistant_draft()
            index = event.get("index")
            delta = event.get("delta") or {}
            if not isinstance(index, int):
                return None
            if self._apply_content_delta(index, delta):
                return self._build_assistant_snapshot()
            return None

        if event_type == "message_delta":
            self._ensure_assistant_draft()
            delta = event.get("delta") or {}
            self.draft.stop_reason = delta.get("stop_reason") or self.draft.stop_reason
            self.draft.usage = event.get("usage") or self.draft.usage
            return self._build_assistant_snapshot()

        return None

    def _consume_assistant_message(self, response_msg: AssistantMessage) -> Message:
        """消费 assistant 完整消息。"""
        raw = self._to_plain_dict(response_msg)
        self._ensure_assistant_draft()
        self.draft.content = self._normalize_content_blocks(raw.get("content"))
        self.draft.model = raw.get("model") or self.draft.model
        self.draft.stop_reason = raw.get("stop_reason") or self.draft.stop_reason
        self.draft.usage = raw.get("usage") or self.draft.usage
        return self._build_assistant_snapshot(is_complete=True)

    def _consume_tool_result_message(self, response_msg: UserMessage) -> Optional[Message]:
        """消费工具结果回灌消息。"""
        raw = self._to_plain_dict(response_msg)
        content_blocks = self._normalize_content_blocks(raw.get("content"))
        if not content_blocks:
            return None
        if not all(block.get("type") == "tool_result" for block in content_blocks):
            return None

        self._ensure_assistant_draft()
        for block in content_blocks:
            self._merge_or_append_block(block)
        return self._build_assistant_snapshot()

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
            parent_id=self.draft.message_id or self.round_id,
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

    def _build_assistant_snapshot(self, is_complete: bool = False) -> Message:
        """根据当前草稿生成 assistant 快照。"""
        self._ensure_assistant_draft()
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
            is_complete=is_complete,
        )

    def _ensure_assistant_draft(self) -> None:
        """确保 assistant 草稿已初始化。"""
        if not self.round_id:
            self.round_id = str(uuid.uuid4())
        if not self.draft.message_id:
            self.draft.message_id = str(uuid.uuid4())

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

    def _merge_or_append_block(self, incoming_block: Dict[str, Any]) -> None:
        """将新内容块幂等合入 assistant 草稿。"""
        block_type = incoming_block.get("type")
        if block_type == "thinking":
            self._replace_matching_block(
                lambda item: item.get("type") == "thinking",
                incoming_block,
                insert_front=True,
            )
            return
        if block_type == "tool_use":
            self._replace_matching_block(
                lambda item: item.get("type") == "tool_use" and item.get("id") == incoming_block.get("id"),
                incoming_block,
            )
            return
        if block_type == "tool_result":
            self._replace_matching_block(
                lambda item: item.get("type") == "tool_result"
                and item.get("tool_use_id") == incoming_block.get("tool_use_id"),
                incoming_block,
            )
            return
        if block_type == "text":
            self._replace_matching_block(
                lambda item: item.get("type") == "text" and item.get("text") == incoming_block.get("text"),
                incoming_block,
            )
            return
        self.draft.content.append(incoming_block)

    def _replace_matching_block(
        self,
        match_func,
        incoming_block: Dict[str, Any],
        insert_front: bool = False,
    ) -> None:
        """替换匹配内容块，不存在时追加。"""
        for index, current_block in enumerate(self.draft.content):
            if match_func(current_block):
                self.draft.content[index] = incoming_block
                return

        if insert_front:
            self.draft.content.insert(0, incoming_block)
            return
        self.draft.content.append(incoming_block)

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
            return dict(block)
        if hasattr(block, "model_dump"):
            return block.model_dump(mode="json", exclude_none=True)
        if hasattr(block, "__dict__"):
            return dict(block.__dict__)
        return {"type": "text", "text": str(block)}

    def _print_message(self, message: SDKMessage) -> None:
        """打印 SDK 消息，便于跟踪执行过程。"""
        logger.debug(
            "📨 SDK message: session=%s type=%s payload=%s",
            self.session_key,
            type(message).__name__,
            self._to_plain_dict(message),
        )
