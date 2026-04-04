# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_hook_settings.py
# @Date   ：2026/04/04 12:45
# @Author ：leemysw
# 2026/04/04 12:45   Create
# =====================================================

"""Workspace hook 配置写入器。"""

from __future__ import annotations

import json
from pathlib import Path

from agent.utils.logger import logger


class WorkspaceHookSettings:
    """负责把记忆提醒 hooks 写入 Claude 本地配置。"""

    def __init__(self, workspace_path: Path):
        self._workspace_path = workspace_path
        self._settings_path = workspace_path / ".claude" / "settings.local.json"

    def ensure_memory_hooks(self) -> None:
        """确保 memory-manager 相关 hooks 已启用。"""
        payload = self._load_payload()
        hooks = payload.setdefault("hooks", {})

        self._upsert_command_hook(
            hooks=hooks,
            event_name="UserPromptSubmit",
            matcher="",
            command="bash ./.claude/skills/memory-manager/scripts/activator.sh",
        )
        self._upsert_command_hook(
            hooks=hooks,
            event_name="PostToolUse",
            matcher="Bash",
            command="bash ./.claude/skills/memory-manager/scripts/error-detector.sh",
        )

        self._settings_path.parent.mkdir(parents=True, exist_ok=True)
        self._settings_path.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        logger.info(f"🪝 已写入记忆 hooks: {self._settings_path}")

    def _load_payload(self) -> dict:
        """读取已有配置。"""
        if not self._settings_path.exists():
            return {}
        return json.loads(self._settings_path.read_text(encoding="utf-8"))

    @staticmethod
    def _upsert_command_hook(
        hooks: dict,
        event_name: str,
        matcher: str,
        command: str,
    ) -> None:
        """向指定事件中写入命令 hook。"""
        event_hooks = hooks.setdefault(event_name, [])
        group = WorkspaceHookSettings._find_group(event_hooks, matcher)
        command_hook = {
            "type": "command",
            "command": command,
        }

        if group is None:
            event_hooks.append(
                {
                    "matcher": matcher,
                    "hooks": [command_hook],
                }
            )
            return

        hooks_list = group.setdefault("hooks", [])
        for item in hooks_list:
            if item.get("type") == "command" and item.get("command") == command:
                return
        hooks_list.append(command_hook)

    @staticmethod
    def _find_group(event_hooks: list[dict], matcher: str) -> dict | None:
        """按 matcher 查找 hook 分组。"""
        for item in event_hooks:
            if item.get("matcher", "") == matcher:
                return item
        return None
