# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：register_middleware
# @Date   ：2024/1/22 16:16
# @Author ：leemysw

# 2024/1/22 16:16   Create
# =====================================================

import json

from fastapi import FastAPI
from starlette.middleware.cors import CORSMiddleware

from agent.config.config import settings


def _resolve_cors_origins() -> tuple[list[str], str | None]:
    """解析 CORS 白名单，兼容逗号分隔与 JSON 数组。"""
    raw_origins = settings.BACKEND_CORS_ORIGINS
    if isinstance(raw_origins, str):
        normalized_raw_origins = raw_origins.strip()
        if normalized_raw_origins.startswith("["):
            try:
                parsed_origins = json.loads(normalized_raw_origins)
            except json.JSONDecodeError:
                parsed_origins = [normalized_raw_origins]
        else:
            parsed_origins = [item.strip() for item in normalized_raw_origins.split(",") if item.strip()]
    else:
        parsed_origins = [str(item).strip() for item in raw_origins if str(item).strip()]

    if "*" in parsed_origins:
        # 中文注释：带 Cookie 的浏览器请求不能返回 Access-Control-Allow-Origin:*，
        # 这里改成反射请求源，既兼容开发环境，也避免登录态直接失效。
        return [], r"https?://.*"
    return parsed_origins, None


def register_middleware(app: FastAPI) -> None:
    """
    支持跨域
    :param app:
    :return:
    """
    allow_origins, allow_origin_regex = _resolve_cors_origins()
    app.add_middleware(
        CORSMiddleware,
        allow_origins=allow_origins,
        allow_origin_regex=allow_origin_regex,
        allow_credentials=True,
        # 设置允许跨域的http方法，比如 get、post、put等。
        allow_methods=["*"],
        # 允许跨域的headers，可以用来鉴别来源等作用。
        allow_headers=["*"]
    )
