# !/usr/bin/env pythonwsgi_app
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：config_gunicorn
# @Date   ：2025/4/19 17:59
# @Author ：leemysw

# 2025/4/19 17:59   Create
# =====================================================

import os

from agent.core.config import settings
from agent.utils.logger import logger

bind = f"{settings.HOST}:{settings.PORT}"
workers = settings.WORKERS
worker_class = "uvicorn.workers.UvicornWorker"
logger_class = "agent.utils.logger.logger_gunicorn.GunicornLogger"

reuse_port=True
capture_output = True
loglevel = settings.LOG_LEVEL

# Timeout for graceful workers restart
timeout = 30 * 60
max_requests = 2000
max_requests_jitter = 500


def on_starting(server):
    """
    Attach a set of IDs that can be temporarily re-used.
    Used on reloads when each worker exists twice.
    """
    server._worker_id_overload = set()


def nworkers_changed(server, new_value, old_value):
    """
    Gets called on startup too.
    Set the current number of workers.  Required if we raise the worker count
    temporarily using TTIN because server.cfg.workers won't be updated and if
    one of those workers dies, we wouldn't know the ids go that far.
    """
    server._worker_id_current_workers = new_value


def _next_worker_id(server):
    """
    If there are IDs open for re-use, take one.  Else look for a free one.
    """
    if server._worker_id_overload:
        return server._worker_id_overload.pop()

    in_use = set(w._worker_id for w in server.WORKERS.values() if w.alive)
    free = set(range(1, server._worker_id_current_workers + 1)) - in_use

    return free.pop()


def on_reload(server):
    """
    Add a full set of ids into overload so it can be re-used once.
    """
    server._worker_id_overload = set(range(1, server.cfg.workers + 1))


def pre_fork(server, worker):
    """
    Attach the next free worker_id before forking off.
    """
    worker._worker_id = _next_worker_id(server)

    logger.info(f"Worker {worker._worker_id} (pid: {worker.pid}) is ready")


def post_fork(server, worker):
    """
    Put the worker_id into an env variable for further use within the app.
    """
    os.environ["APP_WORKER_ID"] = str(worker._worker_id)

    logger.info(f"Worker {worker._worker_id} (pid: {worker.pid}) started")

    worker.notify()
