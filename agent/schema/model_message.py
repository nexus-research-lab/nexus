# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_message.py
# @Date   ：2026/3/14 00:22
# @Author ：leemysw
# 2026/3/14 00:22   Create
# =====================================================

"""统一消息协议模型。"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List, Literal, Optional, Union

from pydantic import BaseModel, Field, field_validator


def current_timestamp_ms() -> int:
    """返回当前毫秒时间戳。"""
    return int(datetime.now(timezone.utc).timestamp() * 1000)


def current_envelope_id() -> str:
    """返回 envelope 唯一标识。"""
    return str(uuid.uuid4())


class TextContent(BaseModel):
    """文本内容块。"""

    type: Literal["text"] = "text"
    text: str = Field(default="", description="文本内容")


class ToolUseContent(BaseModel):
    """工具调用内容块。"""

    type: Literal["tool_use"] = "tool_use"
    id: str = Field(..., description="工具调用 ID")
    name: str = Field(..., description="工具名称")
    input: Dict[str, Any] = Field(default_factory=dict, description="工具输入")


class ToolResultContent(BaseModel):
    """工具结果内容块。"""

    type: Literal["tool_result"] = "tool_result"
    tool_use_id: str = Field(..., description="关联工具调用 ID")
    content: Union[str, List[Any]] = Field(default="", description="工具结果内容")
    is_error: bool = Field(default=False, description="是否错误")
    error_code: Optional[str] = Field(default=None, description="结构化错误码")


class ThinkingContent(BaseModel):
    """思考内容块。"""

    type: Literal["thinking"] = "thinking"
    thinking: str = Field(default="", description="思考内容")
    signature: Optional[str] = Field(default=None, description="思考签名")


class TaskProgressContent(BaseModel):
    """任务进度内容块。"""

    type: Literal["task_progress"] = "task_progress"
    task_id: str = Field(..., description="后台任务 ID")
    description: str = Field(default="", description="当前进度描述")
    tool_use_id: Optional[str] = Field(default=None, description="关联工具调用 ID")
    last_tool_name: Optional[str] = Field(default=None, description="最近执行的工具")
    usage: Dict[str, Any] = Field(default_factory=dict, description="进度统计信息")


ContentBlock = Union[
    TextContent,
    ToolUseContent,
    ToolResultContent,
    ThinkingContent,
    TaskProgressContent,
]


class Usage(BaseModel):
    """模型调用用量。"""

    input_tokens: int = Field(default=0, description="输入 token")
    output_tokens: int = Field(default=0, description="输出 token")
    cache_creation_input_tokens: Optional[int] = Field(default=None, description="缓存创建 token")
    cache_read_input_tokens: Optional[int] = Field(default=None, description="缓存命中 token")

    model_config = {"extra": "allow"}


class Message(BaseModel):
    """统一完整消息。"""

    message_id: str = Field(..., description="消息 ID")
    session_key: str = Field(..., description="会话路由键")
    room_id: Optional[str] = Field(default=None, description="Room ID")
    conversation_id: Optional[str] = Field(default=None, description="对话 ID")
    agent_id: str = Field(default="", description="Agent ID")
    round_id: str = Field(..., description="轮次 ID")
    session_id: Optional[str] = Field(default=None, description="SDK Session ID")
    parent_id: Optional[str] = Field(default=None, description="父消息 ID")
    role: Literal["user", "assistant", "system", "result"] = Field(..., description="消息角色")
    timestamp: int = Field(default_factory=current_timestamp_ms, description="毫秒时间戳")
    content: Optional[Union[str, List[ContentBlock]]] = Field(default=None, description="消息内容")
    stop_reason: Optional[str] = Field(default=None, description="停止原因")
    model: Optional[str] = Field(default=None, description="模型名称")
    usage: Optional[Usage] = Field(default=None, description="用量信息")
    is_complete: bool = Field(default=True, description="消息是否已完成")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="系统元数据")
    subtype: Optional[Literal["success", "error", "interrupted"]] = Field(
        default=None,
        description="结果类型",
    )
    duration_ms: int = Field(default=0, description="总耗时")
    duration_api_ms: int = Field(default=0, description="API 耗时")
    num_turns: int = Field(default=0, description="轮次数")
    total_cost_usd: Optional[float] = Field(default=None, description="总成本")
    result: Optional[str] = Field(default=None, description="结果文本")
    is_error: bool = Field(default=False, description="是否错误")
    model_config = {"extra": "allow"}

    @field_validator("content", mode="before")
    @classmethod
    def _validate_content(cls, value: Any) -> Any:
        """将内容块统一解析为协议模型。"""
        if value is None or isinstance(value, str):
            return value
        if not isinstance(value, list):
            return value
        return [
            parse_content_block(item) if isinstance(item, dict) else item
            for item in value
        ]

    @field_validator("usage", mode="before")
    @classmethod
    def _validate_usage(cls, value: Any) -> Any:
        """将 usage 统一解析为模型。"""
        if value is None or isinstance(value, Usage):
            return value
        if isinstance(value, dict):
            return Usage(**value)
        return value


class StreamMessage(BaseModel):
    """统一流式消息。"""

    message_id: str = Field(..., description="目标消息 ID")
    session_key: str = Field(..., description="会话路由键")
    room_id: Optional[str] = Field(default=None, description="Room ID")
    conversation_id: Optional[str] = Field(default=None, description="对话 ID")
    agent_id: str = Field(default="", description="Agent ID")
    round_id: str = Field(..., description="轮次 ID")
    session_id: Optional[str] = Field(default=None, description="SDK Session ID")
    type: Literal[
        "message_start",
        "content_block_start",
        "content_block_delta",
        "message_delta",
        "message_stop",
    ] = Field(..., description="流式事件类型")
    index: Optional[int] = Field(default=None, description="内容块索引")
    content_block: Optional[ContentBlock] = Field(default=None, description="内容块快照")
    message: Dict[str, Any] = Field(default_factory=dict, description="消息级别元数据")
    usage: Optional[Usage] = Field(default=None, description="用量信息")
    timestamp: int = Field(default_factory=current_timestamp_ms, description="毫秒时间戳")

    model_config = {"extra": "allow"}

    @field_validator("content_block", mode="before")
    @classmethod
    def _validate_content_block(cls, value: Any) -> Any:
        """将内容块统一解析为协议模型。"""
        if value is None or not isinstance(value, dict):
            return value
        return parse_content_block(value)

    @field_validator("usage", mode="before")
    @classmethod
    def _validate_usage(cls, value: Any) -> Any:
        """将 usage 统一解析为模型。"""
        if value is None or isinstance(value, Usage):
            return value
        if isinstance(value, dict):
            return Usage(**value)
        return value


class EventMessage(BaseModel):
    """事件消息。"""

    envelope_id: str = Field(default_factory=current_envelope_id, description="Envelope ID")
    protocol_version: int = Field(default=2, description="协议版本")
    delivery_mode: Literal["durable", "ephemeral"] = Field(
        default="durable",
        description="投递模式",
    )
    event_type: str = Field(..., description="事件类型")
    session_key: Optional[str] = Field(default=None, description="会话路由键")
    session_seq: Optional[int] = Field(default=None, description="Session 内递增序号")
    room_id: Optional[str] = Field(default=None, description="Room ID")
    room_seq: Optional[int] = Field(default=None, description="Room 内递增序号")
    conversation_id: Optional[str] = Field(default=None, description="对话 ID")
    agent_id: Optional[str] = Field(default=None, description="Agent ID")
    message_id: Optional[str] = Field(default=None, description="关联消息 ID")
    session_id: Optional[str] = Field(default=None, description="SDK Session ID")
    caused_by: Optional[str] = Field(default=None, description="上游事件或轮次 ID")
    data: Dict[str, Any] = Field(default_factory=dict, description="事件载荷")
    timestamp: int = Field(default_factory=current_timestamp_ms, description="毫秒时间戳")

    model_config = {"extra": "allow"}


RoundLifecycleStatus = Literal["running", "finished", "interrupted", "error"]


def build_transport_event(message: Message | StreamMessage) -> EventMessage:
    """将完整消息或流式消息包装为统一 Envelope。"""
    event_type = "message" if isinstance(message, Message) else "stream"
    return EventMessage(
        event_type=event_type,
        session_key=message.session_key,
        room_id=message.room_id,
        conversation_id=message.conversation_id,
        agent_id=message.agent_id,
        message_id=message.message_id,
        session_id=message.session_id,
        data=message.model_dump(mode="json", exclude_none=True),
    )


def parse_content_block(payload: Dict[str, Any]) -> ContentBlock:
    """将字典解析为内容块模型。"""
    normalized_payload = dict(payload)
    block_type = normalized_payload.get("type")
    if block_type == "server_tool_use":
        normalized_payload["type"] = "tool_use"
        block_type = "tool_use"
    elif block_type == "server_tool_result":
        normalized_payload["type"] = "tool_result"
        block_type = "tool_result"

    if block_type == "tool_use":
        return ToolUseContent(**normalized_payload)
    if block_type == "tool_result":
        return ToolResultContent(**normalized_payload)
    if block_type == "thinking":
        return ThinkingContent(**normalized_payload)
    if block_type == "task_progress":
        return TaskProgressContent(**normalized_payload)
    return TextContent(**normalized_payload)


def parse_message(payload: Dict[str, Any]) -> Message:
    """将字典解析为完整消息模型。"""
    return Message(**payload)


def build_error_event(
    error_type: str,
    message: str,
    session_key: Optional[str] = None,
    room_id: Optional[str] = None,
    conversation_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    message_id: Optional[str] = None,
    session_id: Optional[str] = None,
    caused_by: Optional[str] = None,
    details: Optional[Dict[str, Any]] = None,
) -> EventMessage:
    """构造统一错误事件。"""
    return EventMessage(
        event_type="error",
        session_key=session_key,
        room_id=room_id,
        conversation_id=conversation_id,
        agent_id=agent_id,
        message_id=message_id,
        session_id=session_id,
        caused_by=caused_by,
        data={
            "error_type": error_type,
            "message": message,
            "details": details or {},
        },
    )


def build_round_status_event(
    round_id: str,
    status: RoundLifecycleStatus,
    session_key: Optional[str] = None,
    room_id: Optional[str] = None,
    conversation_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    message_id: Optional[str] = None,
    session_id: Optional[str] = None,
    caused_by: Optional[str] = None,
    result_subtype: Optional[str] = None,
) -> EventMessage:
    """构造统一 round 生命周期事件。"""
    data: Dict[str, Any] = {
        "round_id": round_id,
        "status": status,
        "is_terminal": status != "running",
    }
    if result_subtype:
        data["result_subtype"] = result_subtype

    return EventMessage(
        event_type="round_status",
        delivery_mode="durable",
        session_key=session_key,
        room_id=room_id,
        conversation_id=conversation_id,
        agent_id=agent_id,
        message_id=message_id,
        session_id=session_id,
        caused_by=caused_by or round_id,
        data=data,
    )
