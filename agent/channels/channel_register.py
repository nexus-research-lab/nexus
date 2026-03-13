# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：channel_register.py
# @Date   ：2026/2/25 15:45
# @Author ：leemysw
#
# 2026/2/25 15:45   Create
# =====================================================

"""
通道管理器

[INPUT]: 依赖 message_channel.py 的 MessageChannel 协议
[OUTPUT]: 对外提供 ChannelRegister 单例
[POS]: channel 模块的编排层，在 app.py lifespan 中统一管理所有通道的生命周期
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import Dict, List

from agent.channels.message_channel import MessageChannel
from agent.utils.logger import logger


class ChannelRegister:
    """通道注册与生命周期管理"""

    def __init__(self):
        self._channels: Dict[str, MessageChannel] = {}

    def register(self, channel: MessageChannel) -> None:
        """注册通道"""
        if channel.channel_type in self._channels:
            logger.warning(f"⚠️ 通道已存在，覆盖: {channel.channel_type}")
        self._channels[channel.channel_type] = channel
        logger.info(f"📡 注册通道: {channel.channel_type}")

    async def start_all(self) -> None:
        """启动所有已注册通道"""
        for channel_type, channel in self._channels.items():
            try:
                await channel.start()
                logger.info(f"✅ 通道已启动: {channel_type}")
            except Exception as e:
                logger.error(f"❌ 通道启动失败: {channel_type}, error={e}")

    async def stop_all(self) -> None:
        """停止所有已注册通道"""
        for channel_type, channel in self._channels.items():
            try:
                await channel.stop()
                logger.info(f"🛑 通道已停止: {channel_type}")
            except Exception as e:
                logger.error(f"❌ 通道停止失败: {channel_type}, error={e}")

    def get(self, channel_type: str) -> MessageChannel:
        """获取指定类型的通道"""
        return self._channels.get(channel_type)

    @property
    def active_channels(self) -> List[str]:
        """返回已注册的通道类型列表"""
        return list(self._channels.keys())
