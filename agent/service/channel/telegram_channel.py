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
         依赖 channel.py 的 MessageSender/MessageChannel,
         依赖 discord_channel.py 的 AutoAllowPermissionStrategy（复用）,
         依赖 handler 层的 ChatHandler
[OUTPUT]: 对外提供 TelegramChannel/TelegramSender
[POS]: channel 模块的 Telegram 实现，独立于 WebSocket/Discord 通道运行
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
import uuid
from typing import List, Optional, Set

from telegram import Update
from telegram.ext import Application, ContextTypes, filters, MessageHandler

from agent.config.config import settings
from agent.service.channel.channel import MessageChannel, MessageSender
from agent.service.channel.discord_channel import AutoAllowPermissionStrategy
from agent.service.schema.model_message import AError, AEvent, AMessage
from agent.service.session.session_router import build_session_key
from agent.utils.logger import logger

# =====================================================
# 常量
# =====================================================

TELEGRAM_MAX_LENGTH = 4096  # Telegram 单条消息字符上限


# =====================================================
# TelegramSender — Telegram 消息发送器
#
# 将 AMessage 转换为 Telegram 文本消息。
# stream/system 类型跳过，超长消息自动分片。
# =====================================================

class TelegramSender(MessageSender):
    """Telegram 消息发送器"""

    def __init__(self, chat_id: int, application: Application):
        self._chat_id = chat_id
        self._app = application

    async def send_message(self, message: AMessage) -> None:
        """发送 Agent 消息到 Telegram"""
        if message.message_type in ("stream", "system"):
            return

        text = self._extract_text(message)
        if not text:
            return

        if not text or message.message_type == "result":
            return

        for chunk in self._split_message(text):
            await self._app.bot.send_message(
                chat_id=self._chat_id,
                text=chunk,
                parse_mode="Markdown",
            )

    async def send_event(self, event: AEvent) -> None:
        """事件消息不推送到 Telegram"""
        pass

    async def send_error(self, error: AError) -> None:
        """发送错误到 Telegram"""
        text = f"⚠️ Error: {error.message}"
        await self._app.bot.send_message(
            chat_id=self._chat_id,
            text=text[:TELEGRAM_MAX_LENGTH],
        )

    @staticmethod
    def _extract_text(message: AMessage) -> Optional[str]:
        """从 AMessage 提取可读文本"""
        msg = message.message

        if message.message_type == "assistant" and message.block_type == "text":
            if hasattr(msg, "content") and msg.content:
                block = msg.content[0]
                if hasattr(block, "text"):
                    return block.text
            return None

        if message.message_type == "result":
            if hasattr(msg, "result") and msg.result:
                return f"✅ {msg.result}"
            return None

        return None

    @staticmethod
    def _split_message(text: str) -> List[str]:
        """将超长文本分片"""
        if len(text) <= TELEGRAM_MAX_LENGTH:
            return [text]
        chunks = []
        while text:
            chunks.append(text[:TELEGRAM_MAX_LENGTH])
            text = text[TELEGRAM_MAX_LENGTH:]
        return chunks


# =====================================================
# TelegramChannel — Telegram 通道
#
# 管理 python-telegram-bot 的生命周期，
# 监听消息并路由到 ChatHandler。
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
        """使用 ChatHandler 处理消息"""
        from agent.service.handler.chat_handler import ChatHandler

        sender = TelegramSender(chat_id, self._app)
        handler = ChatHandler(sender, self._permission_strategy)

        chat_message = {
            "session_key": session_key,
            "content": content,
            "round_id": str(uuid.uuid4()),
        }

        try:
            await handler.handle_chat_message(chat_message)
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
