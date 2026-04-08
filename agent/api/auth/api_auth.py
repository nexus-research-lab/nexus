# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_auth.py
# @Date   ：2026/4/7 18:24
# @Author ：leemysw
# 2026/4/7 18:24   Create
# =====================================================

"""认证 API。"""

from fastapi import APIRouter, HTTPException, Request

from agent.infra.schemas.model_cython import AModel
from agent.infra.server.common import resp
from agent.service.auth import auth_service

router = APIRouter(tags=["auth"])


class LoginRequest(AModel):
    """登录请求。"""

    username: str = ""
    password: str


@router.get("/auth/status")
async def get_auth_status(request: Request):
    """返回当前认证状态。"""
    return resp.ok(resp.Resp(data=await auth_service.build_status_payload(request)))


@router.post("/auth/login")
async def login(request: Request, payload: LoginRequest):
    """执行密码登录并签发 Cookie。"""
    try:
        username = auth_service.verify_login(
            username=payload.username,
            password=payload.password,
        )
    except RuntimeError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=401, detail=str(exc)) from exc

    await auth_service.clear_login_session(
        request.cookies.get(auth_service.get_cookie_name())
    )
    session_token = await auth_service.create_login_session(username)

    response = resp.ok(
        resp.Resp(data=auth_service.build_auth_payload(authenticated=True, username=username))
    )
    response.set_cookie(
        key=auth_service.get_cookie_name(),
        value=session_token,
        max_age=auth_service.get_session_ttl_seconds(),
        httponly=True,
        secure=auth_service.get_cookie_secure(),
        samesite=auth_service.get_cookie_samesite(),
        path=auth_service.get_cookie_path(),
    )
    return response


@router.post("/auth/logout")
async def logout(request: Request):
    """清空登录 Cookie。"""
    await auth_service.clear_login_session(
        request.cookies.get(auth_service.get_cookie_name())
    )
    response = resp.ok(
        resp.Resp(
            data=auth_service.build_auth_payload(
                authenticated=not auth_service.is_auth_required(),
                username=None,
            )
        )
    )
    response.delete_cookie(
        key=auth_service.get_cookie_name(),
        path=auth_service.get_cookie_path(),
    )
    return response
