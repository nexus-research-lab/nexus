# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_auth.py
# @Date   ：2026/04/08 12:02
# @Author ：leemysw
# 2026/04/08 12:02   Create
# =====================================================

"""认证相关持久化模型。"""

from __future__ import annotations

from datetime import datetime

from pydantic import Field

from agent.schema.model_chat_persistence import PersistenceModel


class AuthSessionRecord(PersistenceModel):
    """浏览器登录会话记录。"""

    id: str = Field(..., description="会话记录 ID")
    session_token_hash: str = Field(..., description="会话令牌摘要")
    username: str = Field(..., description="登录用户名")
    expires_at: datetime = Field(..., description="会话过期时间")
    created_at: datetime | None = Field(default=None, description="创建时间")
    updated_at: datetime | None = Field(default=None, description="更新时间")
