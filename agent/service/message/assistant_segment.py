# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：assistant_segment.py
# @Date   ：2026/03/17 20:05
# @Author ：leemysw
# 2026/03/17 20:05   Create
# =====================================================

"""Assistant 段状态。"""

from __future__ import annotations

import uuid
from typing import Any, Dict, Optional

from agent.schema.model_message import Message, StreamMessage, current_timestamp_ms


class AssistantSegment:
    """维护单段 assistant 输出的最小状态。"""

    def __init__(self) -> None:
        self.reset()

    def reset(self) -> None:
        """重置当前段状态。"""
        self.message_id: Optional[str] = None
        self.content: list[Dict[str, Any]] = []
        self.model: Optional[str] = None
        self.stop_reason: Optional[str] = None
        self.usage: Optional[Dict[str, Any]] = None
        self.timestamp: Optional[int] = None

    def start(
        self,
        message_id: Optional[str] = None,
        model: Optional[str] = None,
        usage: Optional[Dict[str, Any]] = None,
        timestamp: Optional[int] = None,
    ) -> None:
        """开始一段新的 assistant 输出。"""
        self.reset()
        self.message_id = message_id or str(uuid.uuid4())
        self.model = model
        self.usage = usage
        self.timestamp = timestamp or current_timestamp_ms()

    def ensure_started(self) -> None:
        """确保当前段已经初始化。"""
        if self.message_id:
            return
        self.start()

    def apply_block(self, index: int, block: Dict[str, Any]) -> None:
        """按索引写入内容块。"""
        self.ensure_started()
        while len(self.content) <= index:
            self.content.append({"type": "text", "text": ""})
        self.content[index] = dict(block)

    def apply_delta(self, index: int, delta: Dict[str, Any]) -> bool:
        """将流式增量写入当前内容块。"""
        self.ensure_started()
        while len(self.content) <= index:
            self.content.append({"type": "text", "text": ""})

        block = dict(self.content[index])
        delta_type = delta.get("type")
        if block.get("type") == "text" and delta_type == "text_delta":
            block["text"] = f"{block.get('text', '')}{delta.get('text', '')}"
            self.content[index] = block
            return True

        if block.get("type") == "thinking" and delta_type == "thinking_delta":
            block["thinking"] = f"{block.get('thinking', '')}{delta.get('thinking', '')}"
            self.content[index] = block
            return True

        if block.get("type") == "thinking" and delta_type == "signature_delta":
            block["signature"] = f"{block.get('signature', '')}{delta.get('signature', '')}"
            self.content[index] = block
            return True

        return False

    def update_message_meta(
        self,
        model: Optional[str] = None,
        usage: Optional[Dict[str, Any]] = None,
        stop_reason: Optional[str] = None,
    ) -> None:
        """更新消息级元信息。"""
        if model is not None:
            self.model = model
        if usage is not None:
            self.usage = usage
        if stop_reason is not None:
            self.stop_reason = stop_reason

    def replace_from_snapshot(
        self,
        content: list[Dict[str, Any]],
        model: Optional[str] = None,
        usage: Optional[Dict[str, Any]] = None,
        stop_reason: Optional[str] = None,
    ) -> None:
        """使用 SDK assistant 快照补齐当前段状态。"""
        self.ensure_started()
        if not self.content:
            self.content = [dict(block) for block in content]
        else:
            for block in content:
                self._upsert_block(dict(block))
        self.update_message_meta(model=model, usage=usage, stop_reason=stop_reason)

    def append_tool_results(self, content: list[Dict[str, Any]]) -> None:
        """将工具结果回填到当前段。"""
        self.ensure_started()
        for block in content:
            self._upsert_block(dict(block))

    def append_task_progress(self, block: Dict[str, Any]) -> None:
        """写入或更新任务进度块。"""
        self.ensure_started()
        self._upsert_block(dict(block))

    def find_tool_name(self, tool_use_id: str) -> str:
        """根据 tool_use_id 反查工具名称。"""
        for block in self.content:
            if block.get("type") != "tool_use":
                continue
            if block.get("id") == tool_use_id:
                return str(block.get("name") or "")
        return ""

    def has_content(self) -> bool:
        """判断当前段是否已有内容。"""
        return bool(self.content)

    def _upsert_block(self, incoming_block: Dict[str, Any]) -> None:
        """按块身份更新或追加内容块。"""
        incoming_type = incoming_block.get("type")
        for index, current_block in enumerate(self.content):
            current_type = current_block.get("type")
            if current_type != incoming_type:
                continue
            if incoming_type == "thinking":
                self.content[index] = incoming_block
                return
            if incoming_type == "tool_use" and current_block.get("id") == incoming_block.get("id"):
                self.content[index] = incoming_block
                return
            if incoming_type == "tool_result" and current_block.get("tool_use_id") == incoming_block.get("tool_use_id"):
                self.content[index] = incoming_block
                return
            if incoming_type == "task_progress" and current_block.get("task_id") == incoming_block.get("task_id"):
                self.content[index] = incoming_block
                return
            if incoming_type == "text" and current_block.get("text") == incoming_block.get("text"):
                self.content[index] = incoming_block
                return
        self.content.append(incoming_block)

    def build_stream_message(
        self,
        session_key: str,
        agent_id: str,
        round_id: str,
        session_id: Optional[str],
        stream_type: str,
        index: Optional[int] = None,
        content_block: Optional[Dict[str, Any]] = None,
        message: Optional[Dict[str, Any]] = None,
        usage: Optional[Dict[str, Any]] = None,
    ) -> StreamMessage:
        """构建流式消息。"""
        self.ensure_started()
        return StreamMessage(
            message_id=self.message_id or str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            type=stream_type,
            index=index,
            content_block=content_block,
            message=message or {},
            usage=usage,
        )

    def build_message(
        self,
        session_key: str,
        agent_id: str,
        round_id: str,
        session_id: Optional[str],
        parent_id: Optional[str],
        is_complete: bool,
    ) -> Message:
        """构建 assistant 完整消息。"""
        self.ensure_started()
        return Message(
            message_id=self.message_id or str(uuid.uuid4()),
            session_key=session_key,
            agent_id=agent_id,
            round_id=round_id,
            session_id=session_id,
            parent_id=parent_id,
            role="assistant",
            timestamp=self.timestamp or current_timestamp_ms(),
            content=[dict(block) for block in self.content],
            model=self.model,
            stop_reason=self.stop_reason,
            usage=self.usage,
            is_complete=is_complete,
        )
