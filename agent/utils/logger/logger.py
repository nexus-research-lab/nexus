# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：logger
# @Date   ：2024/1/22 23:27
# @Author ：leemysw

# 2024/1/22 23:27   Create
# =====================================================

import logging
import os
import re
import sys
import time
from logging import handlers
from logging.handlers import RotatingFileHandler, TimedRotatingFileHandler
from typing import Optional

from agent.core.config import settings
from agent.utils.utils import abspath, ROOT_PATH


def remove_ansi_escape(text):
    ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
    return ansi_escape.sub('', text)


class Formatter(logging.Formatter):
    def __init__(self, fmt, **kwargs):
        super().__init__(fmt, **kwargs)
        self.base_dir = ROOT_PATH

    def format(self, record):
        # 计算相对于指定根目录的路径
        record.filename = os.path.relpath(record.pathname, self.base_dir).replace(os.sep, ".")
        return super().format(record)


def setup_logger(
        name: str,
        save: Optional[bool] = False,
        filename: Optional[str] = None,
        mode: str = 'a',
        distributed_rank: bool = False,
        stdout: bool = True,
        socket: bool = False,
        rotating_size: bool = False,
        rotating_time: bool = False,
        level: str = 'debug',
        backupCount: int = 10

):
    """
    日志模块

    :param level: 日志级别
    :param name: 日志名称
    :param filename: 日志文件名
    :param mode: 写模式
    :param distributed_rank: 是否分布式
    :param stdout: 是否终端输出
    :param save: 是否保存日志文件
    :param socket: 是否输出到socket
    :param rotating_size: 是否按文件大小切割
    :param rotating_time: 是都按日期切割
    :param backupCount: 保留日志文件个数
    :return:
    """

    if name in logging.Logger.manager.loggerDict.keys():
        return logging.getLogger(name)

    logger = logging.getLogger(name)
    level = level.upper()
    logger.setLevel(level)
    logger.propagate = False
    if distributed_rank:
        return logger

    formatter = Formatter(settings.LOGGER_FORMAT, datefmt="%Y-%m-%d %H:%M:%S")
    writer_formatter = Formatter(remove_ansi_escape(settings.LOGGER_FORMAT), datefmt="%Y-%m-%d %H:%M:%S")

    if stdout:
        ch = logging.StreamHandler(stream=sys.stdout)
        ch.setLevel(level)
        ch.setFormatter(formatter)
        logger.addHandler(ch)

    if socket:
        socketHandler = handlers.SocketHandler('localhost', logging.handlers.DEFAULT_TCP_LOGGING_PORT)
        socketHandler.setLevel(level)
        socketHandler.setFormatter(formatter)
        logger.addHandler(socketHandler)

    if save or filename:
        if filename is None:
            filename = time.strftime("%Y-%m-%d_%H.%M.%S", time.localtime()) + ".log"

        if not os.path.exists(os.path.dirname(filename)):
            os.makedirs(os.path.dirname(filename), exist_ok=True)

        if rotating_time:
            # 每 1(interval) 天(when) 重写1个文件,保留7(backupCount) 个旧文件；when还可以是Y/m/H/M/S
            th = TimedRotatingFileHandler(filename, when='D', interval=1, backupCount=backupCount, encoding="UTF-8")
            th.setLevel(level)
            th.setFormatter(writer_formatter)
            logger.addHandler(th)

        elif rotating_size:
            # 每 1024Bytes重写一个文件,保留2(backupCount) 个旧文件
            sh = RotatingFileHandler(filename, mode=mode, maxBytes=1024 * 1024, backupCount=backupCount,
                                     encoding="UTF-8")
            sh.setLevel(level)
            sh.setFormatter(writer_formatter)
            logger.addHandler(sh)

        else:
            fh = logging.FileHandler(filename, mode=mode, encoding="UTF-8")
            fh.setLevel(level)
            fh.setFormatter(writer_formatter)
            logger.addHandler(fh)

    return logger


def cleanup_container_folders(base_path: str, hostname: str, max_days: int = 7):
    """
    清理旧的容器日志目录：
    1. 匹配容器ID格式的目录
    2. 不是当前容器的目录
    3. 目录中最新日志文件超过指定天数未更新

    Args:
        base_path: 基础路径，例如 logs 目录
        hostname: 当前容器的hostname，此目录将被保留
        max_days: 日志保留天数，默认7天
    """
    import re
    import shutil
    from datetime import datetime, timedelta

    # 匹配12位的十六进制字符串（标准Docker容器ID格式）
    container_pattern = re.compile(r'^[0-9a-f]{12}$')
    # 当前时间
    now = datetime.now()
    # 最大保留时间
    max_delta = timedelta(days=max_days)

    def get_latest_file_time(dir_path: str) -> datetime:
        """获取目录下最新文件的修改时间"""
        latest_time = datetime.fromtimestamp(0)  # 初始化为最早时间

        for root, _, files in os.walk(dir_path):
            for file in files:
                file_path = os.path.join(root, file)
                try:
                    mtime = datetime.fromtimestamp(os.path.getmtime(file_path))
                    latest_time = max(latest_time, mtime)
                except Exception:
                    continue

        return latest_time

    try:
        if not os.path.exists(base_path):
            return

        for item in os.listdir(base_path):
            item_path = os.path.join(base_path, item)

            # 跳过非目录和当前容器目录
            if not os.path.isdir(item_path) or item == hostname:
                continue

            # 检查是否是容器ID格式
            if not container_pattern.match(item):
                continue

            # 获取目录下最新文件的修改时间
            latest_time = get_latest_file_time(item_path)
            time_delta = now - latest_time

            # 如果超过指定天数未更新，则删除
            if time_delta > max_delta:
                try:
                    shutil.rmtree(item_path)
                    print(f"Removed old container log directory: {item_path} "
                          f"(last modified: {latest_time.strftime('%Y-%m-%d %H:%M:%S')})")
                except Exception as e:
                    print(f"Failed to remove container folder {item_path}: {e}")

    except Exception as e:
        print(f"Error during container folders cleanup: {e}")


# 清理旧日志文件
# _, hostname = get_host_ip()
# cleanup_container_folders(abspath(settings.LOG_PATH), hostname, max_days=7)
logger = setup_logger(
    name=settings.PROJECT_NAME,
    filename=abspath(f"{settings.LOG_PATH}/logger.log"),
    stdout=True,
    level=settings.LOG_LEVEL,
    rotating_time=True,
    backupCount=7,
)
