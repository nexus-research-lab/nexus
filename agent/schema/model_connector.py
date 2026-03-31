# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_connector.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""Connector（应用授权）数据模型。"""

from __future__ import annotations

from typing import Literal, Optional

from pydantic import Field

from agent.infra.schemas.model_cython import AModel

# 授权方式
ConnectorAuthType = Literal["oauth2", "api_key", "token", "none"]
# 连接状态
ConnectorStatus = Literal["available", "coming_soon"]
# 用户连接状态
ConnectionState = Literal["connected", "disconnected", "expired"]


class ConnectorInfo(AModel):
    """连接器列表项 —— 用于卡片展示。"""

    connector_id: str = ""
    name: str = ""
    title: str = ""
    description: str = ""
    icon: str = ""
    category: str = ""
    auth_type: ConnectorAuthType = "oauth2"
    status: ConnectorStatus = "available"
    # 用户维度的连接状态
    connection_state: ConnectionState = "disconnected"
    connected_at: Optional[str] = None
    is_configured: bool = True
    config_error: Optional[str] = None


class ConnectorDetail(ConnectorInfo):
    """连接器详情 —— 包含授权配置和 MCP 信息。"""

    auth_url: Optional[str] = None
    token_url: Optional[str] = None
    scopes: list[str] = Field(default_factory=list)
    mcp_server_url: Optional[str] = None
    docs_url: Optional[str] = None
    features: list[str] = Field(default_factory=list)


class ConnectRequest(AModel):
    """发起连接请求。"""

    connector_id: str = ""
    # OAuth 回调拿到的 code，或用户手动输入的 api_key / token
    auth_code: Optional[str] = None
    api_key: Optional[str] = None
    token: Optional[str] = None
    redirect_uri: Optional[str] = None
    state: Optional[str] = None


class DisconnectRequest(AModel):
    """断开连接请求。"""

    connector_id: str = ""


class ConnectorAuthUrlResponse(AModel):
    """OAuth 授权 URL 响应。"""

    auth_url: str = ""
    state: str = ""


class CompleteConnectorOAuthRequest(AModel):
    """完成 OAuth 回调请求。"""

    code: str = Field(..., description="OAuth 回调返回的授权码")
    state: str = Field(..., description="OAuth state")
    redirect_uri: Optional[str] = Field(default=None, description="实际回调地址")
