# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_sql_repository.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""SQL 仓储基类。"""

from __future__ import annotations

from sqlalchemy.ext.asyncio import AsyncSession


class BaseSqlRepository:
    """异步 SQL 仓储基类。"""

    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def flush(self) -> None:
        """刷新当前事务。"""
        await self._session.flush()

    async def refresh(self, entity: object) -> None:
        """刷新实体，拉取数据库默认值。"""
        await self._session.refresh(entity)
