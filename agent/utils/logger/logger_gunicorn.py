# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：gunicorn_logger
# @Date   ：2025/5/16 12:43
# @Author ：leemysw

# 2025/5/16 12:43   Create
# =====================================================

import logging
import os
from logging import handlers

from gunicorn.glogging import Logger

from agent.core.config import settings
from agent.utils.logger.logger import cleanup_container_folders, remove_ansi_escape
from agent.utils.utils import abspath, get_host_ip


class GunicornLogger(Logger):
    datefmt = "%Y-%m-%d %H:%M:%S"
    error_fmt = remove_ansi_escape(settings.LOGGER_FORMAT)

    access_fmt = error_fmt
    syslog_fmt = error_fmt

    def setup(self, cfg):
        super().setup(cfg)

        # 清理旧日志文件
        _, hostname = get_host_ip()
        cleanup_container_folders(abspath(settings.LOG_PATH), hostname, max_days=7)


        s_logger_path = abspath(f"{settings.LOG_PATH}/logger_gunicorn.log")
        logger_path = abspath(f"{settings.LOG_PATH}")

        if not os.path.exists(logger_path):
            os.makedirs(logger_path)

        formatter = logging.Formatter(self.error_fmt, self.datefmt)
        th = handlers.TimedRotatingFileHandler(s_logger_path, when='D', interval=1, backupCount=7, encoding="UTF-8")
        th.setLevel(settings.LOG_LEVEL)
        th.setFormatter(formatter)
        self.error_log.addHandler(th)
        self.access_log.addHandler(th)
