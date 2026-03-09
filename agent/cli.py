# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cli
# @Date   ：2025/6/18 15:00
# @Author ：leemysw
# 2025/6/18 15:00   Create
# =====================================================

import warnings

warnings.filterwarnings("ignore", category=RuntimeWarning)

import signal
import sys
import os
from typing import Annotated, Optional, Set

import typer

from agent.core.config import settings
from agent.utils import utils
from agent.utils.logger import logger

client = typer.Typer(rich_markup_mode="rich")
VALID_CHANNELS = {"ws", "tg", "dg"}


def _normalize_channels(channels: Optional[list[str]]) -> Optional[Set[str]]:
    """规范化通道参数，支持重复参数和逗号分隔。"""
    if not channels:
        return None

    normalized: Set[str] = set()
    for raw_value in channels:
        for part in raw_value.split(","):
            channel = part.strip().lower()
            if not channel:
                continue
            if channel not in VALID_CHANNELS:
                raise typer.BadParameter(
                    f"Invalid channel: {channel}. Supported channels: ws, tg, dg"
                )
            normalized.add(channel)

    if not normalized:
        raise typer.BadParameter("At least one channel must be specified")
    return normalized


def _set_bool_env(key: str, value: bool) -> None:
    """写入布尔环境变量，确保子进程配置一致。"""
    os.environ[key] = "true" if value else "false"


def _apply_channel_overrides(
        selected_channels: Optional[Set[str]],
        discord_bot_token: Optional[str],
        discord_allowed_guilds: Optional[str],
        discord_trigger_word: Optional[str],
        telegram_bot_token: Optional[str],
        telegram_allowed_users: Optional[str],
) -> None:
    """将 CLI 参数覆盖到运行时 settings。"""
    if selected_channels is not None:
        settings.WEBSOCKET_ENABLED = "ws" in selected_channels
        settings.DISCORD_ENABLED = "dg" in selected_channels
        settings.TELEGRAM_ENABLED = "tg" in selected_channels

        _set_bool_env("WEBSOCKET_ENABLED", settings.WEBSOCKET_ENABLED)
        _set_bool_env("DISCORD_ENABLED", settings.DISCORD_ENABLED)
        _set_bool_env("TELEGRAM_ENABLED", settings.TELEGRAM_ENABLED)

        logger.info(
            "🔧 CLI 通道覆盖: "
            f"ws={settings.WEBSOCKET_ENABLED}, "
            f"dg={settings.DISCORD_ENABLED}, "
            f"tg={settings.TELEGRAM_ENABLED}"
        )

    if discord_bot_token is not None:
        settings.DISCORD_BOT_TOKEN = discord_bot_token
        os.environ["DISCORD_BOT_TOKEN"] = discord_bot_token
    if discord_allowed_guilds is not None:
        settings.DISCORD_ALLOWED_GUILDS = discord_allowed_guilds
        os.environ["DISCORD_ALLOWED_GUILDS"] = discord_allowed_guilds
    if discord_trigger_word is not None:
        settings.DISCORD_TRIGGER_WORD = discord_trigger_word
        os.environ["DISCORD_TRIGGER_WORD"] = discord_trigger_word

    if telegram_bot_token is not None:
        settings.TELEGRAM_BOT_TOKEN = telegram_bot_token
        os.environ["TELEGRAM_BOT_TOKEN"] = telegram_bot_token
    if telegram_allowed_users is not None:
        settings.TELEGRAM_ALLOWED_USERS = telegram_allowed_users
        os.environ["TELEGRAM_ALLOWED_USERS"] = telegram_allowed_users

    if settings.DISCORD_ENABLED and not settings.DISCORD_BOT_TOKEN:
        logger.warning("⚠️ Discord 已启用但未提供 DISCORD_BOT_TOKEN")
    if settings.TELEGRAM_ENABLED and not settings.TELEGRAM_BOT_TOKEN:
        logger.warning("⚠️ Telegram 已启用但未提供 TELEGRAM_BOT_TOKEN")


async def run_server(**uvicorn_kwargs) -> None:
    from agent.shared.server.launcher import serve_http

    # workaround to avoid footguns where uvicorn drops requests with too
    # many concurrent requests active
    if settings.ENABLE_VLLM:
        from vllm.utils.system_utils import set_ulimit
        set_ulimit()

    def signal_handler(*_) -> None:
        # Interrupt server on sigterm while initializing
        raise KeyboardInterrupt("terminated")

    signal.signal(signal.SIGTERM, signal_handler)

    signal.signal(signal.SIGTERM, signal_handler)
    shutdown_task = await serve_http(**uvicorn_kwargs)

    # NB: Await server shutdown only after the backend context is exited
    await shutdown_task


@client.command(context_settings={"allow_extra_args": True, "ignore_unknown_options": True})
def run(
        server_type: Annotated[
            Optional[str],
            typer.Option(
                "--server-type",
                "-t",
                help="The server type to run the app. Options: gunicorn or uvicorn. Default: uvicorn."
            )
        ] = None,
        channels: Annotated[
            Optional[list[str]],
            typer.Option(
                "--channel",
                "-c",
                help="Enable channels, can be passed multiple times or comma-separated. Supported: ws, tg, dg",
            ),
        ] = None,
        discord_bot_token: Annotated[
            Optional[str],
            typer.Option("--discord-bot-token", help="Discord bot token"),
        ] = None,
        discord_allowed_guilds: Annotated[
            Optional[str],
            typer.Option("--discord-allowed-guilds", help="Discord allowed guild IDs, comma-separated"),
        ] = None,
        discord_trigger_word: Annotated[
            Optional[str],
            typer.Option("--discord-trigger-word", help="Discord trigger word, e.g. @nexus-core or cc"),
        ] = None,
        telegram_bot_token: Annotated[
            Optional[str],
            typer.Option("--telegram-bot-token", help="Telegram bot token"),
        ] = None,
        telegram_allowed_users: Annotated[
            Optional[str],
            typer.Option("--telegram-allowed-users", help="Telegram allowed user IDs, comma-separated"),
        ] = None,
):
    """
    Nexus-Core CLI - The [bold]Nexus-Core[/bold] command line app. 😎

    Run a [bold]FastAPI[/bold] app in [green]production[/green] mode. 🚀
    """

    selected_channels = _normalize_channels(channels)
    _apply_channel_overrides(
        selected_channels=selected_channels,
        discord_bot_token=discord_bot_token,
        discord_allowed_guilds=discord_allowed_guilds,
        discord_trigger_word=discord_trigger_word,
        telegram_bot_token=telegram_bot_token,
        telegram_allowed_users=telegram_allowed_users,
    )

    resolved_server_type = (server_type or settings.SERVER_TYPE or "uvicorn").lower()

    # Print config info
    utils.print_info(settings, logger)

    if resolved_server_type not in ["uvicorn", "gunicorn"]:
        typer.echo(f"Invalid server type: {resolved_server_type}. Options are [uvicorn] or [gunicorn].")
        return

    if resolved_server_type == "uvicorn":
        from agent.app import app
        kwargs = {
            "app": app,
            "host": settings.HOST,
            "port": settings.PORT,
            "reload": False if settings.WORKERS != 1 else settings.DEBUG,
            "workers": settings.WORKERS,
            "lifespan": 'on',
            "ws": "websockets-sansio",
            "log_config": utils.set_uvicorn_logger(settings.LOGGER_FORMAT)
        }
        import uvloop
        uvloop.run(run_server(**kwargs))
    elif resolved_server_type == "gunicorn":
        from gunicorn.app.wsgiapp import WSGIApplication

        sys.argv = [
            'gunicorn',  # 程序名
            'agent.app:app',  # 应用模块
            '-c',  # 配置文件参数
            utils.abspath('core/config_gunicorn.py')  # 配置文件路径
        ]

        WSGIApplication("%(prog)s [OPTIONS] [APP_MODULE]", prog=None).run()


def main() -> None:
    client()
