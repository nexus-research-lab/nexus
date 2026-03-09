#!/usr/bin/env python
# -*- coding: UTF-8 -*-
# =====================================================
# @File   ：snowfake
# @Date   ：2021/10/19 9:37
# @Author ：leemysw
# @Modify Time      @Author    @Version    @Description
# ------------      -------    --------    ------------
# 2021/10/19 9:37   leemysw      1.0.0         Create

# =====================================================

import time
import random

from agent.utils.logger import logger


class InvalidSystemClock(Exception):
    """
    时钟回拨异常
    """
    pass


# 64位ID的划分
WORKER_ID_BITS = 5
DATACENTER_ID_BITS = 5
SEQUENCE_BITS = 12

# 最大取值计算
MAX_WORKER_ID = -1 ^ (-1 << WORKER_ID_BITS)  # 2**5-1 0b11111
MAX_DATACENTER_ID = -1 ^ (-1 << DATACENTER_ID_BITS)

# 移位偏移计算
WOKER_ID_SHIFT = SEQUENCE_BITS
DATACENTER_ID_SHIFT = SEQUENCE_BITS + WORKER_ID_BITS
TIMESTAMP_LEFT_SHIFT = SEQUENCE_BITS + WORKER_ID_BITS + DATACENTER_ID_BITS

# 序号循环掩码
SEQUENCE_MASK = -1 ^ (-1 << SEQUENCE_BITS)

# 开始时间截 (2015-01-01)
TWEPOCH = 1288834974657


class IdWorker(object):
    """
    用于生成IDs
    """

    def __init__(self, datacenter_id, worker_id, sequence=0):
        """
        初始化
        :param datacenter_id: 数据中心（机器区域）ID
        :param worker_id: 机器ID
        :param sequence: 起始序号
        """
        # sanity check
        if worker_id > MAX_WORKER_ID or worker_id < 0:
            raise ValueError('worker_id值越界')
        if datacenter_id > MAX_DATACENTER_ID or datacenter_id < 0:
            raise ValueError('datacenter_id值越界')
        self.worker_id = worker_id
        self.datacenter_id = datacenter_id
        self.sequence = sequence
        self.last_timestamp = -1  # 上次计算的时间戳

    def _gen_timestamp(self):
        """
        生成整数时间戳
        :return:int timestamp
        """
        return int(time.time() * 1000)

    def get_id(self) -> str:
        """
        获取新ID
        :return:
        """
        timestamp = self._gen_timestamp()
        # 时钟回拨
        if timestamp < self.last_timestamp:
            logger.warning(f"时钟回拨: last_timestamp={self.last_timestamp}, now_timestamp={timestamp}")
            return self.get_id()
        if timestamp == self.last_timestamp:
            self.sequence = (self.sequence + 1) & SEQUENCE_MASK
            if self.sequence == 0:
                timestamp = self._til_next_millis(self.last_timestamp)
        else:
            self.sequence = 0
        self.last_timestamp = timestamp
        new_id = ((timestamp - TWEPOCH) << TIMESTAMP_LEFT_SHIFT) | (self.datacenter_id << DATACENTER_ID_SHIFT) | \
                 (self.worker_id << WOKER_ID_SHIFT) | self.sequence
        return str(new_id)

    def _til_next_millis(self, last_timestamp):
        """
        等到下一毫秒
        """
        timestamp = self._gen_timestamp()
        while timestamp <= last_timestamp:
            timestamp = self._gen_timestamp()
        return timestamp


worker = IdWorker(random.randint(0, 5), random.randint(0, 5), random.randint(0, 12))
