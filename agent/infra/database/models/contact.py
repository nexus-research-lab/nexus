# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：contact.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Contact ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import ForeignKey, Index, String
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.agent import Agent


class Contact(TimestampMixin, Base):
    """Agent 的联系人列表。"""

    __tablename__ = "contacts"
    __table_args__ = (
        Index(
            "uq_contacts_agent",
            "owner_agent_id",
            "contact_agent_id",
            unique=True,
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    owner_agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
    )
    contact_agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
    )
    alias: Mapped[str | None] = mapped_column(String(128))

    owner_agent: Mapped["Agent"] = relationship(
        back_populates="contacts",
        foreign_keys=[owner_agent_id],
    )
    contact_agent: Mapped["Agent"] = relationship(foreign_keys=[contact_agent_id])
