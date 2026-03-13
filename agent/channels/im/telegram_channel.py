# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：telegram_channel.py
# @Date   ：2026/2/25 15:55
# @Author ：leemysw
#
# 2026/2/25 15:55   Create
# =====================================================

"""
Telegram 通道实现

[INPUT]: 依赖 python-telegram-bot 的 Application/Update/MessageHandler,
         依赖 message_sender.py / message_channel.py 抽象协议,
         依赖 tool_guard.py 的 AutoAllowPermissionStrategy,
         依赖 service 层的 ChatService
[OUTPUT]: 对外提供 TelegramChannel
[POS]: channel 模块的 Telegram 实现，独立于 WebSocket/Discord 通道运行
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
import uuid
from typing import Optional, Set

from telegram import Update
from telegram.ext import Application, ContextTypes, filters, MessageHandler

from agent.channels.message_channel import MessageChannel
from agent.channels.im.telegram_sender import TelegramSender
from agent.config.config import settings
from agent.infra.permission.permission_auto import AutoAllowPermissionStrategy
from agent.infra.session.session_router import build_session_key
from agent.utils.logger import logger


# =====================================================
# TelegramChannel — Telegram 通道
#
# 管理 python-telegram-bot 的生命周期，
# 监听消息并路由到 ChatService。
# =====================================================

class TelegramChannel(MessageChannel):
    """Telegram 通道"""

    def __init__(
            self,
            bot_token: str,
            allowed_user_ids: Optional[Set[int]] = None,
            allowed_tool_names: Optional[Set[str]] = None,
    ):
        self._bot_token = bot_token
        self._allowed_user_ids = allowed_user_ids
        self._permission_strategy = AutoAllowPermissionStrategy(allowed_tool_names)
        self._app: Optional[Application] = None
        self._run_task: Optional[asyncio.Task] = None

    @property
    def channel_type(self) -> str:
        return "telegram"

    async def _handle_message(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        """处理 Telegram 消息"""
        if not update.message or not update.message.text:
            return

        user = update.effective_user
        chat_id = update.effective_chat.id

        # 用户白名单检查
        if self._allowed_user_ids and user.id not in self._allowed_user_ids:
            logger.debug(f"🚫 Telegram 用户不在白名单: {user.id}")
            return

        content = update.message.text.strip()
        if not content:
            return

        # 构建 session_key
        is_private = update.effective_chat.type == "private"
        if is_private:
            session_key = build_session_key(
                channel="tg",
                chat_type="dm",
                ref=str(user.id),
                agent_id=settings.DEFAULT_AGENT_ID,
            )
        else:
            # 群组/Topic 支持
            thread_id = str(update.message.message_thread_id) if update.message.message_thread_id else None
            session_key = build_session_key(
                channel="tg",
                chat_type="group",
                ref=str(chat_id),
                thread_id=thread_id,
                agent_id=settings.DEFAULT_AGENT_ID,
            )

        logger.info(f"📨 Telegram 消息: user={user.username}, chat_id={chat_id}, key={session_key}")

        # 发送「正在输入」状态
        await context.bot.send_chat_action(chat_id=chat_id, action="typing")

        await self._process_message(session_key, content, chat_id)

    async def _process_message(self, session_key: str, content: str, chat_id: int) -> None:
        """使用 ChatService 处理消息。"""
        from agent.service.chat.chat_service import ChatService

        sender = TelegramSender(chat_id, self._app)
        chat_service = ChatService(sender, self._permission_strategy)

        chat_message = {
            "session_key": session_key,
            "content": content,
            "round_id": str(uuid.uuid4()),
        }

        try:
            await chat_service.handle_chat_message(chat_message)
        except Exception as e:
            logger.error(f"❌ Telegram 消息处理失败: {e}")
            await self._app.bot.send_message(
                chat_id=chat_id,
                text=f"⚠️ 处理失败: {str(e)[:500]}",
            )

    async def start(self) -> None:
        """启动 Telegram Bot"""
        if not self._bot_token:
            logger.warning("⚠️ Telegram Bot Token 未配置，跳过启动")
            return

        self._app = Application.builder().token(self._bot_token).build()

        # 注册消息处理器（接收所有文本消息）
        self._app.add_handler(
            MessageHandler(filters.TEXT & ~filters.COMMAND, self._handle_message)
        )

        # 在后台运行 polling
        self._run_task = asyncio.create_task(self._run_polling())
        logger.info("📡 Telegram 通道启动中...")

    async def _run_polling(self) -> None:
        """在后台运行 Telegram polling"""
        try:
            await self._app.initialize()
            await self._app.start()
            await self._app.updater.start_polling()
            logger.info("✅ Telegram Bot polling 已启动")

            # 保持运行
            while True:
                await asyncio.sleep(1)
        except asyncio.CancelledError:
            pass
        except Exception as e:
            logger.error(f"❌ Telegram Bot 运行失败: {e}")
        finally:
            try:
                await self._app.updater.stop()
                await self._app.stop()
                await self._app.shutdown()
            except Exception:
                pass

    async def stop(self) -> None:
        """停止 Telegram Bot"""
        if self._run_task:
            self._run_task.cancel()
            try:
                await self._run_task
            except asyncio.CancelledError:
                pass
        logger.info("🛑 Telegram 通道已停止")
