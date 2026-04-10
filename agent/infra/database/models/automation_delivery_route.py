# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_delivery_route.py
# @Date   ：2026/4/9
# @Author ：Codex
# 2026/4/9   Create
# =====================================================

"""Automation delivery route ORM 模型。"""

from __future__ import annotations

from sqlalchemy import Boolean, CheckConstraint, ForeignKey, Index, String
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class AutomationDeliveryRoute(TimestampMixin, Base):
    """消息投递路由配置。"""

    __tablename__ = "automation_delivery_routes"
    __table_args__ = (
        CheckConstraint(
            "mode IN ('none', 'last', 'explicit')",
            name="ck_automation_delivery_routes_mode",
        ),
        Index("idx_automation_delivery_routes_agent", "agent_id"),
    )

    route_id: Mapped[str] = mapped_column(String(64), primary_key=True)
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
    )
    mode: Mapped[str] = mapped_column(String(32), default="none", nullable=False)
    channel: Mapped[str | None] = mapped_column(String(64))
    to: Mapped[str | None] = mapped_column(String(255))
    account_id: Mapped[str | None] = mapped_column(String(64))
    thread_id: Mapped[str | None] = mapped_column(String(255))
    enabled: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)
