# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_manager.py
# @Date   ：2025/11/27 15:33
# @Author ：leemysw
#
# 2025/11/27 15:33   Create
# 2026/2/25          重构：session_key 路由
# =====================================================

"""
SDK 会话管理器

[INPUT]: 依赖 claude_agent_sdk 的 ClaudeSDKClient/ClaudeAgentOptions,
         依赖 session_store 的会话存取
[OUTPUT]: 对外提供 SessionManager（SDK client 生命周期管理）
[POS]: service 层的 SDK 会话管理，被 ChatHandler 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
from pathlib import Path
from typing import Any, Dict, Optional

from claude_agent_sdk import CanUseTool, ClaudeAgentOptions, ClaudeSDKClient

from agent.service.session_store import session_store
from agent.shared.server.common.base_exception import ServerException
from agent.utils.logger import logger


class SessionManager:
    """管理活跃的 ClaudeSDKClient 会话（以 session_key 为索引）"""

    def __init__(self):
        self._sessions: Dict[str, ClaudeSDKClient] = {}  # session_key → client
        self._locks: Dict[str, asyncio.Lock] = {}

        # SDK session ID 映射 (session_key ↔ sdk_id)
        self._key_sdk_map: Dict[str, str] = {}  # session_key → sdk_id
        self._sdk_key_map: Dict[str, str] = {}  # sdk_id → session_key

    async def get_session(self, session_key: str) -> Optional[ClaudeSDKClient]:
        """获取现有 SDK client"""
        return self._sessions.get(session_key)

    async def create_session(
            self,
            session_key: str,
            can_use_tool: Optional[CanUseTool],
            session_id: Optional[str] = None,
            session_options: Optional[Dict[str, Any]] = None,
    ) -> ClaudeSDKClient:
        """创建新会话或返回现有会话"""
        if session_key in self._sessions:
            logger.info(f"🔄 返回现有会话: {session_key}")
            return self._sessions[session_key]

        # 构建 options
        if session_options:
            options = ClaudeAgentOptions(can_use_tool=can_use_tool, **session_options)
        else:
            options = ClaudeAgentOptions(can_use_tool=can_use_tool)

        # resume
        if session_id:
            options.resume = session_id
            logger.info(f"🔄 恢复历史会话: key={session_key}, sdk_session={session_id}")
        else:
            logger.info(f"✨ 创建新会话: key={session_key}")

        # 验证 cwd
        cwd = Path(options.cwd)
        if not cwd.is_dir():
            raise ServerException(f"指定的cwd路径不存在: {cwd}")
        options.cwd = cwd.absolute().as_posix()

        try:
            client = ClaudeSDKClient(options=options)
            self._sessions[session_key] = client
            self._locks[session_key] = asyncio.Lock()

            logger.info(f"✅ 创建SDK client: key={session_key}")
            return client

        except Exception as e:
            logger.error(f"❌ 创建会话失败 {session_key}: {e}")
            raise

    def get_lock(self, session_key: str) -> asyncio.Lock:
        """获取会话锁"""
        if session_key not in self._locks:
            self._locks[session_key] = asyncio.Lock()
        return self._locks[session_key]

    async def update_session_options(self, session_key: str) -> bool:
        """更新会话配置（销毁旧 client，下次发消息时懒加载）"""
        if session_key not in self._sessions:
            logger.info(f"❌ 会话不存在于内存中: {session_key}")
            return True

        async with self.get_lock(session_key):
            try:
                old_client = self._sessions.get(session_key)
                try:
                    await old_client.disconnect()
                    logger.info(f"🔌 断开旧SDK连接: {session_key}")
                except Exception as e:
                    logger.warning(f"⚠️ 断开旧连接时出错: {e}")

                del self._sessions[session_key]
                logger.info(f"✅ 会话选项已更新，client 已重置: {session_key}")
                return True

            except Exception as e:
                logger.error(f"❌ 更新会话选项失败 {session_key}: {e}")
                return False

    async def refresh_agent_sessions(self, agent_id: str) -> int:
        """刷新指定 Agent 的活跃会话。

        Agent 配置或 workspace 发生变化后，销毁内存中的 SDK client，
        让后续消息按最新 Agent 配置懒加载重建。
        """
        sessions = await session_store.get_all_sessions()
        target_keys = [
            session.session_key
            for session in sessions
            if session.agent_id == agent_id and session.session_key in self._sessions
        ]

        refreshed_count = 0
        for session_key in target_keys:
            updated = await self.update_session_options(session_key)
            if updated:
                refreshed_count += 1

        logger.info(f"🔄 Agent 活跃会话刷新完成: agent={agent_id}, count={refreshed_count}")
        return refreshed_count

    async def register_sdk_session(self, session_key: str, session_id: str) -> None:
        """注册 session_key 与 SDK session_id 的映射"""
        self._key_sdk_map[session_key] = session_id
        self._sdk_key_map[session_id] = session_key

        try:
            await session_store.update_session(session_key=session_key, session_id=session_id)
            logger.info(f"💾 会话映射已记录: {session_key} ↔ {session_id}")
        except Exception as db_error:
            logger.warning(f"⚠️ 会话映射记录失败: {db_error}")

    def get_session_id(self, session_key: str) -> Optional[str]:
        return self._key_sdk_map.get(session_key)

    def get_session_key(self, session_id: str) -> Optional[str]:
        return self._sdk_key_map.get(session_id)

    def remove_session(self, session_key: str) -> None:
        """移除会话"""
        if session_key in self._sessions:
            del self._sessions[session_key]
            logger.debug(f"🗑️ 已移除 client: {session_key}")

        if session_key in self._locks:
            del self._locks[session_key]

        sdk_id = self._key_sdk_map.pop(session_key, None)
        if sdk_id:
            self._sdk_key_map.pop(sdk_id, None)

        logger.info(f"✅ 已移除 session: {session_key}")


# Global instance
session_manager = SessionManager()
