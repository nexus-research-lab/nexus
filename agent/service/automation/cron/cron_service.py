# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_service.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron 服务门面。"""

from __future__ import annotations

from datetime import datetime, timezone

from agent.infra.schemas.model_cython import AModel
from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationCronSource,
    AutomationDeliveryTarget,
    AutomationSessionTarget,
)
from agent.service.automation.cron.cron_normalizer import (
    delivery_to_fields,
    normalize_job_create,
    row_to_job_dict,
    row_to_run_dict,
    schedule_to_fields,
    source_to_fields,
    session_target_to_fields,
)
from agent.service.automation.cron.cron_runner import CronExecutionResult, CronRunner
from agent.service.automation.cron.cron_schedule import compute_next_run_datetime
from agent.utils.utils import random_uuid


class CronJobView(AModel):
    """对外暴露的 cron job 视图。"""

    job_id: str
    name: str
    agent_id: str
    schedule: AutomationCronSchedule
    instruction: str
    session_target: AutomationSessionTarget
    delivery: AutomationDeliveryTarget
    source: AutomationCronSource
    enabled: bool
    next_run_at: datetime | None = None
    running: bool = False
    last_run_at: datetime | None = None


class CronRunView(AModel):
    """run ledger 对外视图。"""

    run_id: str
    job_id: str
    status: str
    scheduled_for: datetime | None = None
    started_at: datetime | None = None
    finished_at: datetime | None = None
    attempts: int = 0
    error_message: str | None = None


class CronService:
    """对外提供 cron job 的 CRUD、运行与 ledger 查询。"""

    def __init__(
        self,
        *,
        store=None,
        runner: CronRunner | None = None,
        timer=None,
        id_factory=None,
        now_fn=None,
    ) -> None:
        if store is None:
            from agent.service.automation.cron.cron_store_service import CronStoreService

            store = CronStoreService()
        self._store = store
        self._id_factory = id_factory or random_uuid
        self._now_fn = now_fn or (lambda: datetime.now(timezone.utc))
        self._runner = runner or CronRunner(store=self._store, now_fn=self._now_fn)
        if timer is None:
            from agent.service.automation.cron.cron_timer import CronTimer

            timer = CronTimer(dispatcher=self._run_due_job, now_fn=self._now_fn)
        self._timer = timer

    async def start(self) -> None:
        rows = await self._store.list_jobs()
        for row in rows:
            await self._sync_job(row)
        await self._timer.start()

    async def stop(self) -> None:
        await self._timer.stop()

    async def list_jobs(self, *, agent_id: str | None = None) -> list[CronJobView]:
        rows = await self._store.list_jobs(agent_id=agent_id)
        return [self._build_job_view(row) for row in rows]

    async def create_job(self, payload: AutomationCronJobCreate) -> CronJobView:
        row = await self._store.upsert_job(**normalize_job_create(self._id_factory(), payload))
        await self._sync_job(row)
        return self._build_job_view(row)

    async def update_job(
        self,
        job_id: str,
        *,
        name: str | None = None,
        agent_id: str | None = None,
        schedule: AutomationCronSchedule | None = None,
        instruction: str | None = None,
        session_target: AutomationSessionTarget | None = None,
        delivery: AutomationDeliveryTarget | None = None,
        source: AutomationCronSource | None = None,
        enabled: bool | None = None,
    ) -> CronJobView:
        row = await self._require_job(job_id)
        fields: dict[str, object] = {
            "job_id": row.job_id,
            "name": row.name if name is None else name,
            "agent_id": row.agent_id if agent_id is None else agent_id,
            "schedule_kind": row.schedule_kind,
        }
        if schedule is not None:
            fields.update(schedule_to_fields(schedule))
        if instruction is not None:
            fields["instruction"] = instruction
        if session_target is not None:
            fields.update(session_target_to_fields(session_target))
        if delivery is not None:
            fields.update(delivery_to_fields(delivery))
        if source is not None:
            fields.update(source_to_fields(source))
        if enabled is not None:
            fields["enabled"] = enabled
        updated = await self._store.upsert_job(**fields)
        await self._sync_job(updated)
        return self._build_job_view(updated)

    async def set_job_enabled(self, job_id: str, *, enabled: bool) -> CronJobView:
        return await self.update_job(job_id, enabled=enabled)

    async def delete_job(self, job_id: str) -> None:
        await self._store.delete_job(job_id)
        await self._timer.remove_job(job_id)

    async def run_now(self, job_id: str) -> CronExecutionResult:
        row = await self._require_job(job_id)
        run_id = None if str(row.session_target_kind) == "main" else self._id_factory()
        return await self._runner.run_job(
            row,
            run_id=run_id,
            trigger_kind="manual",
            scheduled_for=self._now_fn(),
        )

    async def list_runs(self, job_id: str) -> list[CronRunView]:
        rows = await self._store.list_runs_by_job(job_id)
        return [CronRunView(**row_to_run_dict(row)) for row in rows]

    async def _run_due_job(self, job_id: str) -> datetime | None:
        row = await self._store.get_job(job_id)
        if row is None or not bool(row.enabled):
            return None
        run_id = None if str(row.session_target_kind) == "main" else self._id_factory()
        await self._runner.run_job(
            row,
            run_id=run_id,
            trigger_kind="cron",
            scheduled_for=self._now_fn(),
        )
        next_run_at = self._compute_next_run(row)
        await self._timer.sync_job(row, next_run_at)
        return next_run_at

    def _build_job_view(self, row) -> CronJobView:
        runtime = self._timer.get_runtime_status(str(row.job_id))
        return CronJobView(**row_to_job_dict(row, runtime=runtime))

    async def _sync_job(self, row) -> None:
        await self._timer.sync_job(row, self._compute_next_run(row))

    def _compute_next_run(self, row) -> datetime | None:
        if not bool(row.enabled):
            return None
        return compute_next_run_datetime(row_to_job_dict(row)["schedule"], self._now_fn())

    async def _require_job(self, job_id: str):
        row = await self._store.get_job(job_id)
        if row is None:
            raise ValueError(f"cron job not found: {job_id}")
        return row

_cron_service: CronService | None = None


def get_cron_service() -> CronService:
    global _cron_service
    if _cron_service is None:
        _cron_service = CronService()
    return _cron_service
