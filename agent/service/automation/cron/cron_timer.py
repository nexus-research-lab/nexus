# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_timer.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron 定时器。"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass
from datetime import datetime, timezone


@dataclass(slots=True)
class CronTimerState:
    """单个 job 的运行态。"""

    job_id: str
    enabled: bool
    next_run_at: datetime | None = None
    inflight: bool = False
    last_run_at: datetime | None = None


class CronTimer:
    """维护 cron job 的下一次触发时间。"""

    def __init__(self, *, dispatcher, tick_seconds: float = 1.0, now_fn=None) -> None:
        self._dispatcher = dispatcher
        self._tick_seconds = tick_seconds
        self._states: dict[str, CronTimerState] = {}
        self._running = False
        self._task: asyncio.Task | None = None
        self._stop_event = asyncio.Event()
        self._now_fn = now_fn or (lambda: datetime.now(timezone.utc))

    async def start(self) -> None:
        if self._running:
            return
        self._running = True
        self._stop_event = asyncio.Event()
        self._task = asyncio.create_task(self._run_loop())

    async def stop(self) -> None:
        self._running = False
        self._stop_event.set()
        if self._task is not None:
            await self._task
            self._task = None

    async def sync_job(self, job, next_run_at: datetime | None) -> None:
        self._states[job.job_id] = CronTimerState(
            job_id=job.job_id,
            enabled=bool(job.enabled),
            next_run_at=next_run_at,
            last_run_at=self._states.get(job.job_id, CronTimerState(job.job_id, False)).last_run_at,
        )

    async def remove_job(self, job_id: str) -> None:
        self._states.pop(job_id, None)

    def get_runtime_status(self, job_id: str) -> dict[str, object]:
        state = self._states.get(job_id)
        if state is None:
            return {
                "running": self._running,
                "next_run_at": None,
                "last_run_at": None,
            }
        return {
            "running": self._running and state.inflight,
            "next_run_at": state.next_run_at,
            "last_run_at": state.last_run_at,
        }

    async def _run_loop(self) -> None:
        while not self._stop_event.is_set():
            await self._run_due_once()
            try:
                await asyncio.wait_for(self._stop_event.wait(), timeout=self._tick_seconds)
            except asyncio.TimeoutError:
                continue

    async def _run_due_once(self) -> None:
        now = self._now_fn()
        for state in list(self._states.values()):
            if state.inflight or not state.enabled or state.next_run_at is None:
                continue
            if state.next_run_at > now:
                continue
            state.inflight = True
            try:
                state.next_run_at = await self._dispatcher(state.job_id)
                state.last_run_at = now
            finally:
                state.inflight = False
