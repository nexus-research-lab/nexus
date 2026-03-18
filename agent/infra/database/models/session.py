# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Session ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from datetime import datetime

from sqlalchemy import (
    CheckConstraint,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    String,
    func,
    text,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.agent import Agent
    from agent.infra.database.models.conversation import Conversation
    from agent.infra.database.models.message import Message
    from agent.infra.database.models.round import Round
    from agent.infra.database.models.runtime import Runtime


class Session(TimestampMixin, Base):
    """Agent 在对话中的运行时会话。"""

    __tablename__ = "sessions"
    __table_args__ = (
        CheckConstraint(
            "status IN ('active', 'idle', 'interrupted', 'closed')",
            name="ck_sessions_status",
        ),
        Index(
            "uq_sessions_branch",
            "conversation_id",
            "agent_id",
            "branch_key",
            unique=True,
        ),
        Index(
            "uq_sessions_primary",
            "conversation_id",
            "agent_id",
            unique=True,
            sqlite_where=text("is_primary = 1"),
        ),
        Index("idx_sessions_conversation", "conversation_id", "last_activity_at"),
        Index("idx_sessions_agent", "agent_id", "last_activity_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    conversation_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("conversations.id", ondelete="CASCADE"),
        nullable=False,
    )
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
    )
    runtime_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("runtimes.id", ondelete="CASCADE"),
        nullable=False,
    )
    version_no: Mapped[int] = mapped_column(Integer, default=1, nullable=False)
    branch_key: Mapped[str] = mapped_column(String(128), default="main", nullable=False)
    is_primary: Mapped[bool] = mapped_column(default=True, nullable=False)
    sdk_session_id: Mapped[str | None] = mapped_column(String(128))
    status: Mapped[str] = mapped_column(String(32), default="active", nullable=False)
    # 最近活动时间用于会话排序和续接。
    last_activity_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False,
    )

    agent: Mapped["Agent"] = relationship(back_populates="sessions")
    runtime: Mapped["Runtime"] = relationship(back_populates="sessions")
    conversation: Mapped["Conversation"] = relationship(back_populates="sessions")
    messages: Mapped[list["Message"]] = relationship(back_populates="session")
    rounds: Mapped[list["Round"]] = relationship(back_populates="session")
