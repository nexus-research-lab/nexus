# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cli.py
# @Date   ：2026/03/26 01:29
# @Author ：leemysw
# 2026/03/26 01:29   Create
# =====================================================

"""主智能体编排 CLI。"""

from __future__ import annotations

import asyncio
import json
import sys
from collections.abc import Awaitable, Callable
from pathlib import Path

import click
import typer

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from agent.cli.command import (  # noqa: E402
    build_typer_app,
)
from agent.service.agent.main_agent_profile import MainAgentProfile  # noqa: E402

ServiceCall = Callable[[object], Awaitable[object]]


class MainAgentOrchestrationCli:
    """封装主智能体的协作编排命令行。"""

    def __init__(self) -> None:
        self._app = build_typer_app(
            run_service_call=self._run_service_call,
            parse_agent_ids=self._parse_agent_ids,
        )

    def run(self, argv: list[str] | None = None) -> int:
        """执行 CLI。"""
        try:
            self._app(
                args=argv,
                prog_name=f"{MainAgentProfile.display_name()}_orchestration_cli",
                standalone_mode=False,
            )
        except typer.Exit as exc:
            return int(exc.exit_code or 0)
        except click.ClickException as exc:
            exc.show()
            return exc.exit_code
        return 0

    def _run_service_call(self, service_call: ServiceCall) -> None:
        """执行服务调用并统一输出 JSON。"""
        try:
            result = asyncio.run(self._execute(service_call))
        except Exception as exc:  # pylint: disable=broad-except
            self._print({"ok": False, "error": str(exc)})
            raise typer.Exit(code=1) from exc

        self._print({"ok": True, "data": result})

    @staticmethod
    async def _execute(service_call: ServiceCall) -> object:
        """惰性导入服务，避免 help 阶段初始化后端。"""
        from agent.service.agent.main_agent_orchestration_service import (  # noqa: WPS433
            main_agent_orchestration_service,
        )
        return await service_call(main_agent_orchestration_service)

    @staticmethod
    def _parse_agent_ids(raw_value: str) -> list[str]:
        """解析逗号分隔的成员列表。"""
        agent_ids = [item.strip() for item in raw_value.split(",") if item.strip()]
        if not agent_ids:
            raise ValueError("agent_ids 不能为空")
        return agent_ids

    @staticmethod
    def _print(payload: dict[str, object]) -> None:
        """统一输出 JSON。"""
        print(json.dumps(payload, ensure_ascii=False, indent=2))


def main() -> int:
    """CLI 入口。"""
    return MainAgentOrchestrationCli().run()


if __name__ == "__main__":
    raise SystemExit(main())
