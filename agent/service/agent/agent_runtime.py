# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_runtime.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""Claude SDK Client 运行时封装。"""

import os
from typing import Any

from claude_agent_sdk import ClaudeSDKClient, PermissionResult, ToolPermissionContext

from agent.service.session.session_manager import session_manager
from agent.service.permission.strategy.permission_strategy import PermissionStrategy
from agent.service.agent.agent_manager import agent_manager
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class AgentRuntime:
    """负责按 session_key 获取或初始化 Claude SDK client。"""

    @staticmethod
    def _build_connect_error_message(exc: Exception, stderr_lines: list[str]) -> str:
        """拼接连接异常和最近的 CLI stderr，便于直接定位失败原因。"""
        error_message = str(exc).strip() or exc.__class__.__name__
        recent_stderr_lines = [
            line.strip()
            for line in stderr_lines[-3:]
            if isinstance(line, str) and line.strip()
        ]
        if not recent_stderr_lines:
            return error_message
        return f"{error_message}; stderr={' | '.join(recent_stderr_lines)}"

    async def get_or_create_client(
        self,
        session_key: str,
        agent_id: str,
        permission_strategy: PermissionStrategy,
        resume_session_id: str | None = None,
        resolved_agent_id: str | None = None,
        force_fresh: bool = False,
    ) -> ClaudeSDKClient:
        """按需获取或创建 SDK client。"""
        # Room 协作链路要求共享历史以我们自己的快照为准。
        # 因此当 force_fresh=True 时，必须淘汰旧 SDK client，避免隐式携带上一轮对话历史。
        if force_fresh:
            await session_manager.close_session(session_key)

        client = await session_manager.get_session(session_key)
        if client:
            logger.debug(f"♻️ 复用现有 session: {session_key}")
            return client

        existing_session = None
        if not force_fresh and (resume_session_id is None or resolved_agent_id is None):
            existing_session = await session_store.get_session_info(session_key)
        session_id = None if force_fresh else (
            resume_session_id or (existing_session.session_id if existing_session else None)
        )
        real_agent_id = resolved_agent_id or (existing_session.agent_id if existing_session else agent_id)

        try:
            sdk_options = await agent_manager.build_sdk_options(real_agent_id)
            logger.info(f"📋 SDK options 从 Agent 构建: agent={real_agent_id}")
        except Exception:
            logger.warning(f"⚠️ Agent 不存在: {real_agent_id}，使用默认配置")
            sdk_options = {"cwd": os.getcwd()}

        if session_id:
            sdk_options["resume"] = session_id

        stderr_lines: list[str] = []

        def handle_sdk_stderr(line: str) -> None:
            """把 Claude CLI stderr 直接打进服务日志，便于排查异常退出原因。"""
            normalized_line = str(line).strip()
            if normalized_line:
                stderr_lines.append(normalized_line)
            logger.warning(
                "⚠️ Claude CLI stderr: key=%s, agent=%s, line=%s",
                session_key,
                real_agent_id,
                normalized_line or line,
            )

        sdk_options.setdefault("stderr", handle_sdk_stderr)

        async def can_use_tool(
            name: str,
            data: dict[str, Any],
            context: ToolPermissionContext,
        ) -> PermissionResult:
            return await permission_strategy.request_permission(session_key, name, data, context)

        client = await session_manager.create_session(
            session_key=session_key,
            can_use_tool=can_use_tool,
            session_id=session_id,
            session_options=sdk_options,
        )
        try:
            await client.connect()
        except Exception as exc:
            connect_error_message = self._build_connect_error_message(exc, stderr_lines)
            if session_id and not force_fresh:
                logger.warning(
                    "⚠️ 恢复 SDK 会话失败，清空失效 session_id 后重建新会话: "
                    "key=%s, agent=%s, sdk_session=%s, error=%s",
                    session_key,
                    real_agent_id,
                    session_id,
                    connect_error_message,
                )
                await session_store.clear_session_id(session_key)
                session_manager.invalidate_session(
                    session_key,
                    reason=f"恢复 SDK 会话失败，准备降级重建: {connect_error_message}",
                )
                return await self.get_or_create_client(
                    session_key=session_key,
                    agent_id=agent_id,
                    permission_strategy=permission_strategy,
                    resume_session_id=None,
                    resolved_agent_id=real_agent_id,
                    force_fresh=True,
                )
            session_manager.invalidate_session(
                session_key,
                reason=f"SDK client 连接失败: {connect_error_message}",
            )
            raise RuntimeError(connect_error_message) from exc

        logger.info(f"✅ Client 就绪: key={session_key}, agent={real_agent_id}, session_id={session_id}")
        return client

    async def query_with_recovery(
        self,
        *,
        session_key: str,
        agent_id: str,
        permission_strategy: PermissionStrategy,
        prompt: str,
        client: ClaudeSDKClient | None = None,
        resume_session_id: str | None = None,
        resolved_agent_id: str | None = None,
        force_fresh: bool = False,
    ) -> ClaudeSDKClient:
        """对可恢复的会话失效错误执行一次自动重建并重试。"""
        active_client = client or await self.get_or_create_client(
            session_key=session_key,
            agent_id=agent_id,
            permission_strategy=permission_strategy,
            resume_session_id=resume_session_id,
            resolved_agent_id=resolved_agent_id,
            force_fresh=force_fresh,
        )

        has_retried = False
        while True:
            try:
                await active_client.query(prompt)
                return active_client
            except Exception as exc:
                if has_retried or not session_manager.is_recoverable_client_error(exc):
                    raise

                has_retried = True
                session_manager.invalidate_session(
                    session_key,
                    reason=f"查询前检测到 SDK 会话不可写，准备自动重建: {exc}",
                )
                active_client = await self.get_or_create_client(
                    session_key=session_key,
                    agent_id=agent_id,
                    permission_strategy=permission_strategy,
                    resume_session_id=resume_session_id,
                    resolved_agent_id=resolved_agent_id,
                    force_fresh=force_fresh,
                )


agent_runtime = AgentRuntime()
