# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_normalizer.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron 数据归一化工具。"""

from __future__ import annotations

from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationCronSource,
    AutomationDeliveryTarget,
    AutomationSessionTarget,
)


def normalize_job_create(job_id: str, payload: AutomationCronJobCreate) -> dict[str, object]:
    """把创建模型拍平成 store 字段。"""
    return {
        "job_id": job_id,
        "name": payload.name,
        "agent_id": payload.agent_id,
        **schedule_to_fields(payload.schedule),
        **session_target_to_fields(payload.session_target),
        **delivery_to_fields(payload.delivery),
        **source_to_fields(payload.source),
        "instruction": payload.instruction,
        "enabled": payload.enabled,
    }


def schedule_to_fields(schedule: AutomationCronSchedule) -> dict[str, object]:
    return {
        "schedule_kind": schedule.kind,
        "run_at": schedule.run_at,
        "interval_seconds": schedule.interval_seconds,
        "cron_expression": schedule.cron_expression,
        "timezone": schedule.timezone,
    }


def session_target_to_fields(target: AutomationSessionTarget) -> dict[str, object]:
    return {
        "session_target_kind": target.kind,
        "bound_session_key": target.bound_session_key,
        "named_session_key": target.named_session_key,
        "wake_mode": target.wake_mode,
    }


def delivery_to_fields(target: AutomationDeliveryTarget) -> dict[str, object]:
    return {
        "delivery_mode": target.mode,
        "delivery_channel": target.channel,
        "delivery_to": target.to,
        "delivery_account_id": target.account_id,
        "delivery_thread_id": target.thread_id,
    }


def source_to_fields(source: AutomationCronSource) -> dict[str, object]:
    return {
        "source_kind": source.kind,
        "source_creator_agent_id": source.creator_agent_id,
        "source_context_type": source.context_type,
        "source_context_id": source.context_id,
        "source_context_label": source.context_label,
        "source_session_key": source.session_key,
        "source_session_label": source.session_label,
    }


def row_to_schedule(row) -> AutomationCronSchedule:
    return AutomationCronSchedule(
        kind=str(row.schedule_kind),
        run_at=getattr(row, "run_at", None),
        interval_seconds=getattr(row, "interval_seconds", None),
        cron_expression=getattr(row, "cron_expression", None),
        timezone=str(getattr(row, "timezone", "Asia/Shanghai") or "Asia/Shanghai"),
    )


def row_to_session_target(row) -> AutomationSessionTarget:
    return AutomationSessionTarget(
        kind=str(getattr(row, "session_target_kind", "isolated")),
        bound_session_key=getattr(row, "bound_session_key", None),
        named_session_key=getattr(row, "named_session_key", None),
        wake_mode=str(getattr(row, "wake_mode", "next-heartbeat")),
    )


def row_to_delivery(row) -> AutomationDeliveryTarget:
    return AutomationDeliveryTarget(
        mode=str(getattr(row, "delivery_mode", "none")),
        channel=getattr(row, "delivery_channel", None),
        to=getattr(row, "delivery_to", None),
        account_id=getattr(row, "delivery_account_id", None),
        thread_id=getattr(row, "delivery_thread_id", None),
    )


def row_to_source(row) -> AutomationCronSource:
    return AutomationCronSource(
        kind=str(getattr(row, "source_kind", "system") or "system"),
        creator_agent_id=getattr(row, "source_creator_agent_id", None),
        context_type=getattr(row, "source_context_type", None),
        context_id=getattr(row, "source_context_id", None),
        context_label=getattr(row, "source_context_label", None),
        session_key=getattr(row, "source_session_key", None),
        session_label=getattr(row, "source_session_label", None),
    )


def row_to_job_dict(row, *, runtime: dict[str, object] | None = None) -> dict[str, object]:
    runtime_dict = dict(runtime or {})
    return {
        "job_id": str(row.job_id),
        "name": str(row.name),
        "agent_id": str(row.agent_id),
        "schedule": row_to_schedule(row),
        "instruction": str(row.instruction),
        "session_target": row_to_session_target(row),
        "delivery": row_to_delivery(row),
        "source": row_to_source(row),
        "enabled": bool(row.enabled),
        "next_run_at": runtime_dict.get("next_run_at"),
        "running": bool(runtime_dict.get("running", False)),
        "last_run_at": runtime_dict.get("last_run_at"),
    }


def row_to_run_dict(row) -> dict[str, object]:
    return {
        "run_id": str(row.run_id),
        "job_id": str(row.job_id),
        "status": str(row.status),
        "scheduled_for": getattr(row, "scheduled_for", None),
        "started_at": getattr(row, "started_at", None),
        "finished_at": getattr(row, "finished_at", None),
        "attempts": int(getattr(row, "attempts", 0)),
        "error_message": getattr(row, "error_message", None),
    }


def resolve_session_key(job, *, run_id: str | None = None) -> str:
    target = str(getattr(job, "session_target_kind", "isolated"))
    if target == "main":
        return _build_session_key(
            agent_id=getattr(job, "agent_id", None),
            ref="main",
        )
    if target == "bound":
        return str(getattr(job, "bound_session_key"))
    if target == "named":
        return _build_session_key(
            agent_id=getattr(job, "agent_id", None),
            ref=str(getattr(job, "named_session_key")),
        )

    # 中文注释：isolated 目标必须给每次执行单独的 session_key，
    # 这样编排器会创建全新的 automation 会话，不会污染之前的上下文。
    if not run_id:
        raise ValueError("run_id is required for isolated session targets")
    return _build_session_key(
        agent_id=getattr(job, "agent_id", None),
        ref=f"cron:{job.job_id}:{run_id}",
    )


def _build_session_key(*, agent_id: str | None, ref: str) -> str:
    resolved_agent_id = str(agent_id or "").strip() or "default"
    return f"agent:{resolved_agent_id}:automation:dm:{ref.strip()}"
