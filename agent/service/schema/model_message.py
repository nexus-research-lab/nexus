# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model
# @Date   ：2025/11/28 09:52
# @Author ：leemysw

# 2025/11/28 09:52   Create
# =====================================================

import uuid
from datetime import datetime
from typing import Any, Dict, Literal, Optional

from claude_agent_sdk.types import AssistantMessage, ResultMessage, StreamEvent, SystemMessage, UserMessage  # noqa
from claude_agent_sdk.types import ContentBlock  # noqa
from claude_agent_sdk.types import Message  # noqa
from claude_agent_sdk.types import TextBlock, ThinkingBlock, ToolResultBlock, ToolUseBlock  # noqa
from pydantic import BaseModel, Field

from agent.config.config import settings


class AMessage(BaseModel):
    """
    自定义消息模型，封装 Claude Agent SDK 消息
    如果 AssistantMessage 和 UserMessage
     - content 是 ContentBlock 且数量大于1，转换为 多条只包含 1 个 ContentBlock 的 Message
     - content 是 str 转换 为 List[TextBlock]
    """
    session_key: str = Field(default="", description="结构化路由键")
    agent_id: str = Field(default=settings.DEFAULT_AGENT_ID or "main", description="智能体 ID")
    round_id: str = Field(default=..., description="轮次对话ID，标识同一用户问题的所有消息(用户消息ID)")
    session_id: str = Field(..., description="SDK会话ID")
    message_id: str = Field(default_factory=lambda: str(uuid.uuid4()), description="消息ID")
    message: Message = Field(..., description="消息内容")
    message_type: Literal["assistant", "user", "system", "result", "stream"] = Field(..., description="消息类型")
    block_type: Optional[str] = Field(default=None, description="消息块类型, text、thinking、tool_result、tool_use")
    parent_id: Optional[str] = Field(default=None, description="父消息ID")
    timestamp: Optional[datetime] = Field(default_factory=datetime.now, description="时间戳")

    model_config = {"from_attributes": True}


class AEvent(BaseModel):
    event_type: str = Field(..., description="事件类型")
    agent_id: str = Field(..., description="客户端会话ID")
    data: Dict[str, Any] = Field(..., description="事件数据")
    session_id: Optional[str] = Field(default=None, description="SDK会话ID")
    timestamp: datetime = Field(default_factory=datetime.now, description="时间戳")


class AStatus(BaseModel):
    """状态响应模型"""
    success: bool = Field(default=True, description="是否成功")


class AError(BaseModel):
    """错误响应模型"""
    error_type: str = Field(..., description="错误类型")
    message: str = Field(..., description="错误消息")
    agent_id: Optional[str] = Field(default=None, description="客户端ID")
    session_id: Optional[str] = Field(default=None, description="会话ID")
    details: Optional[Dict[str, Any]] = Field(default=None, description="错误详情")
    timestamp: datetime = Field(default_factory=datetime.now, description="时间戳")
