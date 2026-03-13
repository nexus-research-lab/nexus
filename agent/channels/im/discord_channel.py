# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：discord_channel.py
# @Date   ：2026/2/25 15:50
# @Author ：leemysw
#
# 2026/2/25 15:50   Create
# =====================================================

"""
Discord 通道实现

[INPUT]: 依赖 discord.py 的 Bot/Intents,
         依赖 message_sender.py / message_channel.py 抽象协议,
         依赖 service 层的 ChatService,
         依赖 session_manager/session_store 管理会话
[OUTPUT]: 对外提供 DiscordChannel
[POS]: channel 模块的 Discord 实现，独立于 WebSocket 通道运行
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
import re
import uuid
from typing import Dict, Optional, Set

import discord

from agent.channels.message_channel import MessageChannel
from agent.channels.im.discord_sender import DiscordSender
from agent.config.config import settings
from agent.infra.permission.permission_auto import AutoAllowPermissionStrategy
from agent.infra.session.session_router import build_session_key
from agent.utils.logger import logger


# =====================================================
# DiscordChannel — Discord 通道
#
# 管理 discord.py Bot 的生命周期，监听消息，
# 为每条消息创建 ChatService 并调用 Agent。
# =====================================================

class DiscordChannel(MessageChannel):
    """Discord 通道"""

    def __init__(
            self,
            bot_token: str,
            trigger_word: Optional[str] = None,
            allowed_guild_ids: Optional[Set[int]] = None,
            allowed_tool_names: Optional[Set[str]] = None,
    ):
        self._bot_token = bot_token
        self._trigger_word = trigger_word.lower()
        self._allowed_guild_ids = allowed_guild_ids
        self._permission_strategy = AutoAllowPermissionStrategy(allowed_tool_names)
        self._trigger_pattern = re.compile(
            re.escape(self._trigger_word), re.IGNORECASE
        )

        # discord.py Bot
        intents = discord.Intents.default()
        intents.message_content = True
        # 使用 loop 参数确保事件循环正确初始化
        self._bot = discord.Client(intents=intents)
        self._bot_task: Optional[asyncio.Task] = None

        # 活跃会话跟踪
        self._chat_tasks: Dict[str, asyncio.Task] = {}

        # 注册事件处理
        self._setup_events()

    @property
    def channel_type(self) -> str:
        return "discord"

    def _setup_events(self) -> None:
        """注册 discord.py 事件回调"""

        @self._bot.event
        async def on_ready():
            logger.info(f"🤖 Discord Bot 已就绪: {self._bot.user}")

        @self._bot.event
        async def on_message(message: discord.Message):
            logger.debug(f"🗨 Discord 收到消息: {message.author}, {message.content}")
            await self._handle_discord_message(message)

    async def _handle_discord_message(self, message: discord.Message) -> None:
        """处理 Discord 消息"""
        # 忽略 Bot 自身消息
        if message.author == self._bot.user:
            return

        # Guild 白名单检查
        if self._allowed_guild_ids and message.guild:
            if message.guild.id not in self._allowed_guild_ids:
                return

        # 触发词检查
        content = message.content.strip()
        if self._trigger_word:
            if not self._trigger_pattern.search(content):
                return

        # 移除触发词，提取用户消息
        user_content = self._trigger_pattern.sub("", message.content).strip()
        if not user_content:
            await message.channel.send("请输入你的问题 🤔")
            return

        # 构建 session_key
        is_dm = isinstance(message.channel, discord.DMChannel)
        if is_dm:
            session_key = build_session_key(
                channel="dg",
                chat_type="dm",
                ref=str(message.author.id),
                agent_id=settings.DEFAULT_AGENT_ID,
            )
        else:
            ref = f"{message.guild.id}:{message.channel.id}"
            # 线程支持（Phase 2 会完善）
            thread_id = str(message.channel.id) if isinstance(message.channel, discord.Thread) else None
            session_key = build_session_key(
                channel="dg",
                chat_type="group",
                ref=ref,
                thread_id=thread_id,
                agent_id=settings.DEFAULT_AGENT_ID,
            )

        logger.info(
            f"📨 Discord 消息: user={message.author}, channel={message.channel}, key={session_key}"
        )

        # 显示正在输入
        async with message.channel.typing():
            await self._process_message(session_key, user_content, message.channel)

    async def _process_message(
            self,
            session_key: str,
            content: str,
            channel: discord.abc.Messageable,
    ) -> None:
        """使用 ChatService 处理消息。"""
        from agent.service.chat.chat_service import ChatService

        sender = DiscordSender(channel)
        chat_service = ChatService(sender, self._permission_strategy)

        chat_message = {
            "session_key": session_key,
            "content": content,
            "round_id": str(uuid.uuid4()),
        }

        try:
            await chat_service.handle_chat_message(chat_message)
        except Exception as e:
            logger.error(f"❌ Discord 消息处理失败: {e}")
            await channel.send(f"⚠️ 处理失败: {str(e)[:500]}")

    async def start(self) -> None:
        """启动 Discord Bot（在后台任务中运行）"""
        if not self._bot_token:
            logger.warning("⚠️ Discord Bot Token 未配置，跳过启动")
            return

        self._bot_task = asyncio.create_task(self._run_bot())
        logger.info("📡 Discord 通道启动中...")

    async def _run_bot(self) -> None:
        """在后台运行 Bot"""
        try:
            await self._bot.start(self._bot_token)
        except Exception as e:
            logger.error(f"❌ Discord Bot 运行失败: {e}")

    async def stop(self) -> None:
        """停止 Discord Bot"""
        if self._bot and not self._bot.is_closed():
            await self._bot.close()
        if self._bot_task:
            self._bot_task.cancel()
            try:
                await self._bot_task
            except asyncio.CancelledError:
                pass
        logger.info("🛑 Discord 通道已停止")
