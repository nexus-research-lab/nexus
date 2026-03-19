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

from agent.infra.database.get_db import get_db
from agent.service.persistence.agent_persistence_service import (
    agent_persistence_service,
)
from agent.service.persistence.conversation_persistence_service import (
    conversation_persistence_service,
)
from agent.service.persistence.legacy_sync_bridge import (
    build_agent_aggregate_from_legacy,
    build_dm_context_from_legacy,
    build_message_record_from_legacy,
    build_round_record_from_legacy,
    extract_existing_runtime_id,
)
from agent.storage.sqlite.message_sql_repository import MessageSqlRepository
from agent.storage.cost_repository import cost_repository
from agent.storage.session_repository import session_repository
from agent.schema.model_message import Message
from agent.schema.model_session import ASession
from agent.utils.logger import logger


class MessageHistoryStore:
    """消息历史存储 — session_key 路由"""

    def __init__(self):
        self._db = get_db("async_sqlite")
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
            session_info = await session_repository.get_session(session_key)
            if session_info:
                await self._sync_session_to_sql(session_info)
            return session_info
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
            success = await session_repository.create_session(
                session_key=session_key,
                agent_id=agent_id,
                session_id=session_id,
                title=title or "New Chat",
                options=options,
            )
            if success:
                created = await session_repository.get_session(session_key)
                if created:
                    await self._sync_session_to_sql(created)
            return success
        success = await session_repository.update_session(
            session_key=session_key,
            session_id=session_id,
            title=title,
            options=options,
        )
        if success:
            updated = await session_repository.get_session(session_key)
            if updated:
                await self._sync_session_to_sql(updated)
        return success

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

    async def save_message(self, message: Message) -> bool:
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
            if success and message.role == "result":
                await cost_repository.record_result_message(message)
            if success:
                await self._sync_message_to_sql(message, session_info)
            return success
        except Exception as e:
            logger.error(f"❌ 保存消息失败: {e}")
            return False

    async def get_session_messages(self, session_key: str) -> List[Message]:
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

    async def sync_session_to_sql(self, session_info: ASession) -> None:
        """公开的会话双写入口。"""
        await self._sync_session_to_sql(session_info)

    async def sync_session_messages_to_sql(self, session_info: ASession) -> None:
        """公开的消息双写入口。"""
        await self._sync_session_messages_to_sql(session_info)

    async def _sync_session_to_sql(self, session_info: ASession) -> None:
        """将文件版会话同步写入新数据库。"""
        try:
            from agent.service.agent.agent_manager import agent_manager

            agent_aggregate = await agent_persistence_service.get_agent_aggregate(
                session_info.agent_id,
            )
            if agent_aggregate is None:
                legacy_agent = await agent_manager.get_agent(session_info.agent_id)
                if legacy_agent is not None:
                    agent_aggregate = await agent_persistence_service.create_agent_aggregate(
                        build_agent_aggregate_from_legacy(legacy_agent),
                    )

            runtime_id = extract_existing_runtime_id(
                agent_aggregate.runtime.id if agent_aggregate else None,
                session_info.agent_id,
            )
            room, members, conversation, session_record = build_dm_context_from_legacy(
                session_info=session_info,
                runtime_id=runtime_id,
            )
            existing_conversations = await conversation_persistence_service.get_room_conversations(
                room.id,
            )
            if existing_conversations:
                await conversation_persistence_service.touch_session(session_record.id)
                await self._sync_session_messages_to_sql(session_info)
                return

            await conversation_persistence_service.create_dm_context(
                room=room,
                members=members,
                conversation=conversation,
                session_record=session_record,
            )
            await self._sync_session_messages_to_sql(session_info)
        except Exception as exc:
            # 新库同步失败不应阻断现有文件存储主链路。
            logger.warning(
                f"⚠️ Session SQL 同步失败: key={session_info.session_key}, error={exc}",
            )

    async def _sync_message_to_sql(
        self,
        message: Message,
        session_info: ASession,
    ) -> None:
        """将消息索引同步写入新数据库。"""
        try:
            log_path = session_repository._find_message_log_path(message.session_key)
            if log_path is None:
                return
            message_record = build_message_record_from_legacy(
                message=message,
                session_info=session_info,
                jsonl_path=str(log_path),
            )
            round_record = build_round_record_from_legacy(
                message=message,
                session_info=session_info,
            )
            async with self._db.session() as session:
                repository = MessageSqlRepository(session)
                await repository.upsert_message(message_record)
                if round_record is not None:
                    await repository.upsert_round(round_record)
                await session.commit()
        except Exception as exc:
            # 新库同步失败不应阻断现有文件存储主链路。
            logger.warning(
                f"⚠️ Message SQL 同步失败: message={message.message_id}, error={exc}",
            )

    async def _sync_session_messages_to_sql(self, session_info: ASession) -> None:
        """将一个会话下的全部消息索引同步写入新数据库。"""
        messages = await session_repository.get_session_messages(session_info.session_key)
        for message in messages:
            await self._sync_message_to_sql(message=message, session_info=session_info)


# 全局实例
session_store = MessageHistoryStore()
