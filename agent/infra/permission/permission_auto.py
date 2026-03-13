# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：tool_guard.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""工具调用权限拦截与权限策略实现。"""

from typing import Any, Optional, Set

from claude_agent_sdk import (
    PermissionResult,
    PermissionResultAllow,
    PermissionResultDeny,
    ToolPermissionContext,
)

from agent.infra.permission.permission_strategy import PermissionStrategy
from agent.utils.logger import logger


class AutoAllowPermissionStrategy(PermissionStrategy):
    """非交互通道专用的自动允许权限策略。"""

    DEFAULT_ALLOWED_TOOLS: Set[str] = {
        "Task",
        "TaskOutput",
        "Edit",
        "TodoWrite",
        "Read",
        "Bash",
        "KillShell",
        "Grep",
        "Glob",
        "LS",
        "Write",
        "Skill",
        "WebSearch",
        "WebFetch",
        "AskUserQuestion",
    }

    def __init__(self, allowed_tools: Optional[Set[str]] = None) -> None:
        self._allowed_tools = allowed_tools or self.DEFAULT_ALLOWED_TOOLS

    async def request_permission(
            self,
            session_key: str,
            tool_name: str,
            input_data: dict[str, Any],
            context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """白名单工具自动允许，其余拒绝。"""
        del context
        if tool_name in self._allowed_tools:
            logger.debug(f"✅ 自动允许工具: {tool_name} (session={session_key})")
            return PermissionResultAllow(updated_input=input_data)

        logger.info(f"🚫 自动拒绝工具: {tool_name} (session={session_key})")
        return PermissionResultDeny(
            message=f"Tool '{tool_name}' is not allowed in this channel"
        )
