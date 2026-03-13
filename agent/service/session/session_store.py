# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_store.py
# @Date   ：2025/11/28 22:29
# @Author ：leemysw
#
# 2025/11/28 22:29   Create
# 2026/2/25          重构：session_key 路由
# =====================================================

"""
消息历史存储

[INPUT]: 依赖文件版 session_repository
[OUTPUT]: 对外提供 MessageHistoryStore（会话和消息的业务层操作）
[POS]: service 层的存储门面，被 ChatService/SessionManager/API 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import Dict, List, Optional

from agent.infra.storage.cost_repository import cost_repository
from agent.infra.storage.session_repository import session_repository
from agent.schema.model_message import AMessage
from agent.schema.model_session import ASession
from agent.utils.logger import logger


class MessageHistoryStore:
    """消息历史存储 — session_key 路由"""

    def __init__(self):
        logger.info("📁 历史存储初始化: 使用 workspace 文件存储")

    # =====================================================
    # Session 操作 — 以 session_key 为主键
    # =====================================================

    async def get_session_by_key(self, session_key: str) -> Optional[ASession]:
        """按 session_key 获取会话"""
        return await session_repository.get_session(session_key)

    async def create_session_by_key(
            self,
            session_key: str,
            channel_type: str = "websocket",
            chat_type: str = "dm",
            title: Optional[str] = None,
            options: Optional[Dict] = None,
    ) -> Optional[ASession]:
        """创建新会话并返回"""
        success = await session_repository.create_session(
            session_key=session_key,
            channel_type=channel_type,
            chat_type=chat_type,
            title=title or "New Chat",
            options=options,
        )
        if success:
            return await session_repository.get_session(session_key)
        return None

    async def get_session_info(self, session_key: str) -> Optional[ASession]:
        """获取会话信息"""
        return await session_repository.get_session(session_key)

    async def update_session(
            self,
            session_key: str,
            agent_id: str = "main",
            session_id: Optional[str] = None,
            title: Optional[str] = None,
            options: Optional[Dict] = None,
    ) -> bool:
        """创建或更新会话"""
        existing = await session_repository.get_session(session_key)
        if not existing:
            return await session_repository.create_session(
                session_key=session_key,
                agent_id=agent_id,
                session_id=session_id,
                title=title or "New Chat",
                options=options,
            )
        return await session_repository.update_session(
            session_key=session_key,
            session_id=session_id,
            title=title,
            options=options,
        )

    async def get_all_sessions(self) -> List[ASession]:
        """获取所有会话列表"""
        return await session_repository.get_all_sessions()

    async def delete_session(self, session_key: str) -> bool:
        """删除会话"""
        session_info = await session_repository.get_session(session_key)
        success = await session_repository.delete_session(session_key)
        if success and session_info:
            await cost_repository.handle_session_deleted(session_key, session_info.agent_id)
        return success

    # =====================================================
    # Message 操作
    # =====================================================

    async def save_message(self, message: AMessage) -> bool:
        """保存消息"""
        try:
            session_info = await session_repository.get_session(message.session_key)
            if not session_info:
                logger.error(f"❌ 会话不存在: {message.session_key}")
                return False

            # 统一以会话绑定的 Agent 为准，避免 SDK 转换链路丢失 agent_id 后写入错误 workspace。
            if message.agent_id in ("", "main") and session_info.agent_id not in ("", None):
                message.agent_id = session_info.agent_id
            elif message.agent_id:
                message.agent_id = message.agent_id
            else:
                message.agent_id = session_info.agent_id or "main"

            if not message.session_id and session_info.session_id:
                message.session_id = session_info.session_id

            success = await session_repository.create_message(message=message)
            if success and message.message_type == "result":
                await cost_repository.record_result_message(message)
            return success
        except Exception as e:
            logger.error(f"❌ 保存消息失败: {e}")
            return False

    async def get_session_messages(self, session_key: str) -> List[AMessage]:
        """获取会话历史消息"""
        return await session_repository.get_session_messages(session_key)

    async def delete_round(self, session_key: str, round_id: str) -> int:
        """删除一轮对话"""
        deleted_count = await session_repository.delete_round(session_key, round_id)
        if deleted_count > 0:
            session_info = await session_repository.get_session(session_key)
            await cost_repository.delete_round_costs(
                session_key=session_key,
                round_id=round_id,
                agent_id=session_info.agent_id if session_info else None,
            )
        return deleted_count

    async def get_latest_round_id(self, session_key: str) -> Optional[str]:
        """获取最新 round_id"""
        return await session_repository.get_latest_round_id(session_key)

    async def has_round_result(self, session_key: str, round_id: str) -> bool:
        """检查指定轮次是否已有 result 消息。"""
        return await session_repository.has_round_result(session_key, round_id)

    async def get_session_cost_summary(self, session_key: str):
        """获取 Session 成本汇总。"""
        return await cost_repository.get_session_cost_summary(session_key)

    async def get_agent_cost_summary(self, agent_id: str):
        """获取 Agent 成本汇总。"""
        return await cost_repository.get_agent_cost_summary(agent_id)


# 全局实例
session_store = MessageHistoryStore()
