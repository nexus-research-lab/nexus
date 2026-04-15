# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_message_store.py
# @Date   ：2026/03/31 14:20
# @Author ：leemysw
# 2026/03/31 14:20   Create
# =====================================================

"""Room 消息存储。"""

from __future__ import annotations

from datetime import datetime
from pathlib import Path
from threading import Lock
from typing import Any, Optional

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.conversation_sql_repository import (
    ConversationSqlRepository,
)
from agent.infra.database.repositories.message_sql_repository import MessageSqlRepository
from agent.infra.database.repositories.session_sql_repository import SessionSqlRepository
from agent.infra.file_store.json_store import JsonFileStore
from agent.infra.file_store.storage_bootstrap import FileStorageBootstrap
from agent.infra.file_store.storage_paths import FileStoragePaths
from agent.schema.model_chat_persistence import MessageRecord
from agent.schema.model_message import Message, parse_message
from agent.service.room.room_message_mapper import (
    build_preview,
    infer_kind,
    infer_round_status,
    infer_sender_type,
    infer_status,
)
from agent.service.room.room_round_store import room_round_store
from agent.service.room.room_session_keys import (
    parse_room_conversation_id,
)
from agent.service.session.round_snapshot_normalizer import (
    RoundSnapshotNormalizer,
)
from agent.service.session.cost_repository import cost_repository
from agent.service.session.session_manager import session_manager


class RoomMessageStore:
    """负责 Room 消息的 JSONL 正文与 SQL 索引。"""

    def __init__(self) -> None:
        self._bootstrap = FileStorageBootstrap()
        self._paths = FileStoragePaths()
        self._db = get_db("async_sqlite")
        self._lock = Lock()
        self._snapshot_normalizer = RoundSnapshotNormalizer()
        self._bootstrap.ensure_ready()

    def _get_conversation_dir(self, conversation_id: str) -> Path:
        """返回 Room 对话的持久化目录。"""
        encoded = self._paths.build_session_dir_name(conversation_id)
        directory = self._paths.home_root / "rooms" / encoded
        directory.mkdir(parents=True, exist_ok=True)
        return directory

    def _get_conversation_log_path(self, conversation_id: str) -> Path:
        """返回 Room 对话的消息日志路径。"""
        return self._get_conversation_dir(conversation_id) / "messages.jsonl"

    def _load_raw_rows(self, conversation_id: str) -> list[dict[str, Any]]:
        """读取原始消息行。"""
        return JsonFileStore.read_jsonl(self._get_conversation_log_path(conversation_id))

    def _load_compacted_rows(self, conversation_id: str) -> list[dict[str, Any]]:
        """读取压缩后的消息快照。"""
        return self._bootstrap.compact_messages(self._load_raw_rows(conversation_id))

    @staticmethod
    def _to_created_at(timestamp_ms: int) -> datetime:
        """把毫秒时间戳转换为数据库使用的 naive 时间。"""
        return datetime.fromtimestamp(timestamp_ms / 1000)

    @staticmethod
    def _resolve_parent_message_status(message: Message) -> Optional[str]:
        """根据 result 终态推导占位消息状态。"""
        if message.role != "result":
            return None
        if message.subtype == "success":
            return "completed"
        if message.subtype == "interrupted":
            return "cancelled"
        if message.subtype == "error" or message.is_error:
            return "error"
        return None

    async def save_message(
        self,
        message: Message,
        room_session_id: Optional[str] = None,
        cost_session_key: Optional[str] = None,
    ) -> None:
        """保存 Room 消息。"""
        conversation_id = parse_room_conversation_id(message.session_key or "")
        if not conversation_id:
            raise ValueError(f"非法 Room session_key: {message.session_key}")

        created_at = self._to_created_at(message.timestamp)
        log_path = self._get_conversation_log_path(conversation_id)
        payload = message.model_dump(mode="json", exclude_none=True)

        with self._lock:
            JsonFileStore.append_jsonl(log_path, payload)

        async with self._db.session() as session:
            message_repository = MessageSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            await message_repository.upsert_message(
                MessageRecord(
                    id=message.message_id,
                    conversation_id=conversation_id,
                    session_id=room_session_id,
                    sender_type=infer_sender_type(message),
                    sender_user_id=None if message.role != "user" else "user",
                    sender_agent_id=message.agent_id or None,
                    kind=infer_kind(message),
                    status=infer_status(message),
                    content_preview=build_preview(message),
                    jsonl_path=str(log_path),
                    jsonl_offset=None,
                    round_id=message.round_id,
                    created_at=created_at,
                    updated_at=created_at,
                )
            )
            parent_status = self._resolve_parent_message_status(message)
            if parent_status and message.parent_id:
                await message_repository.update_message_status(
                    message_id=message.parent_id,
                    status=parent_status,
                    updated_at=created_at,
                )
            await conversation_repository.touch(conversation_id, touched_at=created_at)
            if room_session_id:
                await session_repository.touch(room_session_id, last_activity_at=created_at)
            await session.commit()

        if message.role != "result":
            return
        if cost_session_key:
            await cost_repository.record_result_message(message.model_copy(update={"session_key": cost_session_key}))
        if room_session_id and message.round_id:
            await room_round_store.finish_round(
                session_id=room_session_id,
                round_id=message.round_id,
                status=infer_round_status(message),
                finished_at_ms=message.timestamp,
                metadata={
                    "message_id": message.message_id,
                    "duration_ms": message.duration_ms,
                    "duration_api_ms": message.duration_api_ms,
                    "total_cost_usd": message.total_cost_usd,
                    "subtype": message.subtype,
                    "is_error": message.is_error,
                },
            )

    async def create_pending_message(
        self,
        message_id: str,
        session_key: str,
        agent_id: str,
        round_id: str,
        room_session_id: Optional[str] = None,
    ) -> None:
        """为流式回复创建 pending 占位索引。"""
        conversation_id = parse_room_conversation_id(session_key or "")
        if not conversation_id:
            raise ValueError(f"非法 Room session_key: {session_key}")

        created_at = datetime.now()
        log_path = self._get_conversation_log_path(conversation_id)
        async with self._db.session() as session:
            repository = MessageSqlRepository(session)
            await repository.upsert_message(
                MessageRecord(
                    id=message_id,
                    conversation_id=conversation_id,
                    session_id=room_session_id,
                    sender_type="agent",
                    sender_agent_id=agent_id,
                    kind="text",
                    status="pending",
                    content_preview=None,
                    jsonl_path=str(log_path),
                    jsonl_offset=None,
                    round_id=round_id,
                    created_at=created_at,
                    updated_at=created_at,
                )
            )
            await session.commit()

    async def mark_message_status(self, message_id: str, status: str) -> None:
        """更新消息状态，不改动既有正文索引。"""
        async with self._db.session() as session:
            repository = MessageSqlRepository(session)
            await repository.update_message_status(
                message_id=message_id,
                status=status,
                updated_at=datetime.now(),
            )
            await session.commit()

    async def register_sdk_session(
        self,
        room_session_id: str,
        sdk_session_key: str,
        sdk_session_id: str,
    ) -> None:
        """记录 Room 成员的 SDK session_id。"""
        await session_manager.register_session_mapping(
            session_key=sdk_session_key,
            session_id=sdk_session_id,
        )
        async with self._db.session() as session:
            repository = SessionSqlRepository(session)
            await repository.update_sdk_session_id(
                room_session_id=room_session_id,
                sdk_session_id=sdk_session_id,
            )
            await session.commit()

    async def get_messages(self, session_key: str) -> list[Message]:
        """读取 Room 历史消息。"""
        conversation_id = parse_room_conversation_id(session_key)
        if not conversation_id:
            return []

        compacted_rows = self._load_compacted_rows(conversation_id)
        round_status = self._snapshot_normalizer.build_round_status(compacted_rows)
        normalized_rows = self._snapshot_normalizer.normalize_assistant_rows(
            compacted_rows,
            round_status,
        )
        messages: list[Message] = []
        for row in normalized_rows:
            try:
                messages.append(parse_message(row))
            except Exception:
                continue
        return messages

    async def get_active_slots(self, conversation_id: str) -> list[dict[str, Any]]:
        """读取当前对话内仍在执行的 slot。"""
        if not conversation_id:
            return []

        async with self._db.session() as session:
            repository = MessageSqlRepository(session)
            records = await repository.list_inflight_by_conversation(conversation_id)

        active_slots: list[dict[str, Any]] = []
        for record in records:
            if not record.sender_agent_id or not record.round_id:
                continue
            timestamp = record.updated_at or record.created_at
            active_slots.append({
                "agent_id": record.sender_agent_id,
                "msg_id": record.id,
                "round_id": record.round_id,
                "status": record.status,
                "timestamp": int(timestamp.timestamp() * 1000) if timestamp else 0,
            })
        return active_slots

    async def get_latest_round_id(self, session_key: str) -> Optional[str]:
        """获取最新 round_id。"""
        conversation_id = parse_room_conversation_id(session_key)
        if not conversation_id:
            return None
        compacted_rows = self._load_compacted_rows(conversation_id)
        if not compacted_rows:
            return None
        return str(compacted_rows[-1].get("round_id") or "") or None

    async def has_round_result(self, session_key: str, round_id: str) -> bool:
        """判断某轮是否已有 result。"""
        conversation_id = parse_room_conversation_id(session_key)
        if not conversation_id:
            return False
        return any(
            str(row.get("round_id") or "") == round_id and row.get("role") == "result"
            for row in self._load_compacted_rows(conversation_id)
        )

    async def delete_conversation(self, conversation_id: str) -> None:
        """删除 Room 对话的 JSONL 正文。"""
        encoded = self._paths.build_session_dir_name(conversation_id)
        conversation_dir = self._paths.home_root / "rooms" / encoded
        if conversation_dir.exists():
            for child in conversation_dir.iterdir():
                if child.is_file():
                    child.unlink(missing_ok=True)
            conversation_dir.rmdir()


room_message_store = RoomMessageStore()
