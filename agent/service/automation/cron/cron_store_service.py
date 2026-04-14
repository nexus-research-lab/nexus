# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_store_service.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron 存储服务。"""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable

from sqlalchemy.exc import OperationalError

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.automation_cron_job_sql_repository import (
    AutomationCronJobSqlRepository,
)
from agent.infra.database.repositories.automation_cron_run_sql_repository import (
    AutomationCronRunSqlRepository,
)


class CronStoreService:
    """薄封装的定时任务存储。"""

    def __init__(
        self,
        *,
        db=None,
        write_retry_attempts: int = 4,
        write_retry_delay_seconds: float = 0.05,
    ) -> None:
        self._db = db or get_db("async_sqlite")
        self._write_retry_attempts = max(1, int(write_retry_attempts))
        self._write_retry_delay_seconds = max(0.0, float(write_retry_delay_seconds))

    async def get_job(self, job_id: str):
        """读取任务定义。"""
        async with self._db.session() as session:
            repo = AutomationCronJobSqlRepository(session)
            return await repo.get_job(job_id)

    async def list_jobs(self, agent_id: str | None = None):
        """列出任务定义。"""
        async with self._db.session() as session:
            repo = AutomationCronJobSqlRepository(session)
            return await repo.list_jobs(agent_id=agent_id)

    async def upsert_job(self, **fields):
        """保存任务定义。"""
        async def _write(session):
            repo = AutomationCronJobSqlRepository(session)
            row = await repo.upsert_job(**fields)
            await session.commit()
            return row
        return await self._run_write(_write)

    async def delete_job(self, job_id: str) -> None:
        """删除任务定义。"""
        async def _write(session):
            repo = AutomationCronJobSqlRepository(session)
            await repo.delete_job(job_id)
            await session.commit()
        await self._run_write(_write)

    async def create_run(self, **fields):
        """创建执行记录。"""
        async def _write(session):
            repo = AutomationCronRunSqlRepository(session)
            row = await repo.create_run(**fields)
            await session.commit()
            return row
        return await self._run_write(_write)

    async def get_run(self, run_id: str):
        """读取执行记录。"""
        async with self._db.session() as session:
            repo = AutomationCronRunSqlRepository(session)
            return await repo.get_run(run_id)

    async def list_runs_by_job(self, job_id: str):
        """列出任务下的执行记录。"""
        async with self._db.session() as session:
            repo = AutomationCronRunSqlRepository(session)
            return await repo.list_runs_by_job(job_id)

    async def update_run_status(self, **fields):
        """更新执行记录状态。"""
        async def _write(session):
            repo = AutomationCronRunSqlRepository(session)
            row = await repo.update_run_status(**fields)
            await session.commit()
            return row
        return await self._run_write(_write)

    async def _run_write(self, operation: Callable[[object], Awaitable[object]]):
        last_error: OperationalError | None = None
        for attempt in range(self._write_retry_attempts):
            try:
                async with self._db.session() as session:
                    return await operation(session)
            except OperationalError as exc:
                if not self._is_sqlite_locked(exc) or attempt == self._write_retry_attempts - 1:
                    raise
                last_error = exc
                # 中文注释：SQLite 同一时刻只允许一个 writer，这里对瞬时锁竞争做短暂退避，
                # 避免前台创建任务与后台调度写入相撞时直接把 500 暴露给用户。
                await asyncio.sleep(self._write_retry_delay_seconds * (attempt + 1))

        if last_error is not None:
            raise last_error
        raise RuntimeError("cron store write retry exhausted without raising OperationalError")

    @staticmethod
    def _is_sqlite_locked(error: OperationalError) -> bool:
        return "database is locked" in str(error).lower()
