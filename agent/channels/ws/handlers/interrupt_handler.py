#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：interrupt_handler.py
# @Date   ：2026/3/13 18:10
# @Author ：leemysw
# 2026/3/13 18:10   Create
# =====================================================

"""WebSocket 中断消息处理器。"""

import asyncio
import uuid
from typing import Any, Dict, Optional

from claude_agent_sdk.types import ResultMessage

from agent.channels.ws.handlers.base_handler import BaseHandler
from agent.infra.agent.session_manager import session_manager
from agent.schema.model_message import AMessage
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class InterruptHandler(BaseHandler):
    """中断消息处理器。"""

    async def handle_interrupt(self, message: Dict[str, Any], chat_tasks: Dict[str, asyncio.Task]) -> None:
        """处理中断消息。"""
        session_key = message.get("session_key") or message.get("agent_id", "")
        round_id = message.get("round_id")
        if not session_key:
            logger.warning("⚠️ interrupt 消息缺少 session_key")
            return

        asyncio.create_task(self._handle_interrupt_async(session_key, chat_tasks, round_id))

    async def _handle_interrupt_async(
        self,
        session_key: str,
        chat_tasks: Dict[str, asyncio.Task],
        round_id: Optional[str] = None,
    ) -> None:
        """异步执行中断流程。"""
        try:
            client = await session_manager.get_session(session_key)
            if client:
                await client.interrupt()
                logger.info(f"⏸️ 中断会话: key={session_key}")
            else:
                logger.warning(f"⚠️ 未找到会话 client: key={session_key}")
                return

            chat_task = chat_tasks.get(session_key)
            if chat_task and not chat_task.done():
                try:
                    await asyncio.wait_for(chat_task, timeout=10.0)
                    logger.info(f"✅ 任务自然结束: {session_key}")
                except asyncio.TimeoutError:
                    logger.info(f"🛑 强制取消任务: {session_key}")
                    chat_task.cancel()
                    try:
                        await chat_task
                    except asyncio.CancelledError:
                        pass
            elif chat_task and chat_task.done():
                logger.info(f"✅ 任务已结束: {session_key}")
            else:
                logger.warning(f"⚠️ 未找到任务: {session_key}")

            await self._send_interrupt_result(session_key, round_id)
        except Exception as exc:
            logger.error(f"❌ 中断处理失败: {exc}")

    async def _send_interrupt_result(self, session_key: str, round_id: Optional[str] = None) -> None:
        """发送中断结果消息。"""
        session_id = session_manager.get_session_id(session_key)
        session_info = await session_store.get_session_info(session_key)
        if not round_id:
            round_id = await session_store.get_latest_round_id(session_key)

        if not round_id:
            logger.warning(f"⚠️ 无法获取 round_id: key={session_key}")
            return

        if await session_store.has_round_result(session_key, round_id):
            logger.info(f"ℹ️ 跳过中断结果: round 已有 result, key={session_key}, round_id={round_id}")
            return

        result_message = AMessage(
            session_key=session_key,
            agent_id=session_info.agent_id if session_info else "main",
            round_id=round_id,
            session_id=session_id,
            message_id=str(uuid.uuid4()),
            message=ResultMessage(
                subtype="interrupted",
                duration_ms=0,
                duration_api_ms=0,
                is_error=True,
                num_turns=0,
                session_id=session_id,
                total_cost_usd=0,
                usage={
                    "input_tokens": 0,
                    "cache_creation_input_tokens": 0,
                    "cache_read_input_tokens": 0,
                    "output_tokens": 0,
                    "server_tool_use": {"web_search_requests": 0, "web_fetch_requests": 0},
                    "service_tier": "standard",
                    "cache_creation": {"ephemeral_1h_input_tokens": 0, "ephemeral_5m_input_tokens": 0},
                },
                result="用户中断",
            ),
            message_type="result",
        )

        await session_store.save_message(result_message)
        logger.info(f"💾 保存中断消息: key={session_key}, round_id={round_id}")
        await self.send(result_message)
