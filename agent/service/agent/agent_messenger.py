# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_messenger.py
# @Date   ：2026/3/30
# @Author ：leemysw
# =====================================================

"""Agent 间消息转发服务。"""

import asyncio
from typing import Any, Dict, Optional

from claude_agent_sdk import ClaudeSDKClient
from agent.config.config import settings
from agent.schema.model_agent import AAgent
from agent.service.agent.agent_repository import agent_repository
from agent.service.agent.agent_runtime import agent_runtime
from agent.service.channels.message_sender import MessageSender
from agent.schema.model_message import StreamMessage, Message
from agent.service.message.sdk_message_mapper import SdkMessageMapper
from agent.service.permission.strategy.permission_strategy import PermissionStrategy
from agent.service.session.session_router import build_session_key
from agent.utils.logger import logger


class AgentMessenger:
    """负责 Agent 之间的消息转发和响应收集。"""

    def __init__(self, sender: MessageSender, permission_strategy: PermissionStrategy):
        self._sender = sender
        self._permission_strategy = permission_strategy
        self._pending_delegates: Dict[str, asyncio.Future] = {}

    async def send_message(
        self,
        from_session_key: str,
        from_agent_id: str,
        to_agent_id: str,
        content: str,
        room_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """发送消息给目标 Agent 并等待响应。

        Args:
            from_session_key: 发送方的 session_key
            from_agent_id: 发送方的 agent_id
            to_agent_id: 接收方的 agent_id
            content: 消息内容
            room_id: 所属 room_id（用于上下文）

        Returns:
            包含响应消息的字典，或错误信息
        """
        # 检查目标 agent 是否存在
        target_agent = await agent_repository.get_agent(to_agent_id)
        if not target_agent:
            logger.warning(f"⚠️ 目标 agent 不存在: {to_agent_id}")
            return {"success": False, "error": "Agent not found"}

        # 中文注释：Agent 内部委派也必须走统一 session_key 规则，避免出现不可解析的孤立格式。
        target_session_key = build_session_key(
            channel="internal",
            chat_type="dm",
            ref="chat",
            agent_id=to_agent_id,
        )

        try:
            # 获取或创建目标 agent 的客户端
            target_client = await agent_runtime.get_or_create_client(
                session_key=target_session_key,
                agent_id=to_agent_id,
                permission_strategy=self._permission_strategy,
            )

            logger.info(
                f"📤 Agent 间通信: {from_agent_id} -> {to_agent_id}, "
                f"session={from_session_key} -> {target_session_key}"
            )

            # 记录委派开始消息
            await self._sender.send(
                self._build_delegate_start_message(
                    from_session_key=from_session_key,
                    from_agent_id=from_agent_id,
                    to_agent_id=to_agent_id,
                    content=content,
                    room_id=room_id,
                )
            )

            # 发送消息给目标 agent
            target_client = await agent_runtime.query_with_recovery(
                session_key=target_session_key,
                agent_id=to_agent_id,
                permission_strategy=self._permission_strategy,
                prompt=content,
                client=target_client,
                resolved_agent_id=to_agent_id,
            )

            # 记录委派完成消息
            await self._sender.send(
                self._build_delegate_complete_message(
                    from_session_key=from_session_key,
                    from_agent_id=from_agent_id,
                    to_agent_id=to_agent_id,
                )
            )

            # 返回成功响应（简化版，实际应该等待并返回响应内容）
            return {
                "success": True,
                "to_agent_id": to_agent_id,
                "to_agent_name": target_agent.name,
                "message": content,
            }

        except Exception as exc:
            logger.error(f"❌ Agent 间通信失败: {exc}")
            return {"success": False, "error": str(exc)}

    def _build_delegate_start_message(
        self,
        from_session_key: str,
        from_agent_id: str,
        to_agent_id: str,
        content: str,
        room_id: Optional[str],
    ) -> StreamMessage:
        """构造委派开始时的流式消息。"""
        return {
            "session_key": from_session_key,
            "type": "agent_delegate_start",
            "round_id": from_session_key,
            "content": {
                "from_agent_id": from_agent_id,
                "to_agent_id": to_agent_id,
                "message": content,
                "room_id": room_id,
            },
            "is_complete": True,
        }

    def _build_delegate_complete_message(
        self,
        from_session_key: str,
        from_agent_id: str,
        to_agent_id: str,
    ) -> StreamMessage:
        """构造委派完成时的消息。"""
        return {
            "session_key": from_session_key,
            "type": "agent_delegate_complete",
            "round_id": from_session_key,
            "content": {
                "from_agent_id": from_agent_id,
                "to_agent_id": to_agent_id,
            },
            "is_complete": True,
        }


# 全局实例
_messenger_instance: Optional[AgentMessenger] = None


def get_messenger() -> AgentMessenger:
    """获取全局 messenger 实例（延迟初始化）。"""
    global _messenger_instance
    if _messenger_instance is None:
        raise RuntimeError("Messenger not initialized. Call init_messenger first.")
    return _messenger_instance


def init_messenger(sender: MessageSender, permission_strategy: PermissionStrategy) -> AgentMessenger:
    """初始化全局 messenger 实例。"""
    global _messenger_instance
    if _messenger_instance is None:
        _messenger_instance = AgentMessenger(sender, permission_strategy)
    return _messenger_instance
