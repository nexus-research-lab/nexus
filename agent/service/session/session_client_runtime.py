# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_client_runtime.py
# @Date   ：2026/04/14 15:21
# @Author ：leemysw
# 2026/04/14 15:21   Create
# =====================================================

"""SDK client 运行态辅助方法。"""

from __future__ import annotations

from pathlib import Path
from typing import Any, Mapping

from claude_agent_sdk import CanUseTool, ClaudeAgentOptions, ClaudeSDKClient

from agent.infra.server.common.base_exception import ServerException
from agent.utils.logger import logger


class SessionClientRuntime:
    """封装 ClaudeSDKClient 的健康检查与兜底关闭逻辑。"""

    RECONNECT_OPTION_FIELDS = (
        "cwd",
        "env",
        "system_prompt",
        "allowed_tools",
        "disallowed_tools",
        "max_turns",
        "max_thinking_tokens",
        "setting_sources",
        "tools",
        "fallback_model",
        "betas",
        "sandbox",
        "thinking",
        "effort",
        "output_format",
        "mcp_servers",
    )

    @staticmethod
    def inspect_health_issue(client: ClaudeSDKClient) -> str | None:
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

    @staticmethod
    def is_connected(client: ClaudeSDKClient) -> bool:
        """判断当前 client 是否仍保持 Claude CLI 连接。"""
        query = getattr(client, "_query", None)
        if query is None:
            return False
        return not bool(getattr(query, "_closed", False))

    @classmethod
    def requires_reconnect(
        cls,
        client: ClaudeSDKClient,
        session_options: Mapping[str, Any],
    ) -> bool:
        """判断最新配置是否要求重连底层 Claude CLI。"""
        current_snapshot = cls._build_option_snapshot(getattr(client, "options", None))
        target_snapshot = cls._build_option_snapshot(session_options)
        return current_snapshot != target_snapshot

    @classmethod
    def _build_option_snapshot(cls, options: Any) -> dict[str, Any]:
        """提取需要通过重连生效的稳定配置快照。"""
        if options is None:
            return {}

        normalized_options = options
        if isinstance(options, Mapping):
            normalized_options = ClaudeAgentOptions(**dict(options))

        source: Mapping[str, Any]
        if hasattr(normalized_options, "__dict__"):
            source = getattr(normalized_options, "__dict__")
        elif isinstance(normalized_options, Mapping):
            source = normalized_options
        else:
            return {}

        snapshot: dict[str, Any] = {}
        for field_name in cls.RECONNECT_OPTION_FIELDS:
            snapshot[field_name] = cls._normalize_value(
                field_name,
                source.get(field_name),
            )
        return snapshot

    @classmethod
    def _normalize_value(cls, field_name: str, value: Any) -> Any:
        """把复杂配置收敛成可稳定比较的结构。"""
        if field_name == "mcp_servers":
            return cls._normalize_mcp_servers(value)
        if isinstance(value, Path):
            return value.as_posix()
        if isinstance(value, Mapping):
            return {
                str(key): cls._normalize_value("", item)
                for key, item in sorted(value.items(), key=lambda item: str(item[0]))
            }
        if isinstance(value, (list, tuple, set)):
            return [cls._normalize_value("", item) for item in value]
        if isinstance(value, (str, int, float, bool)) or value is None:
            return value
        return repr(value)

    @classmethod
    def _normalize_mcp_servers(cls, value: Any) -> Any:
        """规避 SDK MCP 实例对象的地址抖动，保留稳定结构。"""
        if not isinstance(value, Mapping):
            return cls._normalize_value("", value)
        normalized: dict[str, Any] = {}
        for server_name, config in sorted(value.items(), key=lambda item: str(item[0])):
            if isinstance(config, Mapping):
                normalized[str(server_name)] = {
                    str(key): cls._normalize_value("", item)
                    for key, item in sorted(config.items(), key=lambda item: str(item[0]))
                    if str(key) != "instance"
                }
                continue
            normalized[str(server_name)] = type(config).__name__
        return normalized

    @staticmethod
    async def force_terminate_process(
        session_key: str,
        client: ClaudeSDKClient,
    ) -> None:
        """在常规 disconnect 失败时强制终止底层 Claude CLI 进程。"""
        transport = getattr(client, "_transport", None)
        process = getattr(transport, "_process", None)
        if process is None:
            return

        pid = getattr(process, "pid", None)
        try:
            process.terminate()
            await process.aclose()
            logger.warning(f"⚠️ 已强制终止 SDK 子进程: key={session_key}, pid={pid}")
            return
        except Exception as terminate_exc:
            logger.warning(
                f"⚠️ terminate SDK 子进程失败，准备 kill: key={session_key}, pid={pid}, error={terminate_exc}"
            )

        try:
            process.kill()
            await process.aclose()
            logger.warning(f"⚠️ 已 kill SDK 子进程: key={session_key}, pid={pid}")
        except Exception as kill_exc:
            logger.error(
                f"❌ kill SDK 子进程失败: key={session_key}, pid={pid}, error={kill_exc}"
            )

    @staticmethod
    def build_client_options(
        can_use_tool: CanUseTool | None,
        session_id: str | None,
        session_options: dict[str, Any] | None,
    ) -> ClaudeAgentOptions:
        """构造 ClaudeSDKClient 需要的标准 options。"""
        options = ClaudeAgentOptions(can_use_tool=can_use_tool, **(session_options or {}))
        if session_id:
            options.resume = session_id
        cwd = Path(options.cwd)
        if not cwd.is_dir():
            raise ServerException(f"指定的cwd路径不存在: {cwd}")
        options.cwd = cwd.absolute().as_posix()
        return options

    @staticmethod
    async def apply_hot_updates(
        *,
        session_key: str,
        client: ClaudeSDKClient,
        target_permission_mode: str | None,
        target_model: str | None,
        reconnect_required: bool,
    ) -> bool:
        """对 SDK 原生支持的字段执行热更新。"""
        current_permission_mode = getattr(client.options, "permission_mode", None)
        current_model = getattr(client.options, "model", None)

        if target_permission_mode != current_permission_mode:
            try:
                await client.set_permission_mode(target_permission_mode or "default")
                client.options.permission_mode = target_permission_mode
                logger.info(f"🔧 已热更新权限模式: key={session_key}, mode={target_permission_mode}")
            except Exception as exc:
                reconnect_required = True
                logger.warning(
                    f"⚠️ 权限模式热更新失败，将在下次请求前重连: key={session_key}, error={exc}"
                )

        if target_model != current_model:
            try:
                await client.set_model(target_model)
                client.options.model = target_model
                logger.info(f"🔧 已热更新模型: key={session_key}, model={target_model}")
            except Exception as exc:
                reconnect_required = True
                logger.warning(
                    f"⚠️ 模型热更新失败，将在下次请求前重连: key={session_key}, error={exc}"
                )
        return reconnect_required
