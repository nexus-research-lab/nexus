# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_cron_job_sql_repository.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron job SQL 仓储。"""

from __future__ import annotations

from sqlalchemy import delete, select

from agent.infra.database.models.automation_cron_job import AutomationCronJob
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository


class AutomationCronJobSqlRepository(BaseSqlRepository):
    """定时任务定义的 CRUD 仓储。"""

    _UPDATABLE_FIELDS = {
        "run_at",
        "interval_seconds",
        "cron_expression",
        "timezone",
        "instruction",
        "session_target_kind",
        "bound_session_key",
        "named_session_key",
        "wake_mode",
        "delivery_mode",
        "delivery_channel",
        "delivery_to",
        "delivery_account_id",
        "delivery_thread_id",
        "source_kind",
        "source_creator_agent_id",
        "source_context_type",
        "source_context_id",
        "source_context_label",
        "source_session_key",
        "source_session_label",
        "enabled",
        "created_at",
        "updated_at",
    }

    async def get_job(self, job_id: str) -> AutomationCronJob | None:
        """按 job_id 读取任务定义。"""
        return await self._session.get(AutomationCronJob, job_id)

    async def list_jobs(self, agent_id: str | None = None) -> list[AutomationCronJob]:
        """列出任务定义，可按 agent_id 过滤。"""
        stmt = select(AutomationCronJob)
        if agent_id is not None:
            stmt = stmt.where(AutomationCronJob.agent_id == agent_id)
        stmt = stmt.order_by(AutomationCronJob.created_at.asc(), AutomationCronJob.job_id.asc())
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def upsert_job(
        self,
        *,
        job_id: str,
        name: str,
        agent_id: str,
        schedule_kind: str,
        **fields,
    ) -> AutomationCronJob:
        """创建或更新任务定义。"""
        self._reject_unknown_fields(fields)
        defaults = {
            "run_at": None,
            "interval_seconds": None,
            "cron_expression": None,
            "timezone": "Asia/Shanghai",
            "instruction": "",
            "session_target_kind": "isolated",
            "bound_session_key": None,
            "named_session_key": None,
            "wake_mode": "next-heartbeat",
            "delivery_mode": "none",
            "delivery_channel": None,
            "delivery_to": None,
            "delivery_account_id": None,
            "delivery_thread_id": None,
            "source_kind": "system",
            "source_creator_agent_id": None,
            "source_context_type": None,
            "source_context_id": None,
            "source_context_label": None,
            "source_session_key": None,
            "source_session_label": None,
            "enabled": True,
        }
        payload = {**defaults, **fields}
        entity = await self.get_job(job_id)
        if entity is None:
            entity = AutomationCronJob(
                job_id=job_id,
                name=name,
                agent_id=agent_id,
                schedule_kind=schedule_kind,
                **payload,
            )
            self._session.add(entity)
        else:
            # 只更新这次显式传入的字段，避免覆盖已有配置。
            entity.name = name
            entity.agent_id = agent_id
            entity.schedule_kind = schedule_kind
            for field_name, value in fields.items():
                setattr(entity, field_name, value)
        await self.flush()
        await self.refresh(entity)
        return entity

    async def delete_job(self, job_id: str) -> None:
        """删除任务定义。"""
        stmt = delete(AutomationCronJob).where(AutomationCronJob.job_id == job_id)
        await self._session.execute(stmt)
        await self.flush()

    def _reject_unknown_fields(self, fields: dict[str, object]) -> None:
        """阻止把未知字段挂到 ORM 实体上。"""
        unexpected = set(fields) - self._UPDATABLE_FIELDS
        if unexpected:
            raise ValueError(f"unknown cron job fields: {sorted(unexpected)}")
