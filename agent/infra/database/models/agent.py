# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Agent ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import String, Text, CheckConstraint
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.contact import Contact
    from agent.infra.database.models.member import Member
    from agent.infra.database.models.profile import Profile
    from agent.infra.database.models.runtime import Runtime
    from agent.infra.database.models.session import Session


class Agent(TimestampMixin, Base):
    """系统中的真实 Agent 实体。"""

    __tablename__ = "agents"
    __table_args__ = (
        CheckConstraint(
            "status IN ('active', 'archived', 'disabled')",
            name="ck_agents_status",
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    slug: Mapped[str] = mapped_column(String(128), unique=True, nullable=False, index=True)
    name: Mapped[str] = mapped_column(String(128), nullable=False)
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    definition: Mapped[str] = mapped_column(Text, default="", nullable=False)
    status: Mapped[str] = mapped_column(String(32), default="active", nullable=False)
    workspace_path: Mapped[str] = mapped_column(String(512), nullable=False)

    profile: Mapped["Profile"] = relationship(
        back_populates="agent",
        cascade="all, delete-orphan",
        uselist=False,
    )
    runtime: Mapped["Runtime"] = relationship(
        back_populates="agent",
        cascade="all, delete-orphan",
        uselist=False,
    )
    contacts: Mapped[list["Contact"]] = relationship(
        back_populates="owner_agent",
        cascade="all, delete-orphan",
        foreign_keys="Contact.owner_agent_id",
    )
    memberships: Mapped[list["Member"]] = relationship(
        back_populates="agent",
        cascade="all, delete-orphan",
    )
    sessions: Mapped[list["Session"]] = relationship(
        back_populates="agent",
        cascade="all, delete-orphan",
    )
