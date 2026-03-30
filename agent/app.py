# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：server
# @Date   ：2024/2/23 09:55
# @Author ：leemysw

# 2024/2/23 09:55   Create
# =====================================================

import gc
from contextlib import asynccontextmanager

from fastapi import FastAPI

from agent.api.router import api_router
from agent.config.config import settings
from agent.service.channels.channel_register import ChannelRegister
from agent.service.agent.agent_service import agent_service
from agent.infra.server.register import register_exception, register_hook, register_middleware
from agent.utils.logger import logger

# 全局通道管理器
channel_manager = ChannelRegister()


@asynccontextmanager
async def lifespan(app: FastAPI):
    try:
        logger.info("📁 使用 workspace 文件存储模式启动")

        # 显式初始化存储层（避免导入时产生文件系统副作用）
        from agent.storage.session_repository import session_repository
        from agent.storage.cost_repository import cost_repository
        session_repository.ensure_ready()
        cost_repository.ensure_ready()

        # 注册并启动消息通道
        await _register_channels()

        gc.collect()
        gc.freeze()

        yield

    finally:
        await channel_manager.stop_all()
        logger.info("Model shutdown complete.")


async def _register_channels() -> None:
    """按配置注册消息通道"""
    if settings.DISCORD_ENABLED:
        try:
            from agent.service.channels.im.discord_channel import DiscordChannel

            guild_ids = None
            if settings.DISCORD_ALLOWED_GUILDS:
                guild_ids = {int(g.strip()) for g in settings.DISCORD_ALLOWED_GUILDS.split(",") if g.strip()}

            discord_channel = DiscordChannel(
                bot_token=settings.DISCORD_BOT_TOKEN,
                trigger_word=settings.DISCORD_TRIGGER_WORD,
                allowed_guild_ids=guild_ids,
            )
            channel_manager.register(discord_channel)
        except ImportError:
            logger.warning("⚠️ discord.py 未安装，跳过 Discord 通道。安装: pip install discord.py")

    if settings.TELEGRAM_ENABLED:
        try:
            from agent.service.channels.im.telegram_channel import TelegramChannel

            user_ids = None
            if settings.TELEGRAM_ALLOWED_USERS:
                user_ids = {int(u.strip()) for u in settings.TELEGRAM_ALLOWED_USERS.split(",") if u.strip()}

            telegram_channel = TelegramChannel(
                bot_token=settings.TELEGRAM_BOT_TOKEN,
                allowed_user_ids=user_ids,
            )
            channel_manager.register(telegram_channel)
        except ImportError:
            logger.warning("⚠️ python-telegram-bot 未安装，跳过 Telegram 通道。安装: pip install python-telegram-bot")

    await channel_manager.start_all()



def create_app() -> FastAPI:
    # 统一处理mcp_apps参数，确保是列表形式

    app = FastAPI(
        debug=settings.DEBUG,
        title=settings.PROJECT_NAME,
        lifespan=lifespan,
        openapi_url=f"/openapi.json" if settings.ENABLE_SWAGGER_DOC else None,
        docs_url=f"/docs" if settings.ENABLE_SWAGGER_DOC else None,
        redoc_url=f"/redoc" if settings.ENABLE_SWAGGER_DOC else None,
        routes=api_router.routes,

    )

    # 注册中间件
    register_middleware(app)

    # 注册捕获全局异常
    register_exception(app)

    # 请求拦截
    register_hook(app)

    return app


app = create_app()
