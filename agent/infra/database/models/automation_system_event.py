# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_system_event.py
# @Date   ：2026/4/9
# @Author ：Codex
# 2026/4/9   Create
# =====================================================

"""Automation system event ORM 模型。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import CheckConstraint, DateTime, Index, JSON, String
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class AutomationSystemEvent(TimestampMixin, Base):
    """自动化系统事件日志。"""

    __tablename__ = "automation_system_events"
    __table_args__ = (
        CheckConstraint(
            "status IN ('new', 'processing', 'processed', 'failed')",
            name="ck_automation_system_events_status",
        ),
        Index("idx_automation_system_events_type", "event_type"),
        Index("idx_automation_system_events_status", "status"),
        Index("idx_automation_system_events_created", "created_at"),
    )

    event_id: Mapped[str] = mapped_column(String(64), primary_key=True)
    event_type: Mapped[str] = mapped_column(String(64), nullable=False)
    source_type: Mapped[str | None] = mapped_column(String(64))
    source_id: Mapped[str | None] = mapped_column(String(64))
    payload: Mapped[dict[str, object]] = mapped_column(JSON, default=dict, nullable=False)
    status: Mapped[str] = mapped_column(String(32), default="new", nullable=False)
    processed_at: Mapped[datetime | None] = mapped_column(DateTime)
