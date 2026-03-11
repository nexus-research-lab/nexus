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
         依赖 channel.py 的 MessageSender/MessageChannel/PermissionStrategy,
         依赖 handler 层的 ChatHandler,
         依赖 session_manager/session_store 管理会话
[OUTPUT]: 对外提供 DiscordChannel/DiscordSender/AutoAllowPermissionStrategy
[POS]: channel 模块的 Discord 实现，独立于 WebSocket 通道运行
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
import re
import uuid
from typing import Any, Dict, List, Optional, Set

import discord
from claude_agent_sdk import PermissionResult, PermissionResultAllow, PermissionResultDeny, ToolPermissionContext

from agent.core.config import settings
from agent.service.channel.channel import MessageChannel, MessageSender, PermissionStrategy
from agent.service.schema.model_message import AError, AEvent, AMessage
from agent.service.session.session_router import build_session_key
from agent.utils.logger import logger

# =====================================================
# 常量
# =====================================================

DISCORD_MAX_LENGTH = 2000  # Discord 单条消息字符上限


# =====================================================
# AutoAllowPermissionStrategy — 自动允许权限策略
#
# 非交互通道（Discord/Telegram）共用。
# 白名单内的工具自动允许，其余拒绝。
# =====================================================

class AutoAllowPermissionStrategy(PermissionStrategy):
    """自动允许权限策略 — 非交互通道专用"""

    # 默认安全工具白名单
    DEFAULT_ALLOWED_TOOLS: Set[str] = {
        "Task",
        "TaskOutput",
        "Edit",
        "TodoWrite",
        "Read",
        "Bash",
        "KillShell",
        "Grep",
        "Glob",
        "LS",
        "Write",
        "Skill",
        "WebSearch",
        "WebFetch",
        "AskUserQuestion",
    }

    def __init__(self, allowed_tools: Optional[Set[str]] = None):
        self.allowed_tools = allowed_tools or self.DEFAULT_ALLOWED_TOOLS

    async def request_permission(
            self,
            agent_id: str,
            tool_name: str,
            input_data: dict[str, Any],
            context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """白名单工具自动允许，其余拒绝"""
        if tool_name in self.allowed_tools:
            logger.debug(f"✅ 自动允许工具: {tool_name} (agent={agent_id})")
            return PermissionResultAllow(updated_input=input_data)

        logger.info(f"🚫 自动拒绝工具: {tool_name} (agent={agent_id})")
        return PermissionResultDeny(
            message=f"Tool '{tool_name}' is not allowed in this channel"
        )


# =====================================================
# DiscordSender — Discord 消息发送器
#
# 将 AMessage 转换为纯文本发送到 Discord 频道。
# stream 类型消息跳过（Discord 不支持流式渲染）。
# 超长消息自动分片。
# =====================================================

class DiscordSender(MessageSender):
    """Discord 消息发送器"""

    def __init__(self, discord_channel: discord.abc.Messageable):
        self._channel = discord_channel
        self._last_sent_text: Optional[str] = None  # 防止重复发送

    async def send_message(self, message: AMessage) -> None:
        """发送 Agent 消息到 Discord"""
        # 跳过 stream 类型（Discord 无法渲染增量流）
        if message.message_type == "stream":
            return

        # 跳过 system 类型（内部元数据）
        if message.message_type == "system":
            return

        # 提取文本内容
        text = self._extract_text(message)
        if not text or message.message_type == "result":
            return

        # 防止重复发送相同内容
        if text == self._last_sent_text:
            logger.debug(f"🔄 跳过重复消息: {text[:50]}...")
            return
        self._last_sent_text = text

        # 分片发送
        for chunk in self._split_message(text):
            await self._channel.send(chunk)

    async def send_event(self, event: AEvent) -> None:
        """事件消息不推送到 Discord"""
        pass

    async def send_error(self, error: AError) -> None:
        """发送错误到 Discord"""
        text = f"⚠️ Error: {error.message}"
        await self._channel.send(text[:DISCORD_MAX_LENGTH])

    @staticmethod
    def _extract_text(message: AMessage) -> Optional[str]:
        """从 AMessage 提取可读文本"""
        msg = message.message

        # AssistantMessage — 提取 TextBlock 内容
        if message.message_type == "assistant" and message.block_type == "text":
            if hasattr(msg, "content") and msg.content:
                block = msg.content[0]
                if hasattr(block, "text"):
                    return block.text
            return None

        # ResultMessage — 提取结果
        if message.message_type == "result":
            if hasattr(msg, "result") and msg.result:
                return f"✅ {msg.result}"
            return None

        return None

    @staticmethod
    def _split_message(text: str) -> List[str]:
        """将超长文本分片为 Discord 可发送的长度"""
        if len(text) <= DISCORD_MAX_LENGTH:
            return [text]
        chunks = []
        while text:
            chunks.append(text[:DISCORD_MAX_LENGTH])
            text = text[DISCORD_MAX_LENGTH:]
        return chunks


# =====================================================
# DiscordChannel — Discord 通道
#
# 管理 discord.py Bot 的生命周期，监听消息，
# 为每条消息创建 ChatHandler 并调用 Agent。
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
        """使用 ChatHandler 处理消息"""
        from agent.service.handler.chat_handler import ChatHandler

        sender = DiscordSender(channel)
        handler = ChatHandler(sender, self._permission_strategy)

        chat_message = {
            "session_key": session_key,
            "content": content,
            "round_id": str(uuid.uuid4()),
        }

        try:
            await handler.handle_chat_message(chat_message)
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
