# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：connector_sql_repository.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""Connector 连接记录 SQL 仓储。"""

from __future__ import annotations

from datetime import datetime
from typing import Optional

from sqlalchemy import delete, select

from agent.infra.database.models.connector import ConnectorConnection
from agent.storage.sqlite.base_sql_repository import BaseSqlRepository


class ConnectorSqlRepository(BaseSqlRepository):
    """连接器授权记录 CRUD。"""

    async def get_connection(self, connector_id: str) -> Optional[ConnectorConnection]:
        stmt = select(ConnectorConnection).where(
            ConnectorConnection.connector_id == connector_id
        )
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def list_connections(self) -> list[ConnectorConnection]:
        stmt = select(ConnectorConnection).order_by(ConnectorConnection.connector_id)
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def get_connection_by_oauth_state(self, oauth_state: str) -> Optional[ConnectorConnection]:
        stmt = select(ConnectorConnection).where(
            ConnectorConnection.oauth_state == oauth_state
        )
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def set_oauth_state(
        self,
        connector_id: str,
        oauth_state: str | None,
        oauth_state_expires_at: datetime | None,
    ) -> ConnectorConnection:
        """仅更新 OAuth state，不覆盖现有连接状态和凭证。"""
        existing = await self.get_connection(connector_id)
        if existing:
            existing.oauth_state = oauth_state
            existing.oauth_state_expires_at = oauth_state_expires_at
            await self.flush()
            return existing

        row = ConnectorConnection(
            connector_id=connector_id,
            state="disconnected",
            credentials="",
            auth_type="oauth2",
            oauth_state=oauth_state,
            oauth_state_expires_at=oauth_state_expires_at,
        )
        self._session.add(row)
        await self.flush()
        return row

    async def upsert_connection(
        self,
        connector_id: str,
        state: str = "connected",
        credentials: str = "",
        auth_type: str = "oauth2",
        oauth_state: str | None = None,
        oauth_state_expires_at: datetime | None = None,
    ) -> ConnectorConnection:
        """插入或更新连接记录。"""
        existing = await self.get_connection(connector_id)
        if existing:
            existing.state = state
            if credentials:
                existing.credentials = credentials
            existing.auth_type = auth_type
            existing.oauth_state = oauth_state
            existing.oauth_state_expires_at = oauth_state_expires_at
            await self.flush()
            return existing
        row = ConnectorConnection(
            connector_id=connector_id,
            state=state,
            credentials=credentials,
            auth_type=auth_type,
            oauth_state=oauth_state,
            oauth_state_expires_at=oauth_state_expires_at,
        )
        self._session.add(row)
        await self.flush()
        return row

    async def delete_connection(self, connector_id: str) -> None:
        stmt = delete(ConnectorConnection).where(
            ConnectorConnection.connector_id == connector_id
        )
        await self._session.execute(stmt)
        await self.flush()
