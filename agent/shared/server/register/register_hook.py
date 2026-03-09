# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：register_hook
# @Date   ：2024/1/22 16:13
# @Author ：leemysw

# 2024/1/22 16:13   Create
# =====================================================

import time
from datetime import datetime

from fastapi import FastAPI, Request

from agent.utils.logger import logger


def register_hook(app: FastAPI) -> None:
    """
    请求响应拦截 hook
    https://fastapi.tiangolo.com/tutorial/middleware/
    :param app:
    :return:
    """

    @app.middleware("http")
    async def add_process_time_header(request: Request, call_next):
        start_time = time.time()
        x_request_start = datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")
        request.state.start_time = start_time

        logger.info(f"request start -- {datetime.now().strftime('%Y-%m-%d %H:%M:%S.%f')}")
        response = await call_next(request)
        logger.info(f"request end -- {datetime.now().strftime('%Y-%m-%d %H:%M:%S.%f')}")


        process_time = time.time() - start_time
        request_id = request.state.request_id if hasattr(request.state, "request_id") else "unknown"
        logger.info(f"request_id:{request_id} -> process_time:{process_time}")
        response.headers["X-Request-Id"] = str(request_id)
        response.headers["X-Request-Time"] = x_request_start
        response.headers["X-Response-Time"] = datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")
        response.headers["X-Process-Time"] = str(process_time)
        return response
