# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_manager.py
# @Date   ：2026/04/16 10:32
# @Author ：leemysw
# 2026/04/16 10:32   Create
# =====================================================

"""SDK 会话管理器。"""

import asyncio
from typing import Any

from claude_agent_sdk import CanUseTool, ClaudeSDKClient

from agent.service.session.session_client_runtime import SessionClientRuntime
from agent.service.session.session_router import parse_session_key
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class SessionManager:
    """管理活跃的 ClaudeSDKClient 会话。"""

    RECOVERABLE_CLIENT_ERROR_MARKERS = (
        "Cannot write to terminated process",
        "ProcessTransport is not ready for writing",
        "Cannot write to process that exited with error",
        "Not connected. Call connect() first.",
    )

    def __init__(self) -> None:
        self._sessions: dict[str, ClaudeSDKClient] = {}
        self._locks: dict[str, asyncio.Lock] = {}
        self._key_sdk_map: dict[str, str] = {}
        self._sdk_key_map: dict[str, str] = {}
        self._reconnect_sessions: set[str] = set()

    async def get_session(self, session_key: str) -> ClaudeSDKClient | None:
        """返回当前内存中的活跃 client，供中断链直接使用。"""
        return self._sessions.get(session_key)

    async def get_reusable_session(self, session_key: str) -> ClaudeSDKClient | None:
        """返回可安全复用的 SDK client。"""
        client = self._sessions.get(session_key)
        if client is None:
            return None

        health_issue = SessionClientRuntime.inspect_health_issue(client)
        if health_issue:
            self.invalidate_session(
                session_key,
                reason=f"检测到失效的 SDK 会话: {health_issue}",
            )
            return None

        if session_key in self._reconnect_sessions:
            logger.info(f"🔄 会话需要在下次请求前重连: {session_key}")
            return None
        return client

    async def create_session(
        self,
        session_key: str,
        can_use_tool: CanUseTool | None,
        session_id: str | None = None,
        session_options: dict[str, Any] | None = None,
    ) -> ClaudeSDKClient:
        """创建新会话或返回现有会话。"""
        if session_key in self._sessions:
            logger.info(f"🔄 返回现有会话: {session_key}")
            return self._sessions[session_key]

        options = SessionClientRuntime.build_client_options(
            can_use_tool,
            session_id,
            session_options,
        )
        if session_id:
            logger.info(f"🔄 恢复历史会话: key={session_key}, sdk_session={session_id}")
        else:
            logger.info(f"✨ 创建新会话: key={session_key}")

        client = ClaudeSDKClient(options=options)
        self._sessions[session_key] = client
        self._locks[session_key] = asyncio.Lock()
        logger.info(f"✅ 创建SDK client: key={session_key}")
        return client

    async def prepare_session_reconnect(
        self,
        session_key: str,
        can_use_tool: CanUseTool | None,
        session_id: str | None = None,
        session_options: dict[str, Any] | None = None,
    ) -> ClaudeSDKClient:
        """保留同一个 client 对象，切换到底层新连接。"""
        client = self._sessions.get(session_key)
        if client is None:
            return await self.create_session(
                session_key=session_key,
                can_use_tool=can_use_tool,
                session_id=session_id,
                session_options=session_options,
            )

        options = SessionClientRuntime.build_client_options(
            can_use_tool,
            session_id,
            session_options,
        )
        try:
            await client.disconnect()
        except Exception as exc:
            logger.warning(f"⚠️ 断开旧 SDK 会话失败: key={session_key}, error={exc}")
            await SessionClientRuntime.force_terminate_process(session_key, client)

        client.options = options
        self._reconnect_sessions.discard(session_key)
        logger.info(f"🔄 已更新 client 配置，等待重连: {session_key}")
        return client

    def get_lock(self, session_key: str) -> asyncio.Lock:
        """获取会话锁。"""
        if session_key not in self._locks:
            self._locks[session_key] = asyncio.Lock()
        return self._locks[session_key]

    async def update_session_options(self, session_key: str, agent_id: str) -> bool:
        """刷新会话配置，尽量原地同步可热更新字段。"""
        client = self._sessions.get(session_key)
        if client is None:
            logger.info(f"❌ 会话不存在于内存中: {session_key}")
            return False

        from agent.service.agent.agent_manager import agent_manager

        sdk_options = await agent_manager.build_sdk_options(agent_id)
        reconnect_required = SessionClientRuntime.requires_reconnect(client, sdk_options)
        target_permission_mode = sdk_options.get("permission_mode")
        target_model = sdk_options.get("model")

        if SessionClientRuntime.is_connected(client):
            reconnect_required = await SessionClientRuntime.apply_hot_updates(
                session_key=session_key,
                client=client,
                target_permission_mode=target_permission_mode,
                target_model=target_model,
                reconnect_required=reconnect_required,
            )

        if reconnect_required:
            self._reconnect_sessions.add(session_key)
            logger.info(f"🔄 会话已标记为待重连: key={session_key}")
        else:
            self._reconnect_sessions.discard(session_key)
            logger.info(f"✅ 会话配置已原地刷新: key={session_key}")
        return True

    async def refresh_agent_sessions(self, agent_id: str) -> int:
        """刷新指定 Agent 的活跃会话。"""
        sessions = await session_store.get_all_sessions()
        target_keys = {
            session.session_key
            for session in sessions
            if session.agent_id == agent_id and session.session_key in self._sessions
        }
        for session_key in list(self._sessions.keys()):
            parsed = parse_session_key(session_key)
            if parsed.get("agent_id") == agent_id:
                target_keys.add(session_key)

        refreshed_count = 0
        for session_key in target_keys:
            if await self.update_session_options(session_key, agent_id):
                refreshed_count += 1

        logger.info(f"🔄 Agent 活跃会话刷新完成: agent={agent_id}, count={refreshed_count}")
        return refreshed_count

    async def remove_agent_sessions(self, agent_id: str) -> int:
        """移除指定 Agent 的所有活跃会话。"""
        sessions = await session_store.get_all_sessions()
        target_keys = {
            session.session_key
            for session in sessions
            if session.agent_id == agent_id
        }
        for session_key in list(self._sessions.keys()):
            parsed = parse_session_key(session_key)
            if parsed.get("agent_id") == agent_id:
                target_keys.add(session_key)

        removed_count = 0
        for session_key in target_keys:
            if await self.close_session(session_key):
                removed_count += 1

        logger.info(f"🧹 Agent 活跃会话清理完成: agent={agent_id}, count={removed_count}")
        return removed_count

    async def register_session_mapping(self, session_key: str, session_id: str) -> None:
        """仅记录 session_key 与 SDK session_id 的内存映射。"""
        self._key_sdk_map[session_key] = session_id
        self._sdk_key_map[session_id] = session_key

    @classmethod
    def is_recoverable_client_error(cls, exc: Exception) -> bool:
        """判断异常是否属于可自动重建的会话失效错误。"""
        error_message = str(exc)
        return any(marker in error_message for marker in cls.RECOVERABLE_CLIENT_ERROR_MARKERS)

    async def register_sdk_session(self, session_key: str, session_id: str) -> None:
        """注册 session_key 与 SDK session_id 的映射。"""
        await self.register_session_mapping(session_key=session_key, session_id=session_id)
        try:
            await session_store.update_session(session_key=session_key, session_id=session_id)
            logger.info(f"💾 会话映射已记录: {session_key} ↔ {session_id}")
        except Exception as exc:
            logger.warning(f"⚠️ 会话映射记录失败: {exc}")

    def get_session_id(self, session_key: str) -> str | None:
        """根据 session_key 获取 SDK session_id。"""
        return self._key_sdk_map.get(session_key)

    def get_session_key(self, session_id: str) -> str | None:
        """根据 SDK session_id 获取 session_key。"""
        return self._sdk_key_map.get(session_id)

    def needs_reconnect(self, session_key: str) -> bool:
        """判断会话是否需要在下次请求前重连。"""
        return session_key in self._reconnect_sessions

    async def close_session(self, session_key: str) -> bool:
        """关闭并移除会话。"""
        client = self._sessions.get(session_key)
        if client:
            try:
                await client.interrupt()
                await client.disconnect()
                logger.info(f"⏸️ 已中断 SDK 会话: {session_key}")
            except Exception as exc:
                logger.warning(f"⚠️ 中断 SDK 会话失败: key={session_key}, error={exc}")
                await SessionClientRuntime.force_terminate_process(session_key, client)

        existed = (
            session_key in self._sessions
            or session_key in self._locks
            or session_key in self._key_sdk_map
        )
        self.remove_session(session_key)
        return existed

    def invalidate_session(self, session_key: str, reason: str | None = None) -> bool:
        """淘汰已损坏的会话缓存，不再尝试复用。"""
        existed = session_key in self._sessions or session_key in self._key_sdk_map
        if reason:
            logger.warning(f"⚠️ 淘汰失效 SDK 会话: key={session_key}, reason={reason}")
        self._drop_session_runtime(session_key, preserve_lock=True)
        return existed

    def remove_session(self, session_key: str) -> None:
        """移除会话。"""
        self._drop_session_runtime(session_key, preserve_lock=False)
        logger.info(f"✅ 已移除 session: {session_key}")

    def _drop_session_runtime(self, session_key: str, preserve_lock: bool) -> None:
        """删除会话运行态缓存，可选择保留并发锁。"""
        if session_key in self._sessions:
            del self._sessions[session_key]
            logger.debug(f"🗑️ 已移除 client: {session_key}")
        if not preserve_lock and session_key in self._locks:
            del self._locks[session_key]

        sdk_id = self._key_sdk_map.pop(session_key, None)
        if sdk_id:
            self._sdk_key_map.pop(sdk_id, None)
        self._reconnect_sessions.discard(session_key)


session_manager = SessionManager()
