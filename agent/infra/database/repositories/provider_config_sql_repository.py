# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：provider_config_sql_repository.py
# @Date   ：2026/04/14 10:14
# @Author ：leemysw
# 2026/04/14 10:14   Create
# =====================================================

"""Provider 配置 SQL 仓储。"""

from __future__ import annotations

from sqlalchemy import func, select

from agent.infra.database.models.agent import Agent
from agent.infra.database.models.provider_config import ProviderConfig
from agent.infra.database.models.runtime import Runtime
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.schema.model_provider_config import ProviderConfigRecord


class ProviderConfigSqlRepository(BaseSqlRepository):
    """Provider 配置仓储。"""

    @staticmethod
    def mask_auth_token(auth_token: str) -> str:
        """返回仅保留首尾的 token 掩码。"""
        cleaned_token = (auth_token or "").strip()
        if not cleaned_token:
            return ""
        if len(cleaned_token) <= 8:
            return f"{cleaned_token[:1]}***{cleaned_token[-1:]}"
        return f"{cleaned_token[:4]}***{cleaned_token[-4:]}"

    async def create(self, entity: ProviderConfig) -> ProviderConfig:
        """创建 Provider 配置。"""
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return entity

    async def get_by_provider(self, provider: str) -> ProviderConfig | None:
        """按 provider key 获取配置实体。"""
        stmt = select(ProviderConfig).where(ProviderConfig.provider == provider)
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def list_all(self) -> list[ProviderConfig]:
        """列出所有 Provider 配置。"""
        stmt = (
            select(ProviderConfig)
            .order_by(ProviderConfig.is_default.desc(), ProviderConfig.display_name.asc())
        )
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def list_usage_counts(self) -> dict[str, int]:
        """统计每个 provider 被活跃 Agent 使用的数量。"""
        stmt = (
            select(Runtime.provider, func.count(Runtime.agent_id))
            .join(Agent, Agent.id == Runtime.agent_id)
            .where(Agent.status == "active")
            .group_by(Runtime.provider)
        )
        result = await self._session.execute(stmt)
        return {provider: count for provider, count in result.all()}

    @staticmethod
    def to_record(entity: ProviderConfig, usage_count: int = 0) -> ProviderConfigRecord:
        """把 ORM 实体转换为响应模型。"""
        record = ProviderConfigRecord.model_validate(entity)
        record.auth_token_masked = ProviderConfigSqlRepository.mask_auth_token(entity.auth_token)
        record.usage_count = usage_count
        return record
