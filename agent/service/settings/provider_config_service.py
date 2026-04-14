# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：provider_config_service.py
# @Date   ：2026/04/14 10:14
# @Author ：leemysw
# 2026/04/14 10:14   Create
# =====================================================

"""Provider 配置服务。"""

from __future__ import annotations

import re

from agent.infra.database.get_db import get_db
from agent.infra.database.models.provider_config import ProviderConfig
from agent.infra.database.repositories.provider_config_sql_repository import (
    ProviderConfigSqlRepository,
)
from agent.schema.model_provider_config import (
    CreateProviderConfigRequest,
    ProviderConfigRecord,
    ProviderOption,
    ProviderOptionsResponse,
    ProviderRuntimeConfig,
    UpdateProviderConfigRequest,
)
from agent.utils.utils import random_uuid


def normalize_provider(provider: str | None, allow_empty: bool = False) -> str:
    """把 provider key 清理成稳定标识。"""
    cleaned = (provider or "").strip().lower()
    if not cleaned:
        if allow_empty:
            return ""
        raise ValueError("provider 不能为空")
    normalized = re.sub(r"[^a-z0-9]+", "-", cleaned).strip("-")
    if normalized:
        return normalized
    raise ValueError(f"非法的 provider: {provider}")


class ProviderConfigService:
    """负责 Provider 配置的持久化与运行时解析。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def list_provider_configs(self) -> list[ProviderConfigRecord]:
        """列出完整 Provider 配置。"""
        async with self._db.session() as session:
            repository = ProviderConfigSqlRepository(session)
            changed = await self._ensure_default_config(repository)
            entities = await repository.list_all()
            usage_counts = await repository.list_usage_counts()
            if changed:
                await session.commit()
            return [
                repository.to_record(item, usage_count=usage_counts.get(item.provider, 0))
                for item in entities
            ]

    async def list_provider_options(self) -> ProviderOptionsResponse:
        """列出供前端选择的启用 Provider。"""
        async with self._db.session() as session:
            repository = ProviderConfigSqlRepository(session)
            changed = await self._ensure_default_config(repository)
            entities = [item for item in await repository.list_all() if item.enabled]
            if changed:
                await session.commit()
            default_provider = next(
                (item.provider for item in entities if item.is_default),
                None,
            )
            return ProviderOptionsResponse(
                default_provider=default_provider,
                items=[
                    ProviderOption(
                        provider=item.provider,
                        display_name=item.display_name,
                        is_default=item.is_default,
                    )
                    for item in entities
                ],
            )

    async def get_default_provider(self) -> str | None:
        """返回当前默认 Provider。"""
        options = await self.list_provider_options()
        return options.default_provider

    async def create_provider_config(
        self,
        body: CreateProviderConfigRequest,
    ) -> ProviderConfigRecord:
        """创建 Provider 配置。"""
        provider = normalize_provider(body.provider)
        payload = self._validate_payload(
            display_name=body.display_name,
            auth_token=body.auth_token,
            base_url=body.base_url,
            model=body.model,
            enabled=body.enabled,
            is_default=body.is_default,
            require_auth_token=True,
        )

        async with self._db.session() as session:
            repository = ProviderConfigSqlRepository(session)
            existing = await repository.get_by_provider(provider)
            if existing is not None:
                raise ValueError(f"provider 已存在: {provider}")

            entity = ProviderConfig(
                id=random_uuid(),
                provider=provider,
                display_name=payload["display_name"],
                auth_token=payload["auth_token"],
                base_url=payload["base_url"],
                model=payload["model"],
                enabled=payload["enabled"],
                is_default=payload["is_default"],
            )
            await repository.create(entity)
            await self._ensure_default_config(repository, preferred_provider=provider if entity.is_default else None)
            await session.commit()
            usage_counts = await repository.list_usage_counts()
            return repository.to_record(entity, usage_count=usage_counts.get(provider, 0))

    async def update_provider_config(
        self,
        provider: str,
        body: UpdateProviderConfigRequest,
    ) -> ProviderConfigRecord:
        """更新 Provider 配置。"""
        normalized_provider = normalize_provider(provider)
        payload = self._validate_payload(
            display_name=body.display_name,
            auth_token=body.auth_token,
            base_url=body.base_url,
            model=body.model,
            enabled=body.enabled,
            is_default=body.is_default,
            require_auth_token=False,
        )

        async with self._db.session() as session:
            repository = ProviderConfigSqlRepository(session)
            entity = await repository.get_by_provider(normalized_provider)
            if entity is None:
                raise ValueError(f"provider 不存在: {normalized_provider}")

            usage_counts = await repository.list_usage_counts()
            usage_count = usage_counts.get(normalized_provider, 0)
            if usage_count > 0 and not payload["enabled"]:
                raise ValueError(f"provider={normalized_provider} 仍被 {usage_count} 个 Agent 使用，不能禁用")

            entity.display_name = payload["display_name"]
            if payload["auth_token"]:
                entity.auth_token = str(payload["auth_token"])
            entity.base_url = payload["base_url"]
            entity.model = payload["model"]
            entity.enabled = payload["enabled"]
            entity.is_default = payload["is_default"]
            await repository.flush()
            await self._ensure_default_config(
                repository,
                preferred_provider=normalized_provider if entity.is_default else None,
            )
            await session.commit()
            await repository.refresh(entity)
            usage_counts = await repository.list_usage_counts()
            return repository.to_record(entity, usage_count=usage_counts.get(normalized_provider, 0))

    async def delete_provider_config(self, provider: str) -> None:
        """删除 Provider 配置。"""
        normalized_provider = normalize_provider(provider)
        async with self._db.session() as session:
            repository = ProviderConfigSqlRepository(session)
            entity = await repository.get_by_provider(normalized_provider)
            if entity is None:
                raise ValueError(f"provider 不存在: {normalized_provider}")

            usage_counts = await repository.list_usage_counts()
            usage_count = usage_counts.get(normalized_provider, 0)
            if usage_count > 0:
                raise ValueError(f"provider={normalized_provider} 仍被 {usage_count} 个 Agent 使用，不能删除")

            await session.delete(entity)
            await repository.flush()
            await self._ensure_default_config(repository)
            await session.commit()

    async def resolve_runtime_config(self, provider: str | None) -> ProviderRuntimeConfig:
        """解析某个 Agent 最终应该使用的 Provider 运行时配置。"""
        normalized_provider = normalize_provider(provider, allow_empty=True)
        async with self._db.session() as session:
            repository = ProviderConfigSqlRepository(session)
            changed = await self._ensure_default_config(repository)
            target = None
            if normalized_provider:
                target = await repository.get_by_provider(normalized_provider)
                if target is None:
                    raise ValueError(f"provider 不存在: {normalized_provider}")
            if target is None:
                options = [item for item in await repository.list_all() if item.enabled and item.is_default]
                target = options[0] if options else None
            if changed:
                await session.commit()

            if target is None:
                raise ValueError("未配置可用的 Provider，请先到 Settings 添加 Provider")
            if not target.enabled:
                raise ValueError(f"provider={target.provider} 已禁用")

            missing_fields = []
            if not target.auth_token.strip():
                missing_fields.append("auth_token")
            if not target.base_url.strip():
                missing_fields.append("base_url")
            if not target.model.strip():
                missing_fields.append("model")
            if missing_fields:
                raise ValueError(
                    f"provider={target.provider} 配置不完整: {', '.join(missing_fields)}"
                )

            return ProviderRuntimeConfig(
                provider=target.provider,
                display_name=target.display_name,
                auth_token=target.auth_token.strip(),
                base_url=target.base_url.strip(),
                model=target.model.strip(),
            )

    async def _ensure_default_config(
        self,
        repository: ProviderConfigSqlRepository,
        preferred_provider: str | None = None,
    ) -> bool:
        """确保启用列表里始终只有一个默认 Provider。"""
        entities = await repository.list_all()
        enabled_entities = [item for item in entities if item.enabled]
        if not entities:
            return False

        target_provider = None
        if preferred_provider:
            target_provider = next(
                (item.provider for item in enabled_entities if item.provider == preferred_provider),
                None,
            )
        if target_provider is None:
            target_provider = next(
                (item.provider for item in enabled_entities if item.is_default),
                None,
            )
        if target_provider is None and enabled_entities:
            target_provider = enabled_entities[0].provider

        changed = False
        for item in entities:
            expected_default = item.provider == target_provider if target_provider else False
            if item.is_default != expected_default:
                item.is_default = expected_default
                changed = True
        if changed:
            await repository.flush()
        return changed

    @staticmethod
    def _validate_payload(
        *,
        display_name: str,
        auth_token: str | None,
        base_url: str,
        model: str,
        enabled: bool,
        is_default: bool,
        require_auth_token: bool,
    ) -> dict[str, str | bool]:
        """清理并校验 Provider 输入。"""
        cleaned_display_name = display_name.strip()
        cleaned_auth_token = (auth_token or "").strip()
        cleaned_base_url = base_url.strip()
        cleaned_model = model.strip()
        if not cleaned_display_name:
            raise ValueError("display_name 不能为空")
        if require_auth_token and not cleaned_auth_token:
            raise ValueError("auth_token 不能为空")
        if not cleaned_base_url:
            raise ValueError("base_url 不能为空")
        if not cleaned_model:
            raise ValueError("model 不能为空")
        return {
            "display_name": cleaned_display_name,
            "auth_token": cleaned_auth_token,
            "base_url": cleaned_base_url,
            "model": cleaned_model,
            "enabled": bool(enabled),
            "is_default": bool(is_default),
        }


provider_config_service = ProviderConfigService()
