# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main.py
# @Date   ：2024/3/19 17:12
# @Author ：leemysw

# 2024/3/19 17:12   Create
# =====================================================

import warnings

warnings.filterwarnings("ignore", category=RuntimeWarning)

import asyncio
import sys

import uvicorn

from agent.config.config import settings
from agent.infra.server.launcher import serve_http
from agent.utils import utils
from agent.utils.logger import logger


def create_app():
    from agent.app import app
    return app


app = create_app()


@app.get('/health')
async def root_health():
    from agent.infra.server.common import resp

    return resp.ok(response=resp.Resp(data={"status": "ok"}))


if __name__ == '__main__':
    utils.print_info(settings, logger)

    is_debugger_attached = sys.gettrace() is not None
    reload_enabled = False if settings.WORKERS != 1 or is_debugger_attached else settings.DEBUG
    app_reference = "main:app" if settings.DEBUG else app

    kwargs = {
        "app": app_reference,
        "host": settings.HOST,
        "port": settings.PORT,
        "reload": reload_enabled,
        "workers": settings.WORKERS,
        "lifespan": 'on',
        "ws": "websockets-sansio",
        "log_config": utils.set_uvicorn_logger(settings.LOGGER_FORMAT),
    }

    if reload_enabled or settings.WORKERS > 1:
        uvicorn.run(**kwargs)
    else:
        single_process_kwargs = dict(kwargs)
        single_process_kwargs.pop("app", None)
        single_process_kwargs.pop("reload", None)
        single_process_kwargs.pop("workers", None)
        # 中文注释：单进程路径直接走 server.serve()，避免调试器改写 asyncio.run 后与 uvicorn 的
        # loop_factory 调用发生签名冲突。
        asyncio.run(serve_http(app, **single_process_kwargs))
