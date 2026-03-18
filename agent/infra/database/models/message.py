# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：message.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Message ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import CheckConstraint, ForeignKey, Index, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.agent import Agent
    from agent.infra.database.models.conversation import Conversation
    from agent.infra.database.models.round import Round
    from agent.infra.database.models.session import Session


class Message(TimestampMixin, Base):
    """统一消息记录。"""

    __tablename__ = "messages"
    __table_args__ = (
        CheckConstraint(
            "sender_type IN ('user', 'agent', 'system', 'tool')",
            name="ck_messages_sender_type",
        ),
        CheckConstraint(
            "kind IN ('text', 'tool_call', 'tool_result', 'event', 'error')",
            name="ck_messages_kind",
        ),
        Index("idx_messages_conversation", "conversation_id", "created_at"),
        Index("idx_messages_session", "session_id", "created_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    conversation_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("conversations.id", ondelete="CASCADE"),
        nullable=False,
    )
    session_id: Mapped[str | None] = mapped_column(
        String(64),
        ForeignKey("sessions.id", ondelete="SET NULL"),
    )
    sender_type: Mapped[str] = mapped_column(String(32), nullable=False)
    sender_user_id: Mapped[str | None] = mapped_column(String(64))
    sender_agent_id: Mapped[str | None] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="SET NULL"),
    )
    kind: Mapped[str] = mapped_column(String(32), nullable=False)
    content_preview: Mapped[str | None] = mapped_column(Text)
    jsonl_path: Mapped[str] = mapped_column(String(512), nullable=False)
    jsonl_offset: Mapped[int | None] = mapped_column(Integer)
    round_id: Mapped[str | None] = mapped_column(String(64))

    conversation: Mapped["Conversation"] = relationship(back_populates="messages")
    session: Mapped["Session | None"] = relationship(back_populates="messages")
    sender_agent: Mapped["Agent | None"] = relationship(foreign_keys=[sender_agent_id])
    rounds: Mapped[list["Round"]] = relationship(back_populates="trigger_message")
