# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_runtime.py
# @Date   ：2026/04/02 17:18
# @Author ：leemysw
# 2026/04/02 17:18   Create
# =====================================================

"""运行时配置接口。"""

from fastapi import APIRouter

from agent.config.config import settings
from agent.infra.server.common import resp

router = APIRouter(tags=["runtime"])


@router.get('/health')
async def health(self):
    return resp.ok(response=resp.Resp(data={"status": "ok"}))


@router.get("/runtime/options")
async def get_runtime_options():
    """返回前端启动所需的唯一运行时配置。"""
    return resp.ok(
        resp.Resp(
            data={
                "default_agent_id": settings.DEFAULT_AGENT_ID,
            }
        )
    )
