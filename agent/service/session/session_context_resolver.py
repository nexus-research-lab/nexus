# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_context_resolver.py
# @Date   ：2026/03/25 22:14
# @Author ：leemysw
# 2026/03/25 22:14   Create
# =====================================================

"""会话上下文解析服务。"""

from __future__ import annotations

from agent.infra.database.get_db import get_db
from agent.schema.model_session import ASession
from agent.storage.sqlite.conversation_sql_repository import ConversationSqlRepository
from agent.storage.sqlite.session_sql_repository import SessionSqlRepository


class SessionContextResolver:
    """为会话补齐 room / conversation 归属。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def enrich_session(self, session_info: ASession) -> ASession:
        """为单个会话补齐 room 与 conversation 信息。"""
        room_session_id = session_info.room_session_id
        if not room_session_id:
            return session_info

        async with self._db.session() as session:
            session_repository = SessionSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)

            session_record = await session_repository.get(room_session_id)
            if session_record is None:
                return session_info

            conversation_record = await conversation_repository.get(
                session_record.conversation_id
            )
            if conversation_record is None:
                return session_info

            return session_info.model_copy(
                update={
                    "room_id": conversation_record.room_id,
                    "conversation_id": conversation_record.id,
                }
            )

    async def enrich_sessions(self, sessions: list[ASession]) -> list[ASession]:
        """批量补齐会话上下文。"""
        if not sessions:
            return sessions

        async with self._db.session() as session:
            session_repository = SessionSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            conversation_cache: dict[str, tuple[str | None, str | None]] = {}
            enriched_sessions: list[ASession] = []

            for session_info in sessions:
                room_session_id = session_info.room_session_id
                if not room_session_id:
                    enriched_sessions.append(session_info)
                    continue

                session_record = await session_repository.get(room_session_id)
                if session_record is None:
                    enriched_sessions.append(session_info)
                    continue

                cache_key = session_record.conversation_id
                if cache_key not in conversation_cache:
                    conversation_record = await conversation_repository.get(cache_key)
                    conversation_cache[cache_key] = (
                        conversation_record.room_id if conversation_record else None,
                        conversation_record.id if conversation_record else None,
                    )

                room_id, conversation_id = conversation_cache[cache_key]
                enriched_sessions.append(
                    session_info.model_copy(
                        update={
                            "room_id": room_id,
                            "conversation_id": conversation_id,
                        }
                    )
                )

            return enriched_sessions


session_context_resolver = SessionContextResolver()
