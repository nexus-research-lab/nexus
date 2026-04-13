# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_manager.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""SDK 会话管理器。"""

import asyncio
from pathlib import Path
from typing import Any, Dict, Optional

from claude_agent_sdk import CanUseTool, ClaudeAgentOptions, ClaudeSDKClient

from agent.infra.server.common.base_exception import ServerException
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

    def __init__(self):
        self._sessions: Dict[str, ClaudeSDKClient] = {}
        self._locks: Dict[str, asyncio.Lock] = {}
        self._key_sdk_map: Dict[str, str] = {}
        self._sdk_key_map: Dict[str, str] = {}

    async def get_session(self, session_key: str) -> Optional[ClaudeSDKClient]:
        """获取现有 SDK client。"""
        client = self._sessions.get(session_key)
        if client is None:
            return None

        health_issue = self._inspect_client_health_issue(client)
        if not health_issue:
            return client

        # 中文注释：Claude CLI 子进程退出后，旧 client 会继续留在内存里。
        # 如果这里不主动淘汰，后续消息会反复命中同一个坏会话。
        self.invalidate_session(
            session_key,
            reason=f"检测到失效的 SDK 会话: {health_issue}",
        )
        return None

    async def create_session(
            self,
            session_key: str,
            can_use_tool: Optional[CanUseTool],
            session_id: Optional[str] = None,
            session_options: Optional[Dict[str, Any]] = None,
    ) -> ClaudeSDKClient:
        """创建新会话或返回现有会话。"""
        if session_key in self._sessions:
            logger.info(f"🔄 返回现有会话: {session_key}")
            return self._sessions[session_key]

        options = ClaudeAgentOptions(can_use_tool=can_use_tool, **(session_options or {}))
        if session_id:
            options.resume = session_id
            logger.info(f"🔄 恢复历史会话: key={session_key}, sdk_session={session_id}")
        else:
            logger.info(f"✨ 创建新会话: key={session_key}")

        cwd = Path(options.cwd)
        if not cwd.is_dir():
            raise ServerException(f"指定的cwd路径不存在: {cwd}")
        options.cwd = cwd.absolute().as_posix()

        client = ClaudeSDKClient(options=options)
        self._sessions[session_key] = client
        self._locks[session_key] = asyncio.Lock()
        logger.info(f"✅ 创建SDK client: key={session_key}")
        return client

    def get_lock(self, session_key: str) -> asyncio.Lock:
        """获取会话锁。"""
        if session_key not in self._locks:
            self._locks[session_key] = asyncio.Lock()
        return self._locks[session_key]

    async def update_session_options(self, session_key: str) -> bool:
        """刷新会话配置，淘汰旧 client。"""
        if session_key not in self._sessions:
            logger.info(f"❌ 会话不存在于内存中: {session_key}")
            return True

        async with self.get_lock(session_key):
            # Claude SDK client 内部使用 anyio cancel scope。
            # 跨任务直接 disconnect 容易触发 “Attempted to exit cancel scope in a different task”。
            # 这里改为仅淘汰内存缓存，让当前轮次自然结束，下一次请求再按最新配置重建 client。
            self.remove_session(session_key)
            logger.info(f"✅ 会话选项已更新，旧 client 已淘汰: {session_key}")
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
            updated = await self.update_session_options(session_key)
            if updated:
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

        # 兼容尚未完成持久化的运行态会话，避免删除工作区后仍有旧 client 残留。
        for session_key in list(self._sessions.keys()):
            parsed = parse_session_key(session_key)
            if parsed.get("agent_id") == agent_id:
                target_keys.add(session_key)

        removed_count = 0
        for session_key in target_keys:
            removed = await self.close_session(session_key)
            if removed:
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

    def get_session_id(self, session_key: str) -> Optional[str]:
        """根据 session_key 获取 SDK session_id。"""
        return self._key_sdk_map.get(session_key)

    def get_session_key(self, session_id: str) -> Optional[str]:
        """根据 SDK session_id 获取 session_key。"""
        return self._sdk_key_map.get(session_id)

    async def close_session(self, session_key: str) -> bool:
        """关闭并移除会话。"""
        client = self._sessions.get(session_key)
        if client:
            try:
                # 关闭会话时优先请求中断生成，避免跨任务强制 disconnect 带来的 anyio 异常。
                await client.interrupt()
                await client.disconnect()
                logger.info(f"⏸️ 已中断 SDK 会话: {session_key}")
            except Exception as exc:
                logger.warning(f"⚠️ 中断 SDK 会话失败: key={session_key}, error={exc}")

        existed = (
                session_key in self._sessions
                or session_key in self._locks
                or session_key in self._key_sdk_map
        )
        self.remove_session(session_key)
        return existed

    def invalidate_session(self, session_key: str, reason: str | None = None) -> bool:
        """淘汰已损坏的会话缓存，不再尝试复用。"""
        existed = (
                session_key in self._sessions
                or session_key in self._key_sdk_map
        )
        if reason:
            logger.warning(f"⚠️ 淘汰失效 SDK 会话: key={session_key}, reason={reason}")
        self._drop_session_runtime(session_key, preserve_lock=True)
        return existed

    def remove_session(self, session_key: str) -> None:
        """移除会话。"""
        self._drop_session_runtime(session_key, preserve_lock=False)
        logger.info(f"✅ 已移除 session: {session_key}")

    @staticmethod
    def _inspect_client_health_issue(client: ClaudeSDKClient) -> str | None:
        """检测 SDK client 是否仍可安全复用。"""
        query = getattr(client, "_query", None)
        if query is not None and getattr(query, "_closed", False):
            return "query 已关闭"

        transport = getattr(client, "_transport", None)
        if transport is None:
            return None

        exit_error = getattr(transport, "_exit_error", None)
        if exit_error is not None:
            return f"transport 已记录退出错误: {exit_error}"

        process = getattr(transport, "_process", None)
        if process is not None:
            return_code = getattr(process, "returncode", None)
            if return_code is not None:
                return f"Claude CLI 子进程已退出，exit_code={return_code}"

        is_ready = getattr(transport, "is_ready", None)
        if callable(is_ready) and process is not None:
            try:
                if not bool(is_ready()):
                    return "transport 未处于可写状态"
            except Exception as exc:
                return f"transport 健康检查失败: {exc}"

        return None

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


session_manager = SessionManager()
