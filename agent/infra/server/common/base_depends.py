# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：register_hoook
# @Date   ：2024/1/22 16:13
# @Author ：leemysw

# 2024/1/22 16:13   Create
# =====================================================

import json

from fastapi import Request

from agent.config.config import settings
from agent.infra.server.common.base_exception import AuthenticationException, TokenAuthException
from agent.service.auth import auth_service
from agent.utils.snowflake import worker


async def extract_request_id(request: Request = None):
    if request is None:
        return

    content_type = request.headers.get("content-type", "")

    if request.method == "POST" and content_type == "application/json":
        body_bytes = await request.body()
        try:
            body_data = json.loads(body_bytes)
            request_id = body_data.get("request_id", f"{settings.PROJECT_NAME}-{worker.get_id()}")
            request.state.request_id = request_id
        except Exception as e:
            # JSON 解析失败时使用默认 request_id
            request.state.request_id = f"{settings.PROJECT_NAME}-{worker.get_id()}"
    elif request.method == "GET":
        request_id = request.query_params.get("request_id", f"{settings.PROJECT_NAME}-{worker.get_id()}")
        request.state.request_id = request_id
    else:
        request_id = f"{settings.PROJECT_NAME}-{worker.get_id()}"
        request.state.request_id = request_id


async def require_http_auth(request: Request):
    """统一校验 HTTP 登录态。"""
    if not auth_service.is_auth_required():
        return

    identity = auth_service.resolve_http_identity(request)
    if identity:
        request.state.auth_user = identity
        return

    if auth_service.is_password_login_enabled():
        raise AuthenticationException("未登录或登录状态已过期")

    authorization = request.headers.get("Authorization")
    if not authorization:
        raise TokenAuthException("Authorization header is missing")
    raise TokenAuthException("Token is invalid")
