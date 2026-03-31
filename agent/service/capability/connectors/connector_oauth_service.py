# !/usr/bin/env python
# -*- coding: utf-8 -*-

"""Connector OAuth 辅助服务。"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
import json
from typing import Any
from urllib.parse import urlencode

import aiohttp

from agent.config.config import settings
from agent.service.capability.connectors.connector_catalog import CatalogEntry


@dataclass(frozen=True)
class ConnectorOauthConfigState:
    """OAuth 配置检查结果。"""

    is_configured: bool
    error: str | None = None


class ConnectorOauthService:
    """处理连接器 OAuth 配置检查、授权地址生成与 token 交换。"""

    STATE_TTL_MINUTES = 10
    UNSUPPORTED_CONNECTORS = {
        "x-twitter": "当前版本尚未实现 PKCE 流程",
        "shopify": "当前版本尚未实现店铺级 OAuth 流程",
    }

    def get_client_id(self, connector_id: str) -> str:
        config_map = {
            "github": settings.CONNECTOR_GITHUB_CLIENT_ID,
            "gmail": settings.CONNECTOR_GOOGLE_CLIENT_ID,
            "linkedin": settings.CONNECTOR_LINKEDIN_CLIENT_ID,
            "x-twitter": settings.CONNECTOR_TWITTER_CLIENT_ID,
            "instagram": settings.CONNECTOR_INSTAGRAM_CLIENT_ID,
            "shopify": settings.CONNECTOR_SHOPIFY_CLIENT_ID,
        }
        return config_map.get(connector_id, "")

    def get_client_secret(self, connector_id: str) -> str:
        config_map = {
            "github": settings.CONNECTOR_GITHUB_CLIENT_SECRET,
            "gmail": settings.CONNECTOR_GOOGLE_CLIENT_SECRET,
            "linkedin": settings.CONNECTOR_LINKEDIN_CLIENT_SECRET,
            "x-twitter": settings.CONNECTOR_TWITTER_CLIENT_SECRET,
            "instagram": settings.CONNECTOR_INSTAGRAM_CLIENT_SECRET,
            "shopify": settings.CONNECTOR_SHOPIFY_CLIENT_SECRET,
        }
        return config_map.get(connector_id, "")

    def get_config_state(self, entry: CatalogEntry) -> ConnectorOauthConfigState:
        """检查 OAuth 连接器是否满足最小可用配置。"""
        if entry.auth_type != "oauth2":
            return ConnectorOauthConfigState(True)
        if entry.connector_id in self.UNSUPPORTED_CONNECTORS:
            return ConnectorOauthConfigState(False, self.UNSUPPORTED_CONNECTORS[entry.connector_id])
        if not entry.auth_url or not entry.token_url:
            return ConnectorOauthConfigState(False, "连接器缺少 OAuth 端点配置")
        if not self.get_client_id(entry.connector_id):
            return ConnectorOauthConfigState(False, "缺少 OAuth Client ID")
        if not self.get_client_secret(entry.connector_id):
            return ConnectorOauthConfigState(False, "缺少 OAuth Client Secret")
        return ConnectorOauthConfigState(True)

    def resolve_redirect_uri(self, redirect_uri: str | None = None) -> str:
        """优先使用运行时回调地址，未提供时回退到配置值。"""
        resolved = (redirect_uri or settings.CONNECTOR_OAUTH_REDIRECT_URI or "").strip()
        if not resolved:
            raise ValueError("缺少 OAuth redirect_uri")
        return resolved

    def build_auth_url(self, entry: CatalogEntry, state: str, redirect_uri: str | None = None) -> str:
        """拼接标准 authorization code 授权地址。"""
        params = {
            "response_type": "code",
            "client_id": self.get_client_id(entry.connector_id),
            "redirect_uri": self.resolve_redirect_uri(redirect_uri),
            "state": state,
        }
        if entry.scopes:
            params["scope"] = " ".join(entry.scopes)
        return f"{entry.auth_url}?{urlencode(params)}"

    @staticmethod
    def dump_pending_payload(redirect_uri: str | None = None) -> str:
        """保存 OAuth 发起阶段的临时信息。"""
        if not redirect_uri:
            return ""
        return json.dumps({"pending_redirect_uri": redirect_uri}, ensure_ascii=False)

    @staticmethod
    def load_pending_redirect_uri(credentials: str | None) -> str | None:
        """读取 OAuth 发起阶段暂存的 redirect_uri。"""
        if not credentials:
            return None
        try:
            payload = json.loads(credentials)
        except json.JSONDecodeError:
            return None
        if not isinstance(payload, dict):
            return None
        value = payload.get("pending_redirect_uri")
        return value if isinstance(value, str) and value.strip() else None

    def build_state_expiry(self) -> datetime:
        return datetime.now() + timedelta(minutes=self.STATE_TTL_MINUTES)

    async def exchange_code_for_token(
        self,
        entry: CatalogEntry,
        code: str,
        redirect_uri: str | None = None,
    ) -> dict[str, Any]:
        """用授权码向第三方换取 token。"""
        payload = {
            "client_id": self.get_client_id(entry.connector_id),
            "client_secret": self.get_client_secret(entry.connector_id),
            "code": code,
            "redirect_uri": self.resolve_redirect_uri(redirect_uri),
        }

        # 中文注释：GitHub 返回 JSON 需要显式 Accept 头，
        # 其他标准 OAuth 提供方即使忽略该头也通常能正常返回。
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/x-www-form-urlencoded",
        }
        async with aiohttp.ClientSession() as session:
            async with session.post(entry.token_url or "", data=payload, headers=headers) as response:
                if response.status >= 400:
                    raise ValueError(f"Token 交换失败: HTTP {response.status}")
                data = await response.json(content_type=None)

        if not isinstance(data, dict) or not data.get("access_token"):
            error_message = ""
            if isinstance(data, dict):
                error_message = str(data.get("error_description") or data.get("error") or "")
            raise ValueError(error_message or "第三方未返回 access_token")
        return data


connector_oauth_service = ConnectorOauthService()
