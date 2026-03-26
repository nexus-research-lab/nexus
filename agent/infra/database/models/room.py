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

from typing import TYPE_CHECKING, Any

from sqlalchemy import JSON, CheckConstraint, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin
from agent.service.agent.main_agent_profile import MainAgentProfile

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
        CheckConstraint(
            "mode IN ('open', 'protocol')",
            name="ck_rooms_mode",
        ),
        CheckConstraint(
            "runtime_status IN ('created', 'running', 'paused', 'finished', 'error')",
            name="ck_rooms_runtime_status",
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    room_type: Mapped[str] = mapped_column(String(32), nullable=False, index=True)
    name: Mapped[str | None] = mapped_column(String(128))
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    mode: Mapped[str] = mapped_column(String(32), default="open", nullable=False)
    runtime_status: Mapped[str] = mapped_column(String(32), default="created", nullable=False)
    active_run_id: Mapped[str | None] = mapped_column(String(64))
    orchestrator_agent_id: Mapped[str] = mapped_column(
        String(64),
        default=MainAgentProfile.AGENT_ID,
        nullable=False,
    )
    ruleset_slug: Mapped[str | None] = mapped_column(String(128))
    goal: Mapped[str] = mapped_column(Text, default="", nullable=False)
    runtime_state: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    capabilities_json: Mapped[dict[str, Any]] = mapped_column("capabilities", JSON, default=dict, nullable=False)

    members: Mapped[list["Member"]] = relationship(
        back_populates="room",
        cascade="all, delete-orphan",
    )
    conversations: Mapped[list["Conversation"]] = relationship(
        back_populates="room",
        cascade="all, delete-orphan",
    )
