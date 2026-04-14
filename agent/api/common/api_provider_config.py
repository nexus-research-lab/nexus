# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_provider_config.py
# @Date   ：2026/04/14 10:14
# @Author ：leemysw
# 2026/04/14 10:14   Create
# =====================================================

"""Provider 配置 API。"""

from fastapi import APIRouter
from fastapi import status as http_status

from agent.infra.server.common import resp
from agent.schema.model_provider_config import (
    CreateProviderConfigRequest,
    UpdateProviderConfigRequest,
)
from agent.service.settings.provider_config_service import provider_config_service

router = APIRouter(prefix="/settings/providers", tags=["settings"])


@router.get("")
async def list_provider_configs():
    """列出完整 Provider 配置。"""
    items = await provider_config_service.list_provider_configs()
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in items]))


@router.get("/options")
async def list_provider_options():
    """列出可供 Agent 选择的 Provider 选项。"""
    payload = await provider_config_service.list_provider_options()
    return resp.ok(resp.Resp(data=payload.model_dump(mode="json")))


@router.post("")
async def create_provider_config(body: CreateProviderConfigRequest):
    """新增 Provider 配置。"""
    try:
        item = await provider_config_service.create_provider_config(body)
        return resp.ok(resp.Resp(data=item.model_dump(mode="json")))
    except ValueError as exc:
        failure = resp.Resp(code="400", http_status=http_status.HTTP_400_BAD_REQUEST)
        failure.set_detail(str(exc))
        return resp.fail(failure)


@router.put("/{provider}")
async def update_provider_config(provider: str, body: UpdateProviderConfigRequest):
    """更新 Provider 配置。"""
    try:
        item = await provider_config_service.update_provider_config(provider, body)
        return resp.ok(resp.Resp(data=item.model_dump(mode="json")))
    except ValueError as exc:
        failure = resp.Resp(code="400", http_status=http_status.HTTP_400_BAD_REQUEST)
        failure.set_detail(str(exc))
        return resp.fail(failure)


@router.delete("/{provider}")
async def delete_provider_config(provider: str):
    """删除 Provider 配置。"""
    try:
        await provider_config_service.delete_provider_config(provider)
        return resp.ok(resp.Resp(data={"provider": provider}))
    except ValueError as exc:
        failure = resp.Resp(code="400", http_status=http_status.HTTP_400_BAD_REQUEST)
        failure.set_detail(str(exc))
        return resp.fail(failure)
