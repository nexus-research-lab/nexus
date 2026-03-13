#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_handler.py
# @Date   ：2026/3/13 18:10
# @Author ：leemysw
# 2026/3/13 18:10   Create
# =====================================================

"""WebSocket 权限响应处理器。"""

from typing import Any, Dict

from agent.channels.message_sender import MessageSender
from agent.infra.permission.permission_strategy import PermissionStrategy
from agent.channels.ws.handlers.base_handler import BaseHandler
from agent.utils.logger import logger


class PermissionHandler(BaseHandler):
    """权限响应处理器。"""

    def __init__(self, sender: MessageSender, permission_strategy: PermissionStrategy):
        super().__init__(sender)
        self.permission_strategy = permission_strategy

    async def handle_permission_response(self, message: Dict[str, Any]) -> None:
        """处理前端权限响应，转发给权限策略。"""
        if not self.permission_strategy.handle_permission_response(message):
            logger.warning("⚠️ 当前权限策略不支持前端权限响应")
