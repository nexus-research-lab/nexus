#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_handler.py
# @Date   ：2025/12/06
# @Author ：leemysw
#
# 2025/12/06   Create
# 2026/2/25    重构：权限逻辑提取到 InteractivePermissionStrategy
# =====================================================

"""
权限响应处理器（WebSocket 入站消息路由）

[INPUT]: 依赖 channel.channel 的 MessageSender/PermissionStrategy,
         依赖 channel.websocket_channel 的 InteractivePermissionStrategy
[OUTPUT]: 对外提供 PermissionHandler
[POS]: handler 模块的权限消息路由器，只负责接收前端 permission_response 并转发给策略
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import Any, Dict

from agent.infra.channel.channel import MessageSender, PermissionStrategy
from agent.infra.channel.websocket_channel import InteractivePermissionStrategy
from agent.service.handler.base_handler import BaseHandler
from agent.utils.logger import logger


class PermissionHandler(BaseHandler):
    """权限响应处理器"""

    def __init__(self, sender: MessageSender, permission_strategy: PermissionStrategy):
        super().__init__(sender)
        self.permission_strategy = permission_strategy

    async def handle_permission_response(self, message: Dict[str, Any]) -> None:
        """处理前端权限响应，转发给权限策略

        Args:
            message: 权限响应消息
        """
        # 只有交互式策略才需要处理前端响应
        if isinstance(self.permission_strategy, InteractivePermissionStrategy):
            self.permission_strategy.handle_permission_response(message)
        else:
            logger.warning(f"⚠️ 当前权限策略不支持前端权限响应")
