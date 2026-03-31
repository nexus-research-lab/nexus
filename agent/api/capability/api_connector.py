# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_connector.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""Connector 应用授权 API 路由。"""

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Query
from fastapi import status as http_status

from agent.infra.server.common import resp
from agent.schema.model_connector import (
    CompleteConnectorOAuthRequest,
    ConnectRequest,
    DisconnectRequest,
)
from agent.service.capability.connectors.connector_service import connector_service

router = APIRouter(prefix="/connectors", tags=["connectors"])


@router.get("")
async def list_connectors(
    q: Optional[str] = Query(None, description="搜索关键字"),
    category: Optional[str] = Query(None, description="类别过滤"),
    status: Optional[str] = Query(None, description="状态过滤"),
):
    """获取连接器列表（含用户连接状态）。"""
    items = await connector_service.list_connectors(q=q, category=category, status=status)
    return resp.ok(resp.Resp(data=[item.model_dump() for item in items]))


@router.get("/categories")
async def get_categories():
    """获取连接器类别列表。"""
    categories = await connector_service.get_categories()
    return resp.ok(resp.Resp(data=categories))


@router.get("/count")
async def get_connected_count():
    """获取已连接数量。"""
    count = await connector_service.get_connected_count()
    return resp.ok(resp.Resp(data={"count": count}))


@router.post("/oauth/callback")
async def complete_oauth_callback(body: CompleteConnectorOAuthRequest):
    """消费 OAuth 回调并完成 token 交换。"""
    try:
        info = await connector_service.complete_oauth_callback(body)
        return resp.ok(resp.Resp(data=info.model_dump()))
    except ValueError as e:
        response = resp.Resp(code="400", http_status=http_status.HTTP_400_BAD_REQUEST)
        response.set_detail(str(e))
        return resp.fail(response)


@router.get("/{connector_id}")
async def get_connector_detail(connector_id: str):
    """获取连接器详情。"""
    detail = await connector_service.get_connector_detail(connector_id)
    if not detail:
        return resp.fail(resp.Resp(message=f"未找到连接器: {connector_id}"))
    return resp.ok(resp.Resp(data=detail.model_dump()))


@router.get("/{connector_id}/auth-url")
async def get_auth_url(
    connector_id: str,
    redirect_uri: Optional[str] = Query(None, description="运行时 OAuth 回调地址"),
):
    """获取 OAuth 授权跳转 URL。"""
    try:
        result = await connector_service.get_auth_url_with_redirect(connector_id, redirect_uri)
        return resp.ok(resp.Resp(data=result))
    except ValueError as e:
        response = resp.Resp(code="400", http_status=http_status.HTTP_400_BAD_REQUEST)
        response.set_detail(str(e))
        return resp.fail(response)


@router.post("/{connector_id}/connect")
async def connect_connector(connector_id: str, body: ConnectRequest):
    """授权连接某个应用。"""
    # 从请求中提取凭证
    credentials = {}
    if body.auth_code:
        credentials["auth_code"] = body.auth_code
    if body.api_key:
        credentials["api_key"] = body.api_key
    if body.token:
        credentials["token"] = body.token
    if body.redirect_uri:
        credentials["redirect_uri"] = body.redirect_uri
    try:
        info = await connector_service.connect(connector_id, credentials or None)
        return resp.ok(resp.Resp(data=info.model_dump()))
    except ValueError as e:
        return resp.fail(resp.Resp(message=str(e)))


@router.post("/{connector_id}/disconnect")
async def disconnect_connector(connector_id: str):
    """断开某个应用的连接。"""
    try:
        info = await connector_service.disconnect(connector_id)
        return resp.ok(resp.Resp(data=info.model_dump()))
    except ValueError as e:
        return resp.fail(resp.Resp(message=str(e)))
