# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_heartbeat_state.py
# @Date   ：2026/4/9
# @Author ：Codex
# 2026/4/9   Create
# =====================================================

"""Automation heartbeat state ORM 模型。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import Boolean, CheckConstraint, DateTime, ForeignKey, Integer, String, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class AutomationHeartbeatState(TimestampMixin, Base):
    """Agent 心跳监控状态。"""

    __tablename__ = "automation_heartbeat_states"
    __table_args__ = (
        CheckConstraint(
            "target_mode IN ('none', 'last', 'explicit')",
            name="ck_automation_heartbeat_states_target_mode",
        ),
        UniqueConstraint("agent_id", name="uq_automation_heartbeat_states_agent"),
    )

    state_id: Mapped[str] = mapped_column(String(64), primary_key=True)
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
    )
    enabled: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
    every_seconds: Mapped[int] = mapped_column(Integer, default=1800, nullable=False)
    target_mode: Mapped[str] = mapped_column(String(32), default="none", nullable=False)
    ack_max_chars: Mapped[int] = mapped_column(Integer, default=300, nullable=False)
    last_heartbeat_at: Mapped[datetime | None] = mapped_column(DateTime)
    last_ack_at: Mapped[datetime | None] = mapped_column(DateTime)
