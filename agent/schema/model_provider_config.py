# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_provider_config.py
# @Date   ：2026/04/14 10:14
# @Author ：leemysw
# 2026/04/14 10:14   Create
# =====================================================

"""Provider 配置数据模型。"""

from __future__ import annotations

from datetime import datetime

from pydantic import Field

from agent.infra.schemas.model_cython import AModel


class ProviderConfigRecord(AModel):
    """Provider 完整配置。"""

    id: str = ""
    provider: str = ""
    display_name: str = ""
    auth_token_masked: str = ""
    base_url: str = ""
    model: str = ""
    enabled: bool = True
    is_default: bool = False
    usage_count: int = 0
    created_at: datetime | None = None
    updated_at: datetime | None = None

    model_config = {"from_attributes": True}


class ProviderOption(AModel):
    """供下拉框使用的 Provider 选项。"""

    provider: str = ""
    display_name: str = ""
    is_default: bool = False


class CreateProviderConfigRequest(AModel):
    """创建 Provider 配置请求。"""

    provider: str = Field(..., description="provider 标识")
    display_name: str = Field(..., description="展示名称")
    auth_token: str = Field(..., description="鉴权 Token")
    base_url: str = Field(..., description="API Base URL")
    model: str = Field(..., description="默认模型")
    enabled: bool = Field(default=True, description="是否启用")
    is_default: bool = Field(default=False, description="是否设为默认")


class UpdateProviderConfigRequest(AModel):
    """更新 Provider 配置请求。"""

    display_name: str = Field(..., description="展示名称")
    auth_token: str | None = Field(default=None, description="鉴权 Token")
    base_url: str = Field(..., description="API Base URL")
    model: str = Field(..., description="默认模型")
    enabled: bool = Field(default=True, description="是否启用")
    is_default: bool = Field(default=False, description="是否设为默认")


class ProviderOptionsResponse(AModel):
    """Provider 选项响应。"""

    default_provider: str | None = None
    items: list[ProviderOption] = Field(default_factory=list)


class ProviderRuntimeConfig(AModel):
    """Provider 运行时配置。"""

    provider: str = ""
    display_name: str = ""
    auth_token: str = ""
    base_url: str = ""
    model: str = ""
