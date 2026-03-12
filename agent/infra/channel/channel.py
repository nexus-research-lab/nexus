# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：channel.py
# @Date   ：2026/2/25 15:45
# @Author ：leemysw
#
# 2026/2/25 15:45   Create
# =====================================================

"""
通道核心协议定义

[INPUT]: 依赖 agent.service.schema.model_message 的 AMessage/AEvent/AError,
         依赖 claude_agent_sdk 的 PermissionResult
[OUTPUT]: 对外提供 MessageSender/MessageChannel/PermissionStrategy 三个抽象协议
[POS]: channel 模块的协议定义层，所有通道实现都必须遵循这些协议
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from abc import ABC, abstractmethod
from typing import Any, Union

from claude_agent_sdk import PermissionResult, ToolPermissionContext

from agent.service.schema.model_message import AError, AEvent, AMessage


# =====================================================
# MessageSender — 消息发送协议
#
# 替代 BaseHandler 对 WebSocket 的硬编码依赖。
# 所有 handler 通过此协议发送消息，不感知底层传输。
# =====================================================

class MessageSender(ABC):
    """消息发送协议 — handler 层的唯一出口"""

    async def send(self, message: Union[AMessage, AEvent, AError]) -> None:
        """统一发送入口，自动分发到具体方法"""
        if isinstance(message, AMessage):
            await self.send_message(message)
        elif isinstance(message, AEvent):
            await self.send_event(message)
        elif isinstance(message, AError):
            await self.send_error(message)

    @abstractmethod
    async def send_message(self, message: AMessage) -> None:
        """发送 Agent 消息（助手回复/用户消息/系统消息/结果）"""
        ...

    @abstractmethod
    async def send_event(self, event: AEvent) -> None:
        """发送事件（权限请求等）"""
        ...

    @abstractmethod
    async def send_error(self, error: AError) -> None:
        """发送错误"""
        ...


# =====================================================
# MessageChannel — 通道生命周期协议
#
# 每种通道（WebSocket/Discord/Telegram）实现此协议，
# 由 ChannelManager 统一管理启动/停止。
# =====================================================

class MessageChannel(ABC):
    """消息通道生命周期管理"""

    @property
    @abstractmethod
    def channel_type(self) -> str:
        """通道类型标识，如 'websocket'、'discord'、'telegram'"""
        ...

    @abstractmethod
    async def start(self) -> None:
        """启动通道"""
        ...

    @abstractmethod
    async def stop(self) -> None:
        """停止通道"""
        ...


# =====================================================
# PermissionStrategy — 权限决策策略
#
# WebSocket: 交互式审批（弹窗等待用户点击）
# Discord/Telegram: 自动允许 + 工具白名单
# =====================================================

class PermissionStrategy(ABC):
    """可插拔的工具权限决策策略"""

    @abstractmethod
    async def request_permission(
        self,
        agent_id: str,
        tool_name: str,
        input_data: dict[str, Any],
        context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """请求工具使用权限

        Args:
            agent_id: 会话 ID
            tool_name: 工具名称
            input_data: 工具输入参数
            context: SDK 传入的权限上下文

        Returns:
            PermissionResult: 允许或拒绝
        """
        ...
