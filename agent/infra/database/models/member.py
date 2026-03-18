# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：member.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Member ORM 模型。"""

from __future__ import annotations

from datetime import datetime
from typing import TYPE_CHECKING

from sqlalchemy import CheckConstraint, DateTime, ForeignKey, Index, String, func, text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base

if TYPE_CHECKING:
    from agent.infra.database.models.agent import Agent
    from agent.infra.database.models.room import Room


class Member(Base):
    """房间成员模型。"""

    __tablename__ = "members"
    __table_args__ = (
        CheckConstraint(
            "("
            "member_type = 'agent' AND member_agent_id IS NOT NULL AND member_user_id IS NULL"
            ") OR ("
            "member_type = 'user' AND member_user_id IS NOT NULL AND member_agent_id IS NULL"
            ")",
            name="ck_members_target",
        ),
        Index(
            "uq_members_agent",
            "room_id",
            "member_agent_id",
            unique=True,
            sqlite_where=text("member_type = 'agent' AND member_agent_id IS NOT NULL"),
        ),
        Index(
            "uq_members_user",
            "room_id",
            "member_user_id",
            unique=True,
            sqlite_where=text("member_type = 'user' AND member_user_id IS NOT NULL"),
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    room_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("rooms.id", ondelete="CASCADE"),
        nullable=False,
    )
    member_type: Mapped[str] = mapped_column(String(32), nullable=False)
    member_user_id: Mapped[str | None] = mapped_column(String(64))
    member_agent_id: Mapped[str | None] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
    )
    joined_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False,
    )

    room: Mapped["Room"] = relationship(back_populates="members")
    agent: Mapped["Agent | None"] = relationship(
        back_populates="memberships",
        foreign_keys=[member_agent_id],
    )
