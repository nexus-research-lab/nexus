# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_cron_job.py
# @Date   ：2026/4/9
# @Author ：Codex
# 2026/4/9   Create
# =====================================================

"""Automation cron job ORM 模型。"""

from __future__ import annotations

from sqlalchemy import Boolean, CheckConstraint, ForeignKey, Index, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class AutomationCronJob(TimestampMixin, Base):
    """定时自动化任务定义。"""

    __tablename__ = "automation_cron_jobs"
    __table_args__ = (
        CheckConstraint(
            "schedule_kind IN ('every', 'cron', 'at')",
            name="ck_automation_cron_jobs_schedule_kind",
        ),
        CheckConstraint(
            "session_target_kind IN ('isolated', 'main', 'bound', 'named')",
            name="ck_automation_cron_jobs_session_target_kind",
        ),
        CheckConstraint(
            "wake_mode IN ('now', 'next-heartbeat')",
            name="ck_automation_cron_jobs_wake_mode",
        ),
        CheckConstraint(
            "delivery_mode IN ('none', 'last', 'explicit')",
            name="ck_automation_cron_jobs_delivery_mode",
        ),
        CheckConstraint(
            "source_kind IN ('user_page', 'agent', 'cli', 'system')",
            name="ck_automation_cron_jobs_source_kind",
        ),
        CheckConstraint(
            "source_context_type IS NULL OR source_context_type IN ('agent', 'room')",
            name="ck_automation_cron_jobs_source_context_type",
        ),
        Index("idx_automation_cron_jobs_agent", "agent_id"),
    )

    job_id: Mapped[str] = mapped_column(String(64), primary_key=True)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
    )
    schedule_kind: Mapped[str] = mapped_column(String(32), nullable=False)
    run_at: Mapped[str | None] = mapped_column(String(32))
    interval_seconds: Mapped[int | None] = mapped_column(Integer)
    cron_expression: Mapped[str | None] = mapped_column(String(255))
    timezone: Mapped[str] = mapped_column(String(64), default="Asia/Shanghai", nullable=False)
    instruction: Mapped[str] = mapped_column(Text, nullable=False)
    session_target_kind: Mapped[str] = mapped_column(String(32), default="isolated", nullable=False)
    bound_session_key: Mapped[str | None] = mapped_column(String(255))
    named_session_key: Mapped[str | None] = mapped_column(String(255))
    wake_mode: Mapped[str] = mapped_column(String(32), default="next-heartbeat", nullable=False)
    delivery_mode: Mapped[str] = mapped_column(String(32), default="none", nullable=False)
    delivery_channel: Mapped[str | None] = mapped_column(String(64))
    delivery_to: Mapped[str | None] = mapped_column(String(255))
    delivery_account_id: Mapped[str | None] = mapped_column(String(64))
    delivery_thread_id: Mapped[str | None] = mapped_column(String(255))
    source_kind: Mapped[str] = mapped_column(String(32), default="system", nullable=False)
    source_creator_agent_id: Mapped[str | None] = mapped_column(String(64))
    source_context_type: Mapped[str | None] = mapped_column(String(32))
    source_context_id: Mapped[str | None] = mapped_column(String(255))
    source_context_label: Mapped[str | None] = mapped_column(String(255))
    source_session_key: Mapped[str | None] = mapped_column(String(255))
    source_session_label: Mapped[str | None] = mapped_column(String(255))
    enabled: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)
