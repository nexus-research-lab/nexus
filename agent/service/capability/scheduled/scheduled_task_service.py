# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：scheduled_task_service.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Scheduled task 产品门面。"""

from __future__ import annotations

from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationCronSource,
    AutomationDeliveryTarget,
    AutomationSessionTarget,
)
from agent.service.automation.cron.cron_service import get_cron_service


class ScheduledTaskService:
    """把 cron service 暴露为 capability 语义。"""

    def __init__(self, *, service=None) -> None:
        self._service = service

    async def list_tasks(self, *, agent_id: str | None = None):
        return await self._service_instance.list_jobs(agent_id=agent_id)

    async def create_task(self, payload: AutomationCronJobCreate):
        return await self._service_instance.create_job(payload)

    async def update_task(
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
    ):
        return await self._service_instance.update_job(
            job_id,
            name=name,
            agent_id=agent_id,
            schedule=schedule,
            instruction=instruction,
            session_target=session_target,
            delivery=delivery,
            source=source,
            enabled=enabled,
        )

    async def delete_task(self, job_id: str) -> None:
        await self._service_instance.delete_job(job_id)

    async def run_task_now(self, job_id: str):
        return await self._service_instance.run_now(job_id)

    async def update_task_status(self, job_id: str, *, enabled: bool):
        return await self._service_instance.set_job_enabled(job_id, enabled=enabled)

    async def set_task_enabled(self, job_id: str, *, enabled: bool):
        """兼容主智能体编排侧的启停命名。"""
        return await self.update_task_status(job_id, enabled=enabled)

    async def list_task_runs(self, job_id: str):
        return await self._service_instance.list_runs(job_id)

    @property
    def _service_instance(self):
        return self._service or get_cron_service()


scheduled_task_service = ScheduledTaskService()
