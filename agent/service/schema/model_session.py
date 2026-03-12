# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_session
# @Date   ：2025/11/29 11:12
# @Author ：leemysw
#
# 2025/11/29 11:12   Create
# 2026/2/25          重构：增加 session_key 路由支持
# =====================================================

"""
会话 Pydantic 模型

[INPUT]: 依赖 pydantic
[OUTPUT]: 对外提供 ASession / UpdateTitleRequest
[POS]: schema 模块的会话模型定义，被 session_store/session_repository 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from typing import Optional

from pydantic import BaseModel, Field

from agent.config.config import settings


class ASession(BaseModel):
    """会话模型"""
    session_key: str = Field(..., description="结构化路由键")
    agent_id: str = Field(default=settings.DEFAULT_AGENT_ID or "main", description="智能体 ID")
    session_id: Optional[str] = Field(default=None, description="SDK会话ID")
    channel_type: str = Field(default="websocket", description="通道类型")
    chat_type: str = Field(default="dm", description="会话类型")
    status: str = Field(default="active", description="会话状态")
    created_at: datetime = Field(default_factory=datetime.now, description="创建时间")
    last_activity: datetime = Field(default_factory=datetime.now, description="最后活动时间")
    title: Optional[str] = Field(default=None, description="会话标题")
    message_count: int = Field(0, description="消息数量")
    options: Optional[dict] = Field(default=None, description="会话元数据")

    model_config = {"from_attributes": True}


class UpdateTitleRequest(BaseModel):
    title: str
