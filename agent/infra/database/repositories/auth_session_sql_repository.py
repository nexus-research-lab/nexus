# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：auth_session_sql_repository.py
# @Date   ：2026/04/08 12:02
# @Author ：leemysw
# 2026/04/08 12:02   Create
# =====================================================

"""登录会话 SQL 仓储。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import delete, select

from agent.infra.database.models.auth_session import AuthSession
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.schema.model_auth import AuthSessionRecord


class AuthSessionSqlRepository(BaseSqlRepository):
    """浏览器登录会话仓储。"""

    async def create(self, record: AuthSessionRecord) -> AuthSessionRecord:
        """创建登录会话。"""
        entity = AuthSession(
            **record.model_dump(exclude={"created_at", "updated_at"})
        )
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return AuthSessionRecord.model_validate(entity)

    async def get_by_token_hash(self, token_hash: str) -> AuthSessionRecord | None:
        """按会话摘要查询登录会话。"""
        stmt = select(AuthSession).where(AuthSession.session_token_hash == token_hash)
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        if entity is None:
            return None
        return AuthSessionRecord.model_validate(entity)

    async def delete_by_token_hash(self, token_hash: str) -> bool:
        """按会话摘要删除登录会话。"""
        stmt = delete(AuthSession).where(AuthSession.session_token_hash == token_hash)
        result = await self._session.execute(stmt)
        await self.flush()
        return bool(result.rowcount)

    async def delete_expired(self, expires_before: datetime) -> int:
        """删除已过期会话。"""
        stmt = delete(AuthSession).where(AuthSession.expires_at <= expires_before)
        result = await self._session.execute(stmt)
        await self.flush()
        return int(result.rowcount or 0)
