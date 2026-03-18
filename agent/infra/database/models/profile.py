# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：profile.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Profile ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import ForeignKey, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.agent import Agent


class Profile(TimestampMixin, Base):
    """Agent 的身份信息。"""

    __tablename__ = "profiles"

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        unique=True,
        nullable=False,
    )
    display_name: Mapped[str] = mapped_column(String(128), nullable=False)
    avatar_url: Mapped[str | None] = mapped_column(String(512))
    headline: Mapped[str] = mapped_column(String(255), default="", nullable=False)
    profile_markdown: Mapped[str] = mapped_column(Text, default="", nullable=False)

    agent: Mapped["Agent"] = relationship(back_populates="profile")
