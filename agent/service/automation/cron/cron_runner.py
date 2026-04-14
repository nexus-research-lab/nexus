# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_runner.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron 执行器。"""

from __future__ import annotations

from datetime import datetime, timezone

from agent.infra.schemas.model_cython import AModel
from agent.service.automation.cron.cron_normalizer import resolve_session_key
from agent.service.automation.cron.cron_run_log import CronRunLog
from agent.service.automation.runtime.run_context import AutomationRunContext


class CronExecutionResult(AModel):
    """单次 cron 执行结果。"""

    job_id: str
    run_id: str | None = None
    status: str
    session_key: str
    scheduled_for: datetime | None = None
    round_id: str | None = None
    session_id: str | None = None
    message_count: int = 0
    error_message: str | None = None


class CronRunner:
    """按 session target 选择 main queue 或 orchestrator。"""

    def __init__(
        self,
        *,
        store,
        system_event_queue=None,
        heartbeat_service=None,
        agent_run_orchestrator=None,
        run_log: CronRunLog | None = None,
        now_fn=None,
    ) -> None:
        if system_event_queue is None:
            from agent.service.automation.runtime.system_event_queue import SystemEventQueue

            system_event_queue = SystemEventQueue()
        if heartbeat_service is None:
            from agent.service.automation.heartbeat.heartbeat_service import heartbeat_service as default_heartbeat_service

            heartbeat_service = default_heartbeat_service
        if agent_run_orchestrator is None:
            from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator

            agent_run_orchestrator = AgentRunOrchestrator()
        self._system_event_queue = system_event_queue
        self._heartbeat_service = heartbeat_service
        self._agent_run_orchestrator = agent_run_orchestrator
        self._run_log = run_log or CronRunLog(store=store, now_fn=now_fn)
        self._now_fn = now_fn or (lambda: datetime.now(timezone.utc))

    async def run_job(
        self,
        job,
        *,
        run_id: str | None = None,
        trigger_kind: str,
        scheduled_for: datetime | None = None,
    ) -> CronExecutionResult:
        session_key = resolve_session_key(job, run_id=run_id)
        if str(job.session_target_kind) == "main":
            return await self._queue_main_session(
                job,
                session_key=session_key,
                trigger_kind=trigger_kind,
                scheduled_for=scheduled_for,
            )
        if not run_id:
            raise ValueError("run_id is required for non-main cron execution")
        return await self._run_isolated_target(
            job,
            run_id=run_id,
            session_key=session_key,
            trigger_kind=trigger_kind,
            scheduled_for=scheduled_for,
        )

    async def _queue_main_session(
        self,
        job,
        *,
        session_key: str,
        trigger_kind: str,
        scheduled_for: datetime | None,
    ) -> CronExecutionResult:
        payload = {
            "job_id": job.job_id,
            "agent_id": job.agent_id,
            "text": job.instruction,
            "session_target_kind": job.session_target_kind,
            "trigger_kind": trigger_kind,
        }
        event = await self._system_event_queue.enqueue(
            event_type="cron.trigger",
            source_type="cron",
            source_id=job.job_id,
            payload=payload,
        )
        try:
            await self._heartbeat_service.wake(
                agent_id=job.agent_id,
                mode=job.wake_mode,
                text=None,
            )
        except Exception:
            # 中文注释：main 目标先入队再唤醒，如果唤醒失败必须把刚写入的事件标记失败，
            # 否则 dispatcher 之后仍会消费到这条残留事件，形成幽灵执行。
            await self._system_event_queue.mark_failed(event.event_id)
            raise
        return CronExecutionResult(
            job_id=job.job_id,
            status="queued_to_main_session",
            session_key=session_key,
            scheduled_for=scheduled_for,
        )

    async def _run_isolated_target(
        self,
        job,
        *,
        run_id: str,
        session_key: str,
        trigger_kind: str,
        scheduled_for: datetime | None,
    ) -> CronExecutionResult:
        await self._run_log.create_pending(
            job_id=job.job_id,
            run_id=run_id,
            scheduled_for=scheduled_for,
        )
        await self._run_log.mark_running(run_id)

        try:
            result = await self._agent_run_orchestrator.run_turn(
                AutomationRunContext(
                    agent_id=job.agent_id,
                    session_key=session_key,
                    instruction=job.instruction,
                    trigger_kind="cron",
                    delivery_mode=job.delivery_mode,
                    metadata={
                        "job_id": job.job_id,
                        "job_name": getattr(job, "name", ""),
                        "run_id": run_id,
                        "trigger_kind": trigger_kind,
                        "scheduled_for": scheduled_for.isoformat() if scheduled_for else None,
                    },
                )
            )
        except Exception as exc:
            await self._run_log.mark_failed(run_id, str(exc))
            return CronExecutionResult(
                job_id=job.job_id,
                run_id=run_id,
                status="failed",
                session_key=session_key,
                scheduled_for=scheduled_for,
                error_message=str(exc),
            )

        if result.ok:
            await self._run_log.mark_succeeded(run_id)
            status = "succeeded"
        else:
            await self._run_log.mark_failed(run_id, result.error_message)
            status = "failed"
        return CronExecutionResult(
            job_id=job.job_id,
            run_id=run_id,
            status=status,
            session_key=session_key,
            scheduled_for=scheduled_for,
            round_id=result.round_id,
            session_id=result.session_id,
            message_count=result.message_count,
            error_message=result.error_message,
        )
