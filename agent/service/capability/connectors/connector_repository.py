# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：connector_repository.py
# @Date   ：2026/4/1 18:38
# @Author ：leemysw
# 2026/4/1 18:38   Create
# =====================================================

"""Connector 授权连接仓库。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from agent.infra.database.get_db import get_db
from agent.infra.database.models.connector import ConnectorConnection


class ConnectorRepository:
    """负责连接器授权记录的持久化。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def get_connection_states(self) -> dict[str, str]:
        """获取所有连接器当前连接状态。"""
        async with self._db.session() as session:
            result = await session.execute(
                select(ConnectorConnection.connector_id, ConnectorConnection.state)
            )
            return {connector_id: state for connector_id, state in result.all()}

    async def get_connected_count(self) -> int:
        """统计已连接的连接器数量。"""
        async with self._db.session() as session:
            result = await session.execute(
                select(func.count()).select_from(ConnectorConnection).where(
                    ConnectorConnection.state == "connected"
                )
            )
            return int(result.scalar_one() or 0)

    async def get_connection(self, connector_id: str) -> ConnectorConnection | None:
        """获取单个连接器连接记录。"""
        async with self._db.session() as session:
            return await session.get(ConnectorConnection, connector_id)

    async def get_connection_by_oauth_state(
        self,
        oauth_state: str,
    ) -> ConnectorConnection | None:
        """按 OAuth state 读取连接记录。"""
        async with self._db.session() as session:
            result = await session.execute(
                select(ConnectorConnection).where(ConnectorConnection.oauth_state == oauth_state)
            )
            return result.scalar_one_or_none()

    async def set_oauth_state(
        self,
        connector_id: str,
        oauth_state: str,
        oauth_state_expires_at: datetime | None,
    ) -> None:
        """保存 OAuth 临时 state。"""
        async with self._db.session() as session:
            entity = await self._get_or_create_connection(session, connector_id)
            entity.oauth_state = oauth_state
            entity.oauth_state_expires_at = oauth_state_expires_at
            await session.commit()

    async def connect(
        self,
        connector_id: str,
        credentials: str,
        auth_type: str,
        state: str,
        oauth_state: str | None,
        oauth_state_expires_at: datetime | None,
    ) -> None:
        """保存连接器授权结果。"""
        async with self._db.session() as session:
            entity = await self._get_or_create_connection(session, connector_id)
            entity.credentials = credentials
            entity.auth_type = auth_type
            entity.state = state
            entity.oauth_state = oauth_state
            entity.oauth_state_expires_at = oauth_state_expires_at
            await session.commit()

    async def disconnect(self, connector_id: str) -> None:
        """断开连接器连接并清空临时状态。"""
        async with self._db.session() as session:
            entity = await session.get(ConnectorConnection, connector_id)
            if entity is None:
                return
            entity.state = "disconnected"
            entity.credentials = ""
            entity.oauth_state = None
            entity.oauth_state_expires_at = None
            await session.commit()

    @staticmethod
    async def _get_or_create_connection(
        session: AsyncSession,
        connector_id: str,
    ) -> ConnectorConnection:
        """读取连接器记录，不存在时创建默认记录。"""
        entity = await session.get(ConnectorConnection, connector_id)
        if entity is not None:
            return entity

        entity = ConnectorConnection(
            connector_id=connector_id,
            state="disconnected",
            credentials="",
            auth_type="oauth2",
        )
        session.add(entity)
        await session.flush()
        return entity


connector_repository = ConnectorRepository()
