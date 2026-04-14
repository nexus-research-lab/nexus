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

from claude_agent_sdk import ClaudeSDKClient

from agent.utils.logger import logger


class SessionClientRuntime:
    """封装 ClaudeSDKClient 的健康检查与兜底关闭逻辑。"""

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
