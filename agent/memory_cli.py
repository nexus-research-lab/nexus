# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：memory_cli.py
# @Date   ：2026/04/04 12:45
# @Author ：leemysw
# 2026/04/04 12:45   Create
# =====================================================

"""记忆系统 CLI。"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from agent.service.memory import MemoryService  # noqa: E402


class MemoryCliApp:
    """封装记忆相关命令行。"""

    def __init__(self) -> None:
        self._parser = self._build_parser()

    def run(self, argv: list[str] | None = None) -> int:
        """执行命令。"""
        args = self._parser.parse_args(argv)
        service = MemoryService(args.workspace)

        try:
            if args.command == "search":
                payload = service.search(query=args.query, limit=args.limit)
            elif args.command == "get":
                payload = service.get(
                    relative_path=args.path,
                    from_line=args.from_line,
                    lines=args.lines,
                )
            elif args.command == "review":
                payload = service.review_recent_entries(days=args.days, limit=args.limit)
            elif args.command == "log":
                payload = service.log(
                    kind=args.kind,
                    title=args.title,
                    category=args.category,
                    fields=self._parse_fields(args.field),
                    promote_target=args.promote_target,
                )
            elif args.command == "promote":
                payload = service.promote(
                    target=args.target,
                    title=args.title,
                    content=args.content,
                    entry_id=args.entry_id,
                )
            elif args.command == "resolve":
                payload = service.resolve_entry(
                    entry_id=args.entry_id,
                    note=args.note,
                )
            elif args.command == "set-status":
                payload = service.set_entry_status(
                    entry_id=args.entry_id,
                    status=args.status,
                    note=args.note,
                )
            else:
                raise ValueError(f"未知命令: {args.command}")
        except Exception as exc:  # pylint: disable=broad-except
            self._print({"ok": False, "error": str(exc)})
            return 1

        self._print({"ok": True, "data": payload})
        return 0

    @staticmethod
    def _build_parser() -> argparse.ArgumentParser:
        """构造参数解析器。"""
        parser = argparse.ArgumentParser(description="Nexus 记忆管理 CLI")
        parser.add_argument("--workspace", required=True, help="Agent workspace 绝对路径")
        subparsers = parser.add_subparsers(dest="command", required=True)

        search_parser = subparsers.add_parser("search", help="搜索记忆")
        search_parser.add_argument("--query", required=True, help="搜索关键词")
        search_parser.add_argument("--limit", type=int, default=20, help="返回数量")

        get_parser = subparsers.add_parser("get", help="读取文件片段")
        get_parser.add_argument("--path", required=True, help="工作区相对路径")
        get_parser.add_argument("--from_line", type=int, default=1, help="起始行")
        get_parser.add_argument("--lines", type=int, default=50, help="读取行数")

        review_parser = subparsers.add_parser("review", help="读取最近日记标题")
        review_parser.add_argument("--days", type=int, default=3, help="最近天数")
        review_parser.add_argument("--limit", type=int, default=8, help="返回数量")

        log_parser = subparsers.add_parser("log", help="向今日日记追加条目")
        log_parser.add_argument("--kind", required=True, help="LRN | ERR | FEAT | REF")
        log_parser.add_argument("--title", required=True, help="条目标题")
        log_parser.add_argument("--category", help="学习分类，仅 LRN 使用")
        log_parser.add_argument(
            "--promote-target",
            help="写入后立即提升到 memory | soul | tools | agents",
        )
        log_parser.add_argument(
            "--field",
            action="append",
            default=[],
            help="字段，格式为 键=值，可重复传入",
        )

        promote_parser = subparsers.add_parser("promote", help="提升为长期规则")
        promote_parser.add_argument("--target", required=True, help="memory | soul | tools | agents")
        promote_parser.add_argument("--content", required=True, help="提升内容")
        promote_parser.add_argument("--title", help="可选标题")
        promote_parser.add_argument("--entry-id", "--entry_id", help="可选，回写条目状态")

        resolve_parser = subparsers.add_parser("resolve", help="把条目标记为已解决")
        resolve_parser.add_argument("--entry-id", "--entry_id", required=True, help="条目 ID")
        resolve_parser.add_argument("--note", required=True, help="解决说明")

        status_parser = subparsers.add_parser("set-status", help="更新条目状态")
        status_parser.add_argument("--entry-id", "--entry_id", required=True, help="条目 ID")
        status_parser.add_argument("--status", required=True, help="目标状态")
        status_parser.add_argument("--note", help="状态说明")
        return parser

    @staticmethod
    def _parse_fields(raw_fields: list[str]) -> list[tuple[str, str]]:
        """解析重复字段参数。"""
        pairs: list[tuple[str, str]] = []
        for item in raw_fields:
            if "=" not in item:
                raise ValueError(f"field 格式错误: {item}")
            key, value = item.split("=", 1)
            pairs.append((key, value))
        return pairs

    @staticmethod
    def _print(payload: dict[str, object]) -> None:
        """输出 JSON。"""
        print(json.dumps(payload, ensure_ascii=False, indent=2))


def main() -> int:
    """CLI 入口。"""
    return MemoryCliApp().run()


if __name__ == "__main__":
    raise SystemExit(main())
