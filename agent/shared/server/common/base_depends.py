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

from agent.core.config import settings
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
        except Exception:
            pass
    elif request.method == "GET":
        request_id = request.query_params.get("request_id", f"{settings.PROJECT_NAME}-{worker.get_id()}")
        request.state.request_id = request_id
    else:
        request_id = f"{settings.PROJECT_NAME}-{worker.get_id()}"
        request.state.request_id = request_id
