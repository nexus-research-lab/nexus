# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：connector_service.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""Connector 服务 —— 应用授权的业务编排层。"""

from __future__ import annotations

import json
import logging
import secrets
from datetime import datetime
from typing import Optional

from agent.config.config import settings
from agent.schema.model_connector import (
    CompleteConnectorOAuthRequest,
    ConnectorDetail,
    ConnectorInfo,
)
from agent.service.capability.connectors.connector_catalog import (
    CatalogEntry,
    ConnectorCatalog,
)
from agent.service.capability.connectors.connector_oauth_service import connector_oauth_service
from agent.storage.connector_repository import connector_repository

logger = logging.getLogger(__name__)


class ConnectorService:
    """连接器服务 —— 查询目录、授权连接、断开连接。"""

    # ── 查询 ──────────────────────────────────────────

    async def list_connectors(
        self,
        q: Optional[str] = None,
        category: Optional[str] = None,
        status: Optional[str] = None,
    ) -> list[ConnectorInfo]:
        """列出所有连接器，附带用户连接状态。"""
        # 从目录获取条目
        entries = ConnectorCatalog.search(q) if q else ConnectorCatalog.list_all()
        # 可选：按类别过滤
        if category:
            entries = [e for e in entries if e.category == category]
        # 可选：按状态过滤
        if status:
            entries = [e for e in entries if e.status == status]
        # 获取用户连接状态
        conn_states = await connector_repository.get_connection_states()
        # 组装结果
        results: list[ConnectorInfo] = []
        for entry in entries:
            state = conn_states.get(entry.connector_id, "disconnected")
            results.append(self._to_info(entry, state))
        return results

    async def get_connector_detail(
        self, connector_id: str
    ) -> Optional[ConnectorDetail]:
        """获取连接器详情。"""
        entry = ConnectorCatalog.get(connector_id)
        if not entry:
            return None
        conn_states = await connector_repository.get_connection_states()
        state = conn_states.get(connector_id, "disconnected")
        return self._to_detail(entry, state)

    async def get_connected_count(self) -> int:
        """获取已连接数量。"""
        return await connector_repository.get_connected_count()

    async def get_categories(self) -> dict[str, str]:
        """获取类别映射。"""
        return ConnectorCatalog.list_categories()

    # ── 连接 / 断开 ──────────────────────────────────

    async def get_auth_url(self, connector_id: str) -> dict:
        """生成 OAuth 授权跳转地址。"""
        return await self.get_auth_url_with_redirect(connector_id, None)

    async def get_auth_url_with_redirect(
        self,
        connector_id: str,
        redirect_uri: str | None,
    ) -> dict:
        """生成 OAuth 授权跳转地址，优先使用运行时回调地址。"""
        entry = ConnectorCatalog.get(connector_id)
        if not entry:
            raise ValueError(f"未知连接器: {connector_id}")
        if entry.status == "coming_soon":
            raise ValueError(f"连接器 {entry.title} 即将推出，暂不可用")
        config_state = connector_oauth_service.get_config_state(entry)
        if not config_state.is_configured:
            raise ValueError(f"连接器 {entry.title} {config_state.error}")
        resolved_redirect_uri = connector_oauth_service.resolve_redirect_uri(redirect_uri)
        # 生成随机 state 防止 CSRF
        state = secrets.token_urlsafe(32)
        # 中文注释：发起重新授权时只刷新临时 state，不能破坏已有连接与 token。
        await connector_repository.set_oauth_state(
            connector_id=connector_id,
            oauth_state=state,
            oauth_state_expires_at=connector_oauth_service.build_state_expiry(),
        )
        auth_redirect = connector_oauth_service.build_auth_url(entry, state, resolved_redirect_uri)
        return {"auth_url": auth_redirect, "state": state}

    async def complete_oauth_callback(self, body: CompleteConnectorOAuthRequest) -> ConnectorInfo:
        """完成 OAuth 回调，校验 state 并交换 access token。"""
        connection = await connector_repository.get_connection_by_oauth_state(body.state)
        if not connection:
            raise ValueError("OAuth state 无效或已过期")
        if connection.oauth_state_expires_at and connection.oauth_state_expires_at < datetime.now():
            raise ValueError("OAuth state 已过期，请重新发起授权")

        entry = ConnectorCatalog.get(connection.connector_id)
        if not entry:
            raise ValueError(f"未知连接器: {connection.connector_id}")

        resolved_redirect_uri = body.redirect_uri

        token_payload = await connector_oauth_service.exchange_code_for_token(
            entry,
            code=body.code,
            redirect_uri=resolved_redirect_uri,
        )
        # 中文注释：OAuth 完成后清空临时 state，只保留第三方返回的 token 结果。
        await connector_repository.connect(
            connector_id=connection.connector_id,
            credentials=json.dumps(token_payload, ensure_ascii=False),
            auth_type=entry.auth_type,
            state="connected",
            oauth_state=None,
            oauth_state_expires_at=None,
        )
        return self._to_info(entry, "connected")

    async def connect(
        self,
        connector_id: str,
        credentials: Optional[dict] = None,
    ) -> ConnectorInfo:
        """授权连接某个应用。"""
        entry = ConnectorCatalog.get(connector_id)
        if not entry:
            raise ValueError(f"未知连接器: {connector_id}")
        if entry.status == "coming_soon":
            raise ValueError(f"连接器 {entry.title} 即将推出，暂不可用")
        if entry.auth_type == "oauth2":
            raise ValueError(f"连接器 {entry.title} 需要先完成 OAuth 授权回调，不能直接连接")
        # 序列化凭证存储
        cred_str = json.dumps(credentials) if credentials else ""
        await connector_repository.connect(
            connector_id=connector_id,
            credentials=cred_str,
            auth_type=entry.auth_type,
            state="connected",
            oauth_state=None,
            oauth_state_expires_at=None,
        )
        logger.info("连接器 %s 已连接", connector_id)
        return self._to_info(entry, "connected")

    async def disconnect(self, connector_id: str) -> ConnectorInfo:
        """断开某个应用的连接。"""
        entry = ConnectorCatalog.get(connector_id)
        if not entry:
            raise ValueError(f"未知连接器: {connector_id}")
        await connector_repository.disconnect(connector_id)
        logger.info("连接器 %s 已断开", connector_id)
        return self._to_info(entry, "disconnected")

    async def build_runtime_mcp_servers(self) -> dict:
        """把已连接且已配置 MCP 地址的连接器转换为 runtime mcp_servers。"""
        results: dict[str, dict] = {}
        states = await connector_repository.get_connection_states()
        for connector_id, state in states.items():
            if state != "connected":
                continue
            entry = ConnectorCatalog.get(connector_id)
            if not entry or not entry.mcp_server_url:
                continue
            connection = await connector_repository.get_connection(connector_id)
            if not connection:
                continue
            headers = {}
            if connection.credentials:
                try:
                    payload = json.loads(connection.credentials)
                except json.JSONDecodeError:
                    payload = {}
                token = payload.get("access_token") if isinstance(payload, dict) else None
                if token:
                    headers["Authorization"] = f"Bearer {token}"

            results[connector_id] = {
                "transport": "streamable_http",
                "url": entry.mcp_server_url,
                **({"headers": headers} if headers else {}),
            }
        return results

    # ── 内部转换 ──────────────────────────────────────

    @staticmethod
    def _to_info(entry: CatalogEntry, state: str) -> ConnectorInfo:
        config_state = connector_oauth_service.get_config_state(entry)
        return ConnectorInfo(
            connector_id=entry.connector_id,
            name=entry.name,
            title=entry.title,
            description=entry.description,
            icon=entry.icon,
            category=entry.category,
            auth_type=entry.auth_type,
            status=entry.status,
            connection_state=state,
            is_configured=config_state.is_configured,
            config_error=config_state.error,
        )

    @staticmethod
    def _to_detail(entry: CatalogEntry, state: str) -> ConnectorDetail:
        config_state = connector_oauth_service.get_config_state(entry)
        return ConnectorDetail(
            connector_id=entry.connector_id,
            name=entry.name,
            title=entry.title,
            description=entry.description,
            icon=entry.icon,
            category=entry.category,
            auth_type=entry.auth_type,
            status=entry.status,
            connection_state=state,
            auth_url=entry.auth_url,
            token_url=entry.token_url,
            scopes=list(entry.scopes),
            mcp_server_url=entry.mcp_server_url,
            docs_url=entry.docs_url,
            features=list(entry.features),
            is_configured=config_state.is_configured,
            config_error=config_state.error,
        )


# 全局单例
connector_service = ConnectorService()
