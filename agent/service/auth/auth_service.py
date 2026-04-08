# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：auth_service.py
# @Date   ：2026/4/7 18:24
# @Author ：leemysw
# 2026/4/7 18:24   Create
# =====================================================

"""统一处理密码登录、Cookie 会话与 Bearer Token 兼容鉴权。"""

from __future__ import annotations

import base64
import binascii
import hashlib
import hmac
import json
import time
from typing import Optional

from fastapi import Request, WebSocket

from agent.config.config import settings


def _encode_payload(raw_bytes: bytes) -> str:
    """把字节编码为 URL 安全字符串。"""
    return base64.urlsafe_b64encode(raw_bytes).decode("ascii").rstrip("=")


def _decode_payload(raw_value: str) -> bytes:
    """解码 URL 安全字符串。"""
    padding = "=" * (-len(raw_value) % 4)
    return base64.urlsafe_b64decode(f"{raw_value}{padding}".encode("ascii"))


class AuthService:
    """登录鉴权服务。"""

    _TOKEN_VERSION = "v1"

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

    def build_login_cookie(self, username: str) -> str:
        """为登录用户签发会话 Cookie。"""
        now = int(time.time())
        payload = {
            "username": username,
            "iat": now,
            "exp": now + self.get_session_ttl_seconds(),
        }
        payload_bytes = json.dumps(
            payload,
            ensure_ascii=False,
            separators=(",", ":"),
            sort_keys=True,
        ).encode("utf-8")
        encoded_payload = _encode_payload(payload_bytes)
        signature = hmac.new(
            self._get_session_secret().encode("utf-8"),
            encoded_payload.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()
        return f"{self._TOKEN_VERSION}.{encoded_payload}.{signature}"

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

    def resolve_http_identity(self, request: Request) -> Optional[str]:
        """解析 HTTP 请求身份。"""
        session_cookie = request.cookies.get(self.get_cookie_name())
        authorization = request.headers.get("Authorization")
        return self._resolve_identity(
            session_cookie=session_cookie,
            authorization=authorization,
        )

    def resolve_websocket_identity(self, websocket: WebSocket) -> Optional[str]:
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
        return self._resolve_identity(
            session_cookie=session_cookie,
            authorization=authorization,
            query_token=query_token,
        )

    def build_status_payload(self, request: Request) -> dict:
        """构建前端可消费的登录状态。"""
        username = self.resolve_http_identity(request)
        auth_required = self.is_auth_required()
        return {
            "auth_required": auth_required,
            "password_login_enabled": self.is_password_login_enabled(),
            "authenticated": True if not auth_required else username is not None,
            "username": username,
        }

    def _resolve_identity(
        self,
        session_cookie: Optional[str],
        authorization: Optional[str],
        query_token: Optional[str] = None,
    ) -> Optional[str]:
        """按优先级解析 Cookie 会话与 Bearer Token。"""
        if self.is_password_login_enabled() and session_cookie:
            username = self._verify_session_cookie(session_cookie)
            if username:
                return username

        access_token = self.get_access_token()
        if not access_token:
            return None

        provided_token = self._extract_bearer_token(authorization) or (query_token or "").strip()
        if provided_token and hmac.compare_digest(provided_token, access_token):
            return "access-token"
        return None

    def _verify_session_cookie(self, raw_cookie: str) -> Optional[str]:
        """校验会话 Cookie。"""
        try:
            version, encoded_payload, provided_signature = raw_cookie.split(".", 2)
        except ValueError:
            return None

        if version != self._TOKEN_VERSION:
            return None

        expected_signature = hmac.new(
            self._get_session_secret().encode("utf-8"),
            encoded_payload.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()
        if not hmac.compare_digest(provided_signature, expected_signature):
            return None

        try:
            payload = json.loads(_decode_payload(encoded_payload).decode("utf-8"))
        except (ValueError, UnicodeDecodeError, binascii.Error, json.JSONDecodeError):
            return None

        username = payload.get("username")
        expires_at = payload.get("exp")
        if not isinstance(username, str) or not isinstance(expires_at, int):
            return None
        if expires_at <= int(time.time()):
            return None
        if username != self.get_login_username():
            return None
        return username

    def _get_session_secret(self) -> str:
        """返回签名密钥。"""
        configured_secret = (settings.AUTH_SESSION_SECRET or "").strip()
        if configured_secret:
            return configured_secret
        return self.get_login_password()

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
