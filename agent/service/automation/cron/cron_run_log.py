# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_run_log.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron run ledger。"""

from __future__ import annotations

from datetime import datetime, timezone


class CronRunLog:
    """围绕 cron run store 的轻量状态迁移器。"""

    def __init__(self, *, store, now_fn=None) -> None:
        self._store = store
        self._now_fn = now_fn or (lambda: datetime.now(timezone.utc))

    async def create_pending(
        self,
        *,
        job_id: str,
        run_id: str,
        scheduled_for: datetime | None,
    ):
        return await self._store.create_run(
            run_id=run_id,
            job_id=job_id,
            scheduled_for=scheduled_for,
        )

    async def mark_running(self, run_id: str):
        return await self._store.update_run_status(
            run_id=run_id,
            status="running",
            started_at=self._now_fn(),
            attempts=1,
            error_message=None,
        )

    async def mark_succeeded(self, run_id: str):
        return await self._store.update_run_status(
            run_id=run_id,
            status="succeeded",
            finished_at=self._now_fn(),
            error_message=None,
        )

    async def mark_failed(self, run_id: str, error_message: str | None):
        return await self._store.update_run_status(
            run_id=run_id,
            status="failed",
            finished_at=self._now_fn(),
            error_message=error_message,
        )
