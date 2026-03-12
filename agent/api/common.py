# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api
# @Date   ：2025/1/4 13:09
# @Author ：leemysw

# 2025/1/4 13:09   Create
# =====================================================

from fastapi import APIRouter

from agent.api.handler import BaseHandler
from agent.shared.server.common import cbv
from agent.shared.server.common import resp

# import signal

common_router = APIRouter()


@cbv(common_router)
class Api(BaseHandler):

    @common_router.get('/health')
    async def health(self):
        return resp.ok(response=resp.Resp(data={"status": "ok"}))
