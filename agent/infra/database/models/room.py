# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Room ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import CheckConstraint, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.conversation import Conversation
    from agent.infra.database.models.member import Member


class Room(TimestampMixin, Base):
    """房间模型。"""

    __tablename__ = "rooms"
    __table_args__ = (
        CheckConstraint(
            "room_type IN ('dm', 'room')",
            name="ck_rooms_type",
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    room_type: Mapped[str] = mapped_column(String(32), nullable=False, index=True)
    name: Mapped[str | None] = mapped_column(String(128))
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    avatar: Mapped[str | None] = mapped_column(String(64), nullable=True)

    members: Mapped[list["Member"]] = relationship(
        back_populates="room",
        cascade="all, delete-orphan",
    )
    conversations: Mapped[list["Conversation"]] = relationship(
        back_populates="room",
        cascade="all, delete-orphan",
    )
