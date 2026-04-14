# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_scheduled_task.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Scheduled task API。"""

from __future__ import annotations

from fastapi import APIRouter

from agent.infra.schemas.model_cython import AModel
from agent.infra.server.common import resp
from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationCronSource,
    AutomationDeliveryTarget,
    AutomationSessionTarget,
)
from agent.service.capability.scheduled.scheduled_task_service import (
    scheduled_task_service,
)

router = APIRouter(prefix="/capability/scheduled/tasks", tags=["capability"])


class ScheduledTaskUpdateRequest(AModel):
    """任务更新请求。"""

    name: str | None = None
    schedule: AutomationCronSchedule | None = None
    instruction: str | None = None
    session_target: AutomationSessionTarget | None = None
    delivery: AutomationDeliveryTarget | None = None
    enabled: bool | None = None


class ScheduledTaskCreateRequest(AutomationCronJobCreate):
    """任务创建请求。"""

    source: AutomationCronSource | None = None


class ScheduledTaskStatusRequest(AModel):
    """任务启停请求。"""

    enabled: bool


@router.get("")
async def list_scheduled_tasks(agent_id: str | None = None):
    data = await scheduled_task_service.list_tasks(agent_id=agent_id)
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in data]))


@router.post("")
async def create_scheduled_task(payload: ScheduledTaskCreateRequest):
    source = payload.source or AutomationCronSource()
    source.kind = "user_page"
    data = await scheduled_task_service.create_task(
        AutomationCronJobCreate(
            name=payload.name,
            agent_id=payload.agent_id,
            schedule=payload.schedule,
            instruction=payload.instruction,
            session_target=payload.session_target,
            delivery=payload.delivery,
            source=source,
            enabled=payload.enabled,
        )
    )
    return resp.ok(resp.Resp(data=data.model_dump(mode="json")))


@router.patch("/{job_id}")
async def update_scheduled_task(job_id: str, payload: ScheduledTaskUpdateRequest):
    data = await scheduled_task_service.update_task(
        job_id,
        name=payload.name,
        schedule=payload.schedule,
        instruction=payload.instruction,
        session_target=payload.session_target,
        delivery=payload.delivery,
        enabled=payload.enabled,
    )
    return resp.ok(resp.Resp(data=data.model_dump(mode="json")))


@router.delete("/{job_id}")
async def delete_scheduled_task(job_id: str):
    await scheduled_task_service.delete_task(job_id)
    return resp.ok(resp.Resp(data={"job_id": job_id}))


@router.post("/{job_id}/run")
async def run_scheduled_task(job_id: str):
    data = await scheduled_task_service.run_task_now(job_id)
    return resp.ok(resp.Resp(data=data.model_dump(mode="json")))


@router.patch("/{job_id}/status")
async def update_scheduled_task_status(job_id: str, payload: ScheduledTaskStatusRequest):
    data = await scheduled_task_service.update_task_status(job_id, enabled=payload.enabled)
    return resp.ok(resp.Resp(data=data.model_dump(mode="json")))


@router.get("/{job_id}/runs")
async def list_scheduled_task_runs(job_id: str):
    data = await scheduled_task_service.list_task_runs(job_id)
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in data]))
