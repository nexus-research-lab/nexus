# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：register_static
# @Date   ：2024/1/22 16:18
# @Author ：leemysw

# 2024/1/22 16:18   Create
# =====================================================


from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles

from agent.utils.utils import sources_path


def register_static_file(app: FastAPI) -> None:
    """
    :param app:
    :return:
    """
    static_path = sources_path("assets")
    app.mount("/asr", StaticFiles(directory=static_path, html=True), name="assets")
