# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：message_channel.py
# @Date   ：2026/3/13 18:24
# @Author ：leemysw
# 2026/3/13 18:24   Create
# =====================================================

"""通道生命周期协议。"""

from abc import ABC, abstractmethod


class MessageChannel(ABC):
    """消息通道生命周期管理协议。"""

    @property
    @abstractmethod
    def channel_type(self) -> str:
        """返回通道类型标识。"""
        ...

    @abstractmethod
    async def start(self) -> None:
        """启动通道。"""
        ...

    @abstractmethod
    async def stop(self) -> None:
        """停止通道。"""
        ...
