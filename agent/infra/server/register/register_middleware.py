# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：register_middleware
# @Date   ：2024/1/22 16:16
# @Author ：leemysw

# 2024/1/22 16:16   Create
# =====================================================

import json
from typing import Iterable

from fastapi import FastAPI
from starlette.middleware.cors import CORSMiddleware

from agent.config.config import settings


def _normalize_origin_items(items: Iterable[object]) -> list[str]:
    """标准化 origin 列表。"""
    return [str(item).strip() for item in items if str(item).strip()]


def _resolve_cors_origins() -> list[str]:
    """解析 CORS 白名单，兼容逗号分隔与 JSON 数组。"""
    raw_origins = settings.BACKEND_CORS_ORIGINS
    if isinstance(raw_origins, str):
        normalized_raw_origins = raw_origins.strip()
        if not normalized_raw_origins:
            parsed_origins = []
        elif normalized_raw_origins.startswith("["):
            try:
                parsed_origins = json.loads(normalized_raw_origins)
            except json.JSONDecodeError:
                parsed_origins = [normalized_raw_origins]
        else:
            parsed_origins = normalized_raw_origins.split(",")
    else:
        parsed_origins = raw_origins

    normalized_origins = _normalize_origin_items(parsed_origins)
    if "*" in normalized_origins:
        raise RuntimeError("BACKEND_CORS_ORIGINS 不支持使用 '*'，请显式配置允许访问的来源")
    return normalized_origins


def register_middleware(app: FastAPI) -> None:
    """
    支持跨域
    :param app:
    :return:
    """
    allow_origins = _resolve_cors_origins()
    app.add_middleware(
        CORSMiddleware,
        allow_origins=allow_origins,
        allow_credentials=True,
        # 设置允许跨域的http方法，比如 get、post、put等。
        allow_methods=["*"],
        # 允许跨域的headers，可以用来鉴别来源等作用。
        allow_headers=["*"]
    )
