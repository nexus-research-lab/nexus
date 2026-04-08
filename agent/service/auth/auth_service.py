# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：auth_service.py
# @Date   ：2026/4/7 18:24
# @Author ：leemysw
# 2026/4/7 18:24   Create
# =====================================================

"""统一处理密码登录、服务端会话与 Bearer Token 兼容鉴权。"""

from __future__ import annotations

import hashlib
import hmac
import secrets
from datetime import datetime, timedelta
from typing import Optional

from fastapi import Request, WebSocket

from agent.config.config import settings
from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.auth_session_sql_repository import (
    AuthSessionSqlRepository,
)
from agent.schema.model_auth import AuthSessionRecord
from agent.utils.snowflake import worker


class AuthService:
    """登录鉴权服务。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    def is_auth_required(self) -> bool:
        """是否启用了任意鉴权方案。"""
        return self.is_password_login_enabled() or bool(self.get_access_token())

    def is_password_login_enabled(self) -> bool:
        """是否启用密码登录。"""
        return bool(self.get_login_password())

    @staticmethod
    def get_login_username() -> str:
        """返回登录用户名。"""
        configured_username = (settings.AUTH_LOGIN_USERNAME or "").strip()
        return configured_username or "admin"

    @staticmethod
    def get_login_password() -> str:
        """返回登录密码。"""
        return settings.AUTH_LOGIN_PASSWORD or ""

    @staticmethod
    def get_access_token() -> str:
        """返回兼容 Bearer Token。"""
        return (settings.ACCESS_TOKEN or "").strip()

    @staticmethod
    def get_cookie_name() -> str:
        """返回认证 Cookie 名称。"""
        configured_name = (settings.AUTH_SESSION_COOKIE_NAME or "").strip()
        return configured_name or "nexus_session"

    @staticmethod
    def get_cookie_path() -> str:
        """认证 Cookie 仅作用于后端 API 前缀。"""
        normalized_path = (settings.API_PREFIX or "/").strip()
        return normalized_path or "/"

    @staticmethod
    def get_cookie_samesite() -> str:
        """约束 Cookie 的 SameSite 配置。"""
        normalized_value = (settings.AUTH_COOKIE_SAMESITE or "lax").strip().lower()
        if normalized_value in {"lax", "strict", "none"}:
            return normalized_value
        return "lax"

    @staticmethod
    def get_cookie_secure() -> bool:
        """是否只允许 HTTPS 发送 Cookie。"""
        return bool(settings.AUTH_COOKIE_SECURE)

    @staticmethod
    def get_session_ttl_seconds() -> int:
        """返回会话有效期秒数。"""
        hours = max(int(settings.AUTH_SESSION_TTL_HOURS or 24), 1)
        return hours * 3600

    def verify_login(self, username: str, password: str) -> str:
        """校验登录凭据，成功时返回规范化用户名。"""
        if not self.is_password_login_enabled():
            raise RuntimeError("服务端未启用密码登录")

        normalized_username = (username or "").strip() or self.get_login_username()
        expected_username = self.get_login_username()
        expected_password = self.get_login_password()

        # 中文注释：用户名与密码都使用常量时间比较，避免把登录接口降级成可枚举探针。
        if not hmac.compare_digest(normalized_username, expected_username):
            raise ValueError("用户名或密码错误")
        if not hmac.compare_digest(password or "", expected_password):
            raise ValueError("用户名或密码错误")
        return normalized_username

    async def create_login_session(self, username: str) -> str:
        """创建服务端登录会话并返回原始会话令牌。"""
        now = datetime.now()
        token = secrets.token_urlsafe(32)
        token_hash = self._hash_session_token(token)
        expires_at = now + timedelta(seconds=self.get_session_ttl_seconds())
        record = AuthSessionRecord(
            id=str(worker.get_id()),
            session_token_hash=token_hash,
            username=username,
            expires_at=expires_at,
        )

        async with self._db.session() as session:
            repository = AuthSessionSqlRepository(session)
            await repository.delete_expired(now)
            await repository.create(record)
            await session.commit()

        return token

    async def clear_login_session(self, session_token: str | None) -> None:
        """撤销指定浏览器会话。"""
        normalized_token = (session_token or "").strip()
        if not normalized_token:
            return

        async with self._db.session() as session:
            repository = AuthSessionSqlRepository(session)
            deleted = await repository.delete_by_token_hash(
                self._hash_session_token(normalized_token)
            )
            if deleted:
                await session.commit()

    async def resolve_http_identity(self, request: Request) -> Optional[str]:
        """解析 HTTP 请求身份。"""
        session_cookie = request.cookies.get(self.get_cookie_name())
        authorization = request.headers.get("Authorization")
        return await self._resolve_identity(
            session_cookie=session_cookie,
            authorization=authorization,
        )

    async def resolve_websocket_identity(self, websocket: WebSocket) -> Optional[str]:
        """解析 WebSocket 连接身份。"""
        session_cookie = self._extract_cookie_value(
            websocket.headers.get("cookie"),
            self.get_cookie_name(),
        )
        authorization = websocket.headers.get("authorization")
        query_token = (
            websocket.query_params.get("access_token")
            or websocket.query_params.get("token")
        )
        return await self._resolve_identity(
            session_cookie=session_cookie,
            authorization=authorization,
            query_token=query_token,
        )

    async def build_status_payload(self, request: Request) -> dict:
        """构建前端可消费的登录状态。"""
        username = await self.resolve_http_identity(request)
        auth_required = self.is_auth_required()
        return self.build_auth_payload(
            authenticated=True if not auth_required else username is not None,
            username=username,
        )

    def build_auth_payload(self, authenticated: bool, username: str | None) -> dict:
        """构建统一认证状态响应。"""
        return {
            "auth_required": self.is_auth_required(),
            "password_login_enabled": self.is_password_login_enabled(),
            "authenticated": authenticated,
            "username": username,
        }

    async def _resolve_identity(
        self,
        session_cookie: Optional[str],
        authorization: Optional[str],
        query_token: Optional[str] = None,
    ) -> Optional[str]:
        """按优先级解析浏览器会话与 Bearer Token。"""
        if self.is_password_login_enabled() and session_cookie:
            username = await self._resolve_session_username(session_cookie)
            if username:
                return username

        access_token = self.get_access_token()
        if not access_token:
            return None

        provided_token = self._extract_bearer_token(authorization) or (query_token or "").strip()
        if provided_token and hmac.compare_digest(provided_token, access_token):
            return "access-token"
        return None

    async def _resolve_session_username(self, session_token: str) -> Optional[str]:
        """根据服务端会话令牌解析用户名。"""
        token_hash = self._hash_session_token(session_token)
        now = datetime.now()

        async with self._db.session() as session:
            repository = AuthSessionSqlRepository(session)
            record = await repository.get_by_token_hash(token_hash)
            if record is None:
                return None
            if record.expires_at <= now:
                await repository.delete_by_token_hash(token_hash)
                await session.commit()
                return None
            if record.username != self.get_login_username():
                return None
            return record.username

    @staticmethod
    def _hash_session_token(session_token: str) -> str:
        """对会话令牌做不可逆摘要，避免数据库保存明文凭据。"""
        return hashlib.sha256(session_token.encode("utf-8")).hexdigest()

    @staticmethod
    def _extract_bearer_token(authorization: Optional[str]) -> str:
        """从 Authorization 头提取 Bearer Token。"""
        raw_header = (authorization or "").strip()
        if not raw_header.lower().startswith("bearer "):
            return ""
        return raw_header[7:].strip()

    @staticmethod
    def _extract_cookie_value(raw_cookie_header: Optional[str], cookie_name: str) -> Optional[str]:
        """从 Cookie 头里提取指定字段。"""
        if not raw_cookie_header:
            return None

        for item in raw_cookie_header.split(";"):
            key, separator, value = item.strip().partition("=")
            if separator and key == cookie_name:
                return value.strip() or None
        return None


auth_service = AuthService()
