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

import uvicorn

from agent.config.config import settings
from agent.utils import utils
from agent.utils.logger import logger


def create_app():
    from agent.app import app
    return app


app = create_app()

if __name__ == '__main__':
    utils.print_info(settings, logger)

    if settings.DEBUG:
        entrypoint = "main:app"
    else:
        entrypoint = app

    kwargs = {
        "app": entrypoint,
        "host": settings.HOST,
        "port": settings.PORT,
        "reload": False if settings.WORKERS != 1 else settings.DEBUG,
        "workers": settings.WORKERS,
        "lifespan": 'on',
        "ws": "websockets-sansio",
        "log_config": utils.set_uvicorn_logger(settings.LOGGER_FORMAT),
    }

    uvicorn.run(**kwargs)
