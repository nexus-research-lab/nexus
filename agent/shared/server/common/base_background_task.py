# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_background_task
# @Date   ：2025/5/9 19:47
# @Author ：leemysw

# 2025/5/9 19:47   Create
# =====================================================

import traceback
from typing import Any, Callable

from fastapi import Request
from starlette._utils import is_async_callable

from agent.utils.logger import logger
from agent.utils.constants import TermColors


def _log_bg_error(exc: Exception, func: Callable, request: Request, ):
    func_name = getattr(func, '__name__', str(func))
    error_msg = f"【后台任务】{exc}"

    logger.error(
        f"{TermColors.RED}Exception  : {error_msg}\n"
        f"====================后台任务错误 (自定义类)======================\n"
        f"RequestId  : {request.state.request_id}\n"
        f"TaskName   : {func_name}\n"
        f"Host       : {request.client.host}\n"
        f"URL        : {request.method} {request.url}\n"
        f"UserAgent  : {request.headers.get('user-agent')}\n\n"
        f"\n{traceback.format_exc()}"
        f"=========================================================================\n{TermColors.RESET}"
    )


async def background_task_wrapper(
        request: Request,
        func: Callable[..., Any],
        *args: Any,
        **kwargs: Any
):
    try:
        if is_async_callable(func):
            await func(*args, **kwargs)
        else:
            func(*args, **kwargs)
    except Exception as e:
        _log_bg_error(e, func, request)
