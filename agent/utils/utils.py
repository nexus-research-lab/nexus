# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：utils
# @Date   ：2024/1/22 23:34
# @Author ：leemysw

# 2024/1/22 23:34   Create
# =====================================================

import asyncio
import os
import signal
import socket
import sys
import threading
import time
import uuid
from contextlib import contextmanager
from functools import wraps
from types import SimpleNamespace
from typing import Optional

import psutil

ROOT_PATH = os.path.abspath(os.path.abspath(os.path.dirname(__file__)) + '/../')


def dict2obj(d):
    if isinstance(d, dict):
        return SimpleNamespace(**{k: dict2obj(v) for k, v in d.items()})
    else:
        return d


def obj2dict(obj):
    adict = {}
    for name in dir(obj):
        value = getattr(obj, name)
        if not name.startswith('__') and not callable(value) and not name.startswith('_'):
            adict[name] = value
    return adict


def check_gpu(use_gpu):
    import torch
    if use_gpu and not torch.cuda.is_available():
        use_gpu = False


def get_host_ip():
    ip = ''
    host_name = ''
    # noinspection PyBroadException
    try:
        sc = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sc.connect(('8.8.8.8', 80))
        ip = sc.getsockname()[0]
        host_name = socket.gethostname()
        sc.close()
    except Exception:
        pass
    return ip, host_name


def abspath(r_path):
    path = os.path.abspath(os.path.join(ROOT_PATH, r_path))
    sh_path = '/'.join(path.split('\\'))

    return sh_path


def sources_path(spath):
    path = os.path.abspath(os.path.join(ROOT_PATH, "resources"))
    return os.path.join(path, spath)


def cache_path(cache_root: str, spath: str):
    path = os.path.abspath(os.path.join(cache_root, spath))
    return path


# ------------------------------------------------------------------- #
def set_uvicorn_logger(fmt):
    from uvicorn.config import LOGGING_CONFIG

    date_fmt = "%Y-%m-%d %H:%M:%S"
    LOGGING_CONFIG["formatters"]["access"]["datefmt"] = date_fmt
    LOGGING_CONFIG["formatters"]["access"]["fmt"] = fmt

    LOGGING_CONFIG["formatters"]["default"]["datefmt"] = date_fmt
    LOGGING_CONFIG["formatters"]["default"]["fmt"] = fmt

    return LOGGING_CONFIG


def print_info(settings, logger):
    print(settings.LOGO)
    logger.info('=======================================')
    settings.status(logger=logger)
    logger.info('=======================================')


def find_process_using_port(port: int) -> Optional[psutil.Process]:
    # TODO: We can not check for running processes with network
    # port on macOS. Therefore, we can not have a full graceful shutdown
    # of vLLM. For now, let's not look for processes in this case.
    # Ref: https://www.florianreinhard.de/accessdenied-in-psutil/
    if sys.platform.startswith("darwin"):
        return None

    for conn in psutil.net_connections():
        if conn.laddr.port == port:
            try:
                return psutil.Process(conn.pid)
            except psutil.NoSuchProcess:
                return None
    return None


# ------------------------------------------------------------------- #

@contextmanager
def timeblock(label):
    from agent.utils.logger import logger
    start = time.time()
    try:
        yield
    finally:
        if time.time() - start != 0:
            logger.info('【{}】 cost time > {:.4f}'.format(label, time.time() - start))


# ------------------------------------------------------------------- #
def runtime(_func=None, *, prefix=""):
    if prefix:
        prefix = f"{prefix} - "

    def decorator(func):
        from agent.utils.logger import logger
        @wraps(func)
        def wrap(*args, **kwargs):
            t = time.perf_counter()
            ret = func(*args, **kwargs)
            logger.info(f"【{prefix}{func.__name__}】 runtime > {(time.perf_counter() - t)}")
            return ret

        @wraps(func)
        async def async_wrap(*args, **kwargs):
            t = time.perf_counter()
            ret = await func(*args, **kwargs)
            logger.info(f"【{prefix}{func.__name__}】 runtime > {(time.perf_counter() - t)}")
            return ret

        if asyncio.iscoroutinefunction(func):
            return async_wrap
        return wrap

    if _func is None:
        # @runtime(...) 的情况
        return decorator
    else:
        # @runtime 的情况
        return decorator(_func)


def singleton(cls):
    instances = {}
    lock = threading.Lock()

    def _singleton(*args, **kwargs):
        key = (cls, args, frozenset(kwargs.items()))
        # 双重检查锁定
        if key not in instances:
            with lock:
                if key not in instances:
                    instances[key] = cls(*args, **kwargs)
        return instances[key]

    return _singleton


def ensure_dir(directory):
    if not os.path.exists(directory):
        os.makedirs(directory)


def shutdown_func(sig, frame):
    print('close')
    print('=' * 50)
    print('-' * 50)
    print('=' * 50)


def shutdown(func=shutdown_func):
    for sig in [signal.SIGQUIT, signal.SIGTERM, signal.SIGKILL]:
        signal.signal(sig, func)


def synchronized(func):
    """
    Decorator in order to achieve thread-safe singleton class.
    """
    func.__lock__ = threading.Lock()

    def lock_func(*args, **kwargs):
        with func.__lock__:
            return func(*args, **kwargs)

    return lock_func


def random_uuid() -> str:
    return str(uuid.uuid4().hex)
