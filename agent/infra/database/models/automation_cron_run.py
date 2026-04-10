# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_cron_run.py
# @Date   ：2026/4/9
# @Author ：Codex
# 2026/4/9   Create
# =====================================================

"""Automation cron run ORM 模型。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import CheckConstraint, DateTime, ForeignKey, Index, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class AutomationCronRun(TimestampMixin, Base):
    """定时任务执行记录。"""

    __tablename__ = "automation_cron_runs"
    __table_args__ = (
        CheckConstraint(
            "status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')",
            name="ck_automation_cron_runs_status",
        ),
        Index("idx_automation_cron_runs_job", "job_id"),
        Index("idx_automation_cron_runs_status", "status"),
    )

    run_id: Mapped[str] = mapped_column(String(64), primary_key=True)
    job_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("automation_cron_jobs.job_id", ondelete="CASCADE"),
        nullable=False,
    )
    status: Mapped[str] = mapped_column(String(32), default="pending", nullable=False)
    scheduled_for: Mapped[datetime | None] = mapped_column(DateTime)
    started_at: Mapped[datetime | None] = mapped_column(DateTime)
    finished_at: Mapped[datetime | None] = mapped_column(DateTime)
    attempts: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    error_message: Mapped[str | None] = mapped_column(Text)
