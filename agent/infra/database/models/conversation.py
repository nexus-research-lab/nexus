# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：conversation.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Conversation ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import CheckConstraint, ForeignKey, Index, String
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.message import Message
    from agent.infra.database.models.room import Room
    from agent.infra.database.models.session import Session


class Conversation(TimestampMixin, Base):
    """房间里的具体对话。"""

    __tablename__ = "conversations"
    __table_args__ = (
        CheckConstraint(
            "conversation_type IN ('dm', 'room_main', 'topic')",
            name="ck_conversations_type",
        ),
        Index("idx_conversations_room", "room_id", "created_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    room_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("rooms.id", ondelete="CASCADE"),
        nullable=False,
    )
    conversation_type: Mapped[str] = mapped_column(String(32), nullable=False)
    title: Mapped[str | None] = mapped_column(String(255))

    room: Mapped["Room"] = relationship(back_populates="conversations")
    sessions: Mapped[list["Session"]] = relationship(
        back_populates="conversation",
        cascade="all, delete-orphan",
    )
    messages: Mapped[list["Message"]] = relationship(
        back_populates="conversation",
        cascade="all, delete-orphan",
    )
