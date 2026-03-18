# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：round.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Round ORM 模型。"""

from __future__ import annotations

from datetime import datetime
from typing import TYPE_CHECKING

from sqlalchemy import CheckConstraint, DateTime, ForeignKey, String, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.message import Message
    from agent.infra.database.models.session import Session


class Round(TimestampMixin, Base):
    """按轮次归档的对话记录。"""

    __tablename__ = "rounds"
    __table_args__ = (
        CheckConstraint(
            "status IN ('running', 'success', 'error', 'cancelled')",
            name="ck_rounds_status",
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    session_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("sessions.id", ondelete="CASCADE"),
        nullable=False,
    )
    round_id: Mapped[str] = mapped_column(String(64), unique=True, nullable=False)
    trigger_message_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("messages.id", ondelete="CASCADE"),
        nullable=False,
    )
    status: Mapped[str] = mapped_column(String(32), nullable=False)
    started_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False,
    )
    finished_at: Mapped[datetime | None] = mapped_column(DateTime)

    session: Mapped["Session"] = relationship(back_populates="rounds")
    trigger_message: Mapped["Message"] = relationship(
        back_populates="rounds",
        foreign_keys=[trigger_message_id],
    )
