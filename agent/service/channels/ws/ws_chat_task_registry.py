# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：ws_chat_task_registry.py
# @Date   ：2026/04/07 13:02
# @Author ：leemysw
# 2026/04/07 13:02   Create
# =====================================================

"""WebSocket 运行中聊天任务注册表。"""

import asyncio
from typing import Optional


class WsChatTaskRegistry:
    """托管跨连接存活的聊天任务。"""

    def __init__(self) -> None:
        self.tasks: dict[str, asyncio.Task] = {}
        self._round_ids: dict[str, str] = {}

    def register(self, session_key: str, task: asyncio.Task, round_id: Optional[str]) -> None:
        """注册运行中的会话任务。"""
        self.tasks[session_key] = task
        if round_id:
            self._round_ids[session_key] = round_id
        else:
            self._round_ids.pop(session_key, None)

    def unregister(self, session_key: str, task: Optional[asyncio.Task] = None) -> None:
        """注销会话任务，仅移除当前仍匹配的任务。"""
        current_task = self.tasks.get(session_key)
        if task is not None and current_task is not task:
            return
        self.tasks.pop(session_key, None)
        self._round_ids.pop(session_key, None)

    def is_running(self, session_key: str) -> bool:
        """判断指定 session 是否有运行中的任务。"""
        task = self.tasks.get(session_key)
        return task is not None and not task.done()

    def get_running_round_id(self, session_key: str) -> Optional[str]:
        """返回指定 session 当前运行的 round_id。"""
        if not self.is_running(session_key):
            return None
        return self._round_ids.get(session_key)


ws_chat_task_registry = WsChatTaskRegistry()
