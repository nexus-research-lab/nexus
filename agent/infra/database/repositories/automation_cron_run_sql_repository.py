# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_cron_run_sql_repository.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron run SQL 仓储。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import select

from agent.infra.database.models.automation_cron_run import AutomationCronRun
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.utils.utils import random_uuid

_MISSING = object()


class AutomationCronRunSqlRepository(BaseSqlRepository):
    """定时任务执行记录的 CRUD 仓储。"""

    async def create_run(
        self,
        *,
        job_id: str,
        run_id: str | None = None,
        **fields,
    ) -> AutomationCronRun:
        """创建执行记录。"""
        defaults = {
            "status": "pending",
            "scheduled_for": None,
            "started_at": None,
            "finished_at": None,
            "attempts": 0,
            "error_message": None,
        }
        payload = {**defaults, **fields}
        entity = AutomationCronRun(run_id=run_id or random_uuid(), job_id=job_id, **payload)
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return entity

    async def get_run(self, run_id: str) -> AutomationCronRun | None:
        """按 run_id 读取执行记录。"""
        return await self._session.get(AutomationCronRun, run_id)

    async def list_runs_by_job(self, job_id: str) -> list[AutomationCronRun]:
        """列出某个任务下的全部执行记录。"""
        stmt = (
            select(AutomationCronRun)
            .where(AutomationCronRun.job_id == job_id)
            .order_by(AutomationCronRun.created_at.asc(), AutomationCronRun.run_id.asc())
        )
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def update_run_status(
        self,
        *,
        run_id: str,
        status: str,
        started_at: datetime | None | object = _MISSING,
        finished_at: datetime | None | object = _MISSING,
        attempts: int | None = None,
        error_message: str | None | object = _MISSING,
    ) -> AutomationCronRun | None:
        """更新执行状态和结果字段。"""
        entity = await self.get_run(run_id)
        if entity is None:
            return None
        entity.status = status
        if started_at is not _MISSING:
            entity.started_at = started_at
        if finished_at is not _MISSING:
            entity.finished_at = finished_at
        if attempts is not None:
            entity.attempts = attempts
        if error_message is not _MISSING:
            entity.error_message = error_message
        await self.flush()
        await self.refresh(entity)
        return entity
