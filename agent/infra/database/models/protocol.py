# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：protocol.py
# @Date   ：2026/3/25 21:10
# @Author ：OpenAI
# =====================================================

"""Protocol Room ORM 模型。"""

from __future__ import annotations

from datetime import datetime
from typing import TYPE_CHECKING, Any

from sqlalchemy import JSON, CheckConstraint, DateTime, ForeignKey, Index, Integer, String, Text, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.room import Room


class ProtocolDefinition(TimestampMixin, Base):
    """协议定义。"""

    __tablename__ = "protocol_definitions"
    __table_args__ = (
        Index("uq_protocol_definitions_slug_version", "slug", "version", unique=True),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    slug: Mapped[str] = mapped_column(String(128), nullable=False, index=True)
    name: Mapped[str] = mapped_column(String(128), nullable=False)
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    version: Mapped[int] = mapped_column(Integer, default=1, nullable=False)
    coordinator_mode: Mapped[str] = mapped_column(String(64), default="main_agent", nullable=False)
    phases: Mapped[list[str]] = mapped_column(JSON, default=list, nullable=False)
    channel_policy: Mapped[list[dict[str, Any]]] = mapped_column(JSON, default=list, nullable=False)
    turn_policy: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    action_schemas: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    visibility_resolver: Mapped[str] = mapped_column(String(64), default="default", nullable=False)
    completion_rule: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)

    runs: Mapped[list["ProtocolRun"]] = relationship(back_populates="definition")


class ProtocolRun(TimestampMixin, Base):
    """协议运行。"""

    __tablename__ = "protocol_runs"
    __table_args__ = (
        CheckConstraint(
            "status IN ('running', 'paused', 'completed', 'terminated')",
            name="ck_protocol_runs_status",
        ),
        Index("idx_protocol_runs_room", "room_id", "created_at"),
        Index("idx_protocol_runs_definition", "protocol_definition_id", "created_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    room_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("rooms.id", ondelete="CASCADE"),
        nullable=False,
    )
    protocol_definition_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("protocol_definitions.id", ondelete="CASCADE"),
        nullable=False,
    )
    title: Mapped[str | None] = mapped_column(String(255))
    status: Mapped[str] = mapped_column(String(32), default="running", nullable=False)
    current_phase: Mapped[str] = mapped_column(String(64), default="setup", nullable=False)
    phase_index: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    current_turn_key: Mapped[str | None] = mapped_column(String(128))
    coordinator_agent_id: Mapped[str] = mapped_column(String(64), nullable=False)
    run_config: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    state: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    completed_at: Mapped[datetime | None] = mapped_column(DateTime)

    room: Mapped["Room"] = relationship()
    definition: Mapped["ProtocolDefinition"] = relationship(back_populates="runs")
    channels: Mapped[list["Channel"]] = relationship(
        back_populates="protocol_run",
        cascade="all, delete-orphan",
    )
    action_requests: Mapped[list["ActionRequest"]] = relationship(
        back_populates="protocol_run",
        cascade="all, delete-orphan",
    )
    action_submissions: Mapped[list["ActionSubmission"]] = relationship(
        back_populates="protocol_run",
        cascade="all, delete-orphan",
    )
    snapshots: Mapped[list["RunStateSnapshot"]] = relationship(
        back_populates="protocol_run",
        cascade="all, delete-orphan",
    )


class Channel(TimestampMixin, Base):
    """协议频道。"""

    __tablename__ = "channels"
    __table_args__ = (
        CheckConstraint(
            "channel_type IN ('public', 'scoped', 'direct', 'system')",
            name="ck_channels_type",
        ),
        Index("uq_channels_slug", "protocol_run_id", "slug", unique=True),
        Index("idx_channels_run_position", "protocol_run_id", "position"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    room_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("rooms.id", ondelete="CASCADE"),
        nullable=False,
    )
    protocol_run_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("protocol_runs.id", ondelete="CASCADE"),
        nullable=False,
    )
    slug: Mapped[str] = mapped_column(String(128), nullable=False)
    name: Mapped[str] = mapped_column(String(128), nullable=False)
    channel_type: Mapped[str] = mapped_column(String(32), nullable=False)
    visibility: Mapped[str] = mapped_column(String(32), default="public", nullable=False)
    topic: Mapped[str] = mapped_column(Text, default="", nullable=False)
    position: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    metadata: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)

    protocol_run: Mapped["ProtocolRun"] = relationship(back_populates="channels")
    members: Mapped[list["ChannelMember"]] = relationship(
        back_populates="channel",
        cascade="all, delete-orphan",
    )


class ChannelMember(TimestampMixin, Base):
    """频道成员。"""

    __tablename__ = "channel_members"
    __table_args__ = (
        CheckConstraint(
            "("
            "member_type = 'agent' AND member_agent_id IS NOT NULL AND member_user_id IS NULL"
            ") OR ("
            "member_type = 'user' AND member_user_id IS NOT NULL AND member_agent_id IS NULL"
            ")",
            name="ck_channel_members_target",
        ),
        Index(
            "uq_channel_members_agent",
            "channel_id",
            "member_agent_id",
            unique=True,
        ),
        Index(
            "uq_channel_members_user",
            "channel_id",
            "member_user_id",
            unique=True,
        ),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    channel_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("channels.id", ondelete="CASCADE"),
        nullable=False,
    )
    member_type: Mapped[str] = mapped_column(String(32), nullable=False)
    member_user_id: Mapped[str | None] = mapped_column(String(64))
    member_agent_id: Mapped[str | None] = mapped_column(String(64))
    role_label: Mapped[str | None] = mapped_column(String(64))

    channel: Mapped["Channel"] = relationship(back_populates="members")


class ActionRequest(TimestampMixin, Base):
    """动作请求。"""

    __tablename__ = "action_requests"
    __table_args__ = (
        CheckConstraint(
            "status IN ('pending', 'resolved', 'cancelled')",
            name="ck_action_requests_status",
        ),
        Index("idx_action_requests_run_phase", "protocol_run_id", "phase_name", "created_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    protocol_run_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("protocol_runs.id", ondelete="CASCADE"),
        nullable=False,
    )
    channel_id: Mapped[str | None] = mapped_column(
        String(64),
        ForeignKey("channels.id", ondelete="SET NULL"),
    )
    phase_name: Mapped[str] = mapped_column(String(64), nullable=False)
    turn_key: Mapped[str | None] = mapped_column(String(128))
    action_type: Mapped[str] = mapped_column(String(64), nullable=False)
    status: Mapped[str] = mapped_column(String(32), default="pending", nullable=False)
    requested_by_agent_id: Mapped[str | None] = mapped_column(String(64))
    allowed_actor_agent_ids: Mapped[list[str]] = mapped_column(JSON, default=list, nullable=False)
    audience_agent_ids: Mapped[list[str]] = mapped_column(JSON, default=list, nullable=False)
    input_schema: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    target_scope: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    prompt_text: Mapped[str | None] = mapped_column(Text)
    metadata: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    resolved_at: Mapped[datetime | None] = mapped_column(DateTime)

    protocol_run: Mapped["ProtocolRun"] = relationship(back_populates="action_requests")
    submissions: Mapped[list["ActionSubmission"]] = relationship(
        back_populates="request",
        cascade="all, delete-orphan",
    )


class ActionSubmission(TimestampMixin, Base):
    """动作提交。"""

    __tablename__ = "action_submissions"
    __table_args__ = (
        CheckConstraint(
            "actor_type IN ('agent', 'user', 'system')",
            name="ck_action_submissions_actor_type",
        ),
        CheckConstraint(
            "status IN ('submitted', 'overridden', 'rejected', 'accepted')",
            name="ck_action_submissions_status",
        ),
        Index("idx_action_submissions_request", "request_id", "created_at"),
        Index("idx_action_submissions_run", "protocol_run_id", "created_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    request_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("action_requests.id", ondelete="CASCADE"),
        nullable=False,
    )
    protocol_run_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("protocol_runs.id", ondelete="CASCADE"),
        nullable=False,
    )
    channel_id: Mapped[str | None] = mapped_column(
        String(64),
        ForeignKey("channels.id", ondelete="SET NULL"),
    )
    actor_type: Mapped[str] = mapped_column(String(32), default="agent", nullable=False)
    actor_agent_id: Mapped[str | None] = mapped_column(String(64))
    actor_user_id: Mapped[str | None] = mapped_column(String(64))
    action_type: Mapped[str] = mapped_column(String(64), nullable=False)
    payload: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    status: Mapped[str] = mapped_column(String(32), default="submitted", nullable=False)
    metadata: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)

    request: Mapped["ActionRequest"] = relationship(back_populates="submissions")
    protocol_run: Mapped["ProtocolRun"] = relationship(back_populates="action_submissions")


class RunStateSnapshot(TimestampMixin, Base):
    """运行态快照/事件流。"""

    __tablename__ = "run_state_snapshots"
    __table_args__ = (
        CheckConstraint(
            "visibility IN ('public', 'scoped', 'direct', 'system')",
            name="ck_run_state_snapshots_visibility",
        ),
        Index("uq_run_state_snapshots_seq", "protocol_run_id", "event_seq", unique=True),
        Index("idx_run_state_snapshots_run", "protocol_run_id", "created_at"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    protocol_run_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("protocol_runs.id", ondelete="CASCADE"),
        nullable=False,
    )
    event_seq: Mapped[int] = mapped_column(Integer, nullable=False)
    phase_name: Mapped[str] = mapped_column(String(64), nullable=False)
    event_type: Mapped[str] = mapped_column(String(64), nullable=False)
    channel_id: Mapped[str | None] = mapped_column(
        String(64),
        ForeignKey("channels.id", ondelete="SET NULL"),
    )
    actor_agent_id: Mapped[str | None] = mapped_column(String(64))
    visibility: Mapped[str] = mapped_column(String(32), default="public", nullable=False)
    audience_agent_ids: Mapped[list[str]] = mapped_column(JSON, default=list, nullable=False)
    headline: Mapped[str | None] = mapped_column(String(255))
    body: Mapped[str | None] = mapped_column(Text)
    state: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    metadata: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)

    protocol_run: Mapped["ProtocolRun"] = relationship(back_populates="snapshots")
