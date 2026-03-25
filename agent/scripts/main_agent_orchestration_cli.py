# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main_agent_orchestration_cli.py
# @Date   ：2026/03/26 01:29
# @Author ：leemysw
# 2026/03/26 01:29   Create
# =====================================================

"""main agent 编排 CLI。"""

from __future__ import annotations

import argparse
import asyncio
import json
import sys
from pathlib import Path
from typing import Any

PROJECT_ROOT = Path(__file__).resolve().parents[2]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from agent.service.agent.main_agent_orchestration_service import (  # noqa: E402
    main_agent_orchestration_service,
)


class MainAgentOrchestrationCli:
    """封装 main agent 的协作编排命令行。"""

    def __init__(self) -> None:
        self._parser = argparse.ArgumentParser(
            prog="main_agent_orchestration_cli",
            description="供 main agent 调用的协作编排 CLI",
        )
        self._register_commands()

    def _register_commands(self) -> None:
        subparsers = self._parser.add_subparsers(dest="command", required=True)

        list_agents_parser = subparsers.add_parser("list_agents", help="列出成员 agent")
        list_agents_parser.add_argument(
            "--include_main",
            action="store_true",
            help="是否包含 main agent",
        )

        validate_agent_parser = subparsers.add_parser(
            "validate_agent_name",
            help="校验 agent 名称",
        )
        validate_agent_parser.add_argument("--name", required=True, help="待校验名称")

        create_agent_parser = subparsers.add_parser("create_agent", help="创建成员 agent")
        create_agent_parser.add_argument("--name", required=True, help="成员名称")
        create_agent_parser.add_argument("--model", default=None, help="模型名")

        list_rooms_parser = subparsers.add_parser("list_rooms", help="列出最近 room")
        list_rooms_parser.add_argument("--limit", type=int, default=20, help="数量上限")

        create_room_parser = subparsers.add_parser("create_room", help="创建 room")
        create_room_parser.add_argument(
            "--agent_ids",
            required=True,
            help="逗号分隔的 agent_id 列表",
        )
        create_room_parser.add_argument("--name", default=None, help="room 名称")
        create_room_parser.add_argument("--title", default=None, help="首条对话标题")
        create_room_parser.add_argument("--description", default="", help="room 描述")

        add_member_parser = subparsers.add_parser("add_room_member", help="向 room 追加成员")
        add_member_parser.add_argument("--room_id", required=True, help="room_id")
        add_member_parser.add_argument("--agent_id", required=True, help="agent_id")

    def run(self, argv: list[str] | None = None) -> int:
        """执行 CLI。"""
        args = self._parser.parse_args(argv)
        return asyncio.run(self._dispatch(args))

    async def _dispatch(self, args: argparse.Namespace) -> int:
        try:
            result = await self._execute(args)
        except Exception as exc:  # pylint: disable=broad-except
            self._print({
                "ok": False,
                "error": str(exc),
            })
            return 1

        self._print({
            "ok": True,
            "data": result,
        })
        return 0

    async def _execute(self, args: argparse.Namespace) -> Any:
        if args.command == "list_agents":
            return await main_agent_orchestration_service.list_agents(
                include_main=args.include_main,
            )
        if args.command == "validate_agent_name":
            return await main_agent_orchestration_service.validate_agent_name(args.name)
        if args.command == "create_agent":
            return await main_agent_orchestration_service.create_agent(
                name=args.name,
                model=args.model,
            )
        if args.command == "list_rooms":
            return await main_agent_orchestration_service.list_rooms(limit=args.limit)
        if args.command == "create_room":
            return await main_agent_orchestration_service.create_room(
                agent_ids=self._parse_agent_ids(args.agent_ids),
                name=args.name,
                title=args.title,
                description=args.description,
            )
        if args.command == "add_room_member":
            return await main_agent_orchestration_service.add_room_member(
                room_id=args.room_id,
                agent_id=args.agent_id,
            )
        raise ValueError(f"不支持的命令: {args.command}")

    @staticmethod
    def _parse_agent_ids(raw_value: str) -> list[str]:
        """解析逗号分隔的成员列表。"""
        agent_ids = [item.strip() for item in raw_value.split(",") if item.strip()]
        if not agent_ids:
            raise ValueError("agent_ids 不能为空")
        return agent_ids

    @staticmethod
    def _print(payload: dict[str, Any]) -> None:
        """统一输出 JSON。"""
        print(json.dumps(payload, ensure_ascii=False, indent=2))


def main() -> int:
    """CLI 入口。"""
    return MainAgentOrchestrationCli().run()


if __name__ == "__main__":
    raise SystemExit(main())
