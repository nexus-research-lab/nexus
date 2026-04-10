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

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.automation_cron_job_sql_repository import (
    AutomationCronJobSqlRepository,
)
from agent.infra.database.repositories.automation_cron_run_sql_repository import (
    AutomationCronRunSqlRepository,
)


class CronStoreService:
    """薄封装的定时任务存储。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

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
        async with self._db.session() as session:
            repo = AutomationCronJobSqlRepository(session)
            row = await repo.upsert_job(**fields)
            await session.commit()
            return row

    async def delete_job(self, job_id: str) -> None:
        """删除任务定义。"""
        async with self._db.session() as session:
            repo = AutomationCronJobSqlRepository(session)
            await repo.delete_job(job_id)
            await session.commit()

    async def create_run(self, **fields):
        """创建执行记录。"""
        async with self._db.session() as session:
            repo = AutomationCronRunSqlRepository(session)
            row = await repo.create_run(**fields)
            await session.commit()
            return row

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
        async with self._db.session() as session:
            repo = AutomationCronRunSqlRepository(session)
            row = await repo.update_run_status(**fields)
            await session.commit()
            return row
