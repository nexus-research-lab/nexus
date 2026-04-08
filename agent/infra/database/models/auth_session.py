# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：auth_session.py
# @Date   ：2026/04/08 12:02
# @Author ：leemysw
# 2026/04/08 12:02   Create
# =====================================================

"""浏览器登录会话 ORM 模型。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, Index, String
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class AuthSession(TimestampMixin, Base):
    """服务端登录会话。"""

    __tablename__ = "auth_sessions"
    __table_args__ = (
        Index("uq_auth_sessions_token_hash", "session_token_hash", unique=True),
        Index("idx_auth_sessions_expires_at", "expires_at"),
        Index("idx_auth_sessions_username", "username"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    session_token_hash: Mapped[str] = mapped_column(String(64), nullable=False)
    username: Mapped[str] = mapped_column(String(128), nullable=False)
    expires_at: Mapped[datetime] = mapped_column(DateTime, nullable=False)
