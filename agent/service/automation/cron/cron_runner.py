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
from agent.service.automation.delivery.delivery_memory import DeliveryMemory
from agent.service.automation.delivery.delivery_router import DeliveryRouter
from agent.service.automation.cron.cron_normalizer import resolve_session_key
from agent.service.automation.cron.cron_run_log import CronRunLog
from agent.service.automation.runtime.run_context import AutomationRunContext
from agent.service.channels.message_sender import MessageSender
from agent.service.channels.ws.ws_session_routing_sender import WsSessionRoutingSender


class _NoopMessageSender(MessageSender):
    """为 websocket delivery 提供兜底 sender，避免无活跃连接时中断任务。"""

    async def send_message(self, message) -> None:
        del message

    async def send_stream_message(self, message) -> None:
        del message

    async def send_event_message(self, event) -> None:
        del event


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
        delivery_router=None,
        message_store=None,
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
        if delivery_router is None:
            delivery_router = DeliveryRouter(
                memory=DeliveryMemory(),
                websocket_sender=WsSessionRoutingSender(_NoopMessageSender()),
            )
        if message_store is None:
            from agent.service.session.session_store import session_store as default_session_store

            message_store = default_session_store
        self._system_event_queue = system_event_queue
        self._heartbeat_service = heartbeat_service
        self._agent_run_orchestrator = agent_run_orchestrator
        self._delivery_router = delivery_router
        self._message_store = message_store
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
                    trigger_kind=trigger_kind,
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
            await self._deliver_result_text(job, session_key=session_key, round_id=result.round_id)
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

    async def _deliver_result_text(self, job, *, session_key: str, round_id: str | None) -> None:
        if str(getattr(job, "delivery_mode", "none")) == "none":
            return
        text = await self._extract_result_text(session_key, round_id)
        if not text.strip():
            return
        await self._delivery_router.send_text(
            agent_id=str(getattr(job, "agent_id", "")),
            text=text,
            target={
                "mode": getattr(job, "delivery_mode", "none"),
                "channel": getattr(job, "delivery_channel", None),
                "to": getattr(job, "delivery_to", None),
                "account_id": getattr(job, "delivery_account_id", None),
                "thread_id": getattr(job, "delivery_thread_id", None),
            },
        )

    async def _extract_result_text(self, session_key: str, round_id: str | None) -> str:
        messages = await self._message_store.get_session_messages(session_key)
        for message in reversed(messages):
            if round_id and getattr(message, "round_id", None) != round_id:
                continue
            if getattr(message, "role", None) == "result" and getattr(message, "result", None):
                return str(message.result)
            content = getattr(message, "content", None)
            if getattr(message, "role", None) != "assistant" or not isinstance(content, list):
                continue
            text_parts = [getattr(block, "text", "") for block in content if getattr(block, "type", "") == "text"]
            if text_parts:
                return "\n".join(part for part in text_parts if part)
        return ""
