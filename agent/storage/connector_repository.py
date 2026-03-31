# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：connector_repository.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""Connector 仓库 —— 连接器授权状态的数据库操作封装。"""

from __future__ import annotations

from typing import Optional

from agent.infra.database.get_db import get_db
from agent.infra.database.models.connector import ConnectorConnection
from agent.storage.sqlite.connector_sql_repository import ConnectorSqlRepository


class ConnectorRepository:
    """连接器授权记录的高层仓库。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def get_connection_states(self) -> dict[str, str]:
        """返回 {connector_id: state} 映射。"""
        async with self._db.session() as session:
            repo = ConnectorSqlRepository(session)
            rows = await repo.list_connections()
        return {row.connector_id: row.state for row in rows}

    async def get_connection(self, connector_id: str) -> Optional[ConnectorConnection]:
        async with self._db.session() as session:
            repo = ConnectorSqlRepository(session)
            return await repo.get_connection(connector_id)

    async def get_connection_by_oauth_state(self, oauth_state: str) -> Optional[ConnectorConnection]:
        async with self._db.session() as session:
            repo = ConnectorSqlRepository(session)
            return await repo.get_connection_by_oauth_state(oauth_state)

    async def set_oauth_state(
        self,
        connector_id: str,
        oauth_state: str | None,
        oauth_state_expires_at = None,
    ) -> None:
        """更新 OAuth state，不影响现有凭证与连接状态。"""
        async with self._db.session() as session:
            repo = ConnectorSqlRepository(session)
            await repo.set_oauth_state(
                connector_id=connector_id,
                oauth_state=oauth_state,
                oauth_state_expires_at=oauth_state_expires_at,
            )
            await session.commit()

    async def connect(
        self,
        connector_id: str,
        credentials: str = "",
        auth_type: str = "oauth2",
        state: str = "connected",
        oauth_state: str | None = None,
        oauth_state_expires_at = None,
    ) -> None:
        """建立连接（保存授权凭证）。"""
        async with self._db.session() as session:
            repo = ConnectorSqlRepository(session)
            await repo.upsert_connection(
                connector_id=connector_id,
                state=state,
                credentials=credentials,
                auth_type=auth_type,
                oauth_state=oauth_state,
                oauth_state_expires_at=oauth_state_expires_at,
            )
            await session.commit()

    async def disconnect(self, connector_id: str) -> None:
        """断开连接（删除记录）。"""
        async with self._db.session() as session:
            repo = ConnectorSqlRepository(session)
            await repo.delete_connection(connector_id)
            await session.commit()

    async def get_connected_count(self) -> int:
        """获取已连接的连接器数量。"""
        states = await self.get_connection_states()
        return sum(1 for s in states.values() if s == "connected")


# 全局单例
connector_repository = ConnectorRepository()
