# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：retry
# @Date   ：2025/9/9 19:25
# @Author ：leemysw

# 2025/9/9 19:25   Create
# =====================================================

import asyncio
import time
from functools import wraps


def retry(max_retries=3, delay=1, backoff=2, exceptions=(Exception,)):
    """
    A decorator that retries the decorated function if it raises specified exceptions.

    Args:
        max_retries (int): Maximum number of retry attempts
        delay (float): Initial delay between retries in seconds
        backoff (float): Multiplier applied to delay between retries
        exceptions (tuple): Tuple of exceptions to catch and retry on

    Returns:
        The decorator function
    """

    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            from agent.utils.logger import logger
            retries = 0
            cur_delay = delay

            while retries <= max_retries:
                try:
                    return func(*args, **kwargs)
                except exceptions as e:
                    retries += 1
                    if retries > max_retries:
                        raise e

                    logger.warning(f"Retry {retries}/{max_retries} after {cur_delay}s. Error: {str(e)}")
                    time.sleep(cur_delay)
                    cur_delay *= backoff
            return None

        @wraps(func)
        async def async_wrapper(*args, **kwargs):
            from agent.utils.logger import logger
            retries = 0
            cur_delay = delay

            while retries <= max_retries:
                try:
                    return await func(*args, **kwargs)
                except exceptions as e:
                    retries += 1
                    if retries > max_retries:
                        raise e

                    logger.warning(f"Retry {retries}/{max_retries} after {cur_delay}s. Error: {str(e)}")
                    await asyncio.sleep(cur_delay)
                    cur_delay *= backoff
            return None

        if asyncio.iscoroutinefunction(func):
            return async_wrapper
        return wrapper

    return decorator
