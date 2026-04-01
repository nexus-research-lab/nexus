# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：activity_event.py
# @Date   ：2026/3/29 11:20
# @Author ：leemysw
# 2026/3/29 11:20   Create
# =====================================================

"""活动事件 ORM 模型。"""

from __future__ import annotations

from sqlalchemy import JSON, DateTime, String, func
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base


class ActivityEventType:
    """活动事件类型常量。"""

    AGENT_CREATED = "agent_created"
    AGENT_UPDATED = "agent_updated"
    ROOM_CREATED = "room_created"
    ROOM_MESSAGE = "room_message"
    ROOM_ROUND_COMPLETED = "room_round_completed"
    ROOM_ROUND_CANCELLED = "room_round_cancelled"
    ROOM_ROUND_FAILED = "room_round_failed"
    DM_MESSAGE = "dm_message"
    PERMISSION_REQUESTED = "permission_requested"
    PERMISSION_RESOLVED = "permission_resolved"
    SKILL_INSTALLED = "skill_installed"
    SKILL_UNINSTALLED = "skill_uninstalled"
    TASK_COMPLETED = "task_completed"
    TASK_FAILED = "task_failed"


class ActivityEvent(Base):
    """活动事件记录表 — 记录系统中的各类活动事件。"""

    __tablename__ = "activity_events"

    # 事件 ID（Snowflake 生成）
    id: Mapped[str] = mapped_column(String(32), primary_key=True)

    # 事件类型（参见 ActivityEventType）
    event_type: Mapped[str] = mapped_column(
        String(50), nullable=False, index=True, comment="事件类型",
    )

    # 执行者类型（user / agent / system）
    actor_type: Mapped[str] = mapped_column(
        String(20), nullable=False, comment="执行者类型",
    )

    # 执行者 ID
    actor_id: Mapped[str | None] = mapped_column(
        String(32), nullable=True, comment="执行者 ID",
    )

    # 目标类型（room / agent / skill 等）
    target_type: Mapped[str | None] = mapped_column(
        String(20), nullable=True, comment="目标类型",
    )

    # 目标 ID
    target_id: Mapped[str | None] = mapped_column(
        String(32), nullable=True, comment="目标 ID",
    )

    # 事件摘要
    summary: Mapped[str | None] = mapped_column(
        String(500), nullable=True, comment="事件摘要",
    )

    # 事件元数据（JSON 存储扩展信息）
    metadata_json: Mapped[dict | None] = mapped_column(
        "metadata", JSON, nullable=True, comment="事件元数据",
    )

    # 创建时间
    created_at: Mapped[str] = mapped_column(
        DateTime, server_default=func.now(), nullable=False, comment="创建时间",
    )
