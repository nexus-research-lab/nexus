# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_error_warp
# @Date   ：2025/5/18 23:41
# @Author ：leemysw

# 2025/5/18 23:41   Create
# =====================================================

import asyncio
import functools
from typing import Callable, ParamSpec, TypeVar

from .base_exception import ServerException

P = ParamSpec('P')
R = TypeVar('R')


def exception_to_base_error(func: Callable[P, R]) -> Callable[P, R]:
    """
    装饰器: 将函数中的所有异常转换为BaseError
    """

    @functools.wraps(func)
    async def async_wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        try:
            return await func(*args, **kwargs)
        except ServerException:
            raise
        except Exception as e:
            raise ServerException(f"系统错误: {str(e)}")

    @functools.wraps(func)
    def sync_wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        try:
            return func(*args, **kwargs)
        except ServerException:
            raise
        except Exception as e:
            raise ServerException(f"系统错误: {str(e)}")

    if asyncio.iscoroutinefunction(func):
        return async_wrapper
    return sync_wrapper
