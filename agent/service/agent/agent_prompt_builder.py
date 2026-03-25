# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_prompt_builder.py
# @Date   ：2026/3/17 20:27
# @Author ：leemysw
# 2026/3/17 20:27   Create
# =====================================================

"""Agent 系统提示词构建器。"""
import os
from pathlib import Path
from typing import Optional

from agent.config.config import settings


class AgentPromptBuilder:
    """负责从基础提示词和 workspace 文件组装 system prompt。"""

    _PROMPT_FILE_NAMES = ("agents", "user", "memory", "runbook")

    @staticmethod
    def load_base_system_prompt() -> Optional[str]:
        """加载独立于 workspace 的基础 system prompt。"""

        if settings.BASE_SYSTEM_PROMPT:
            return settings.BASE_SYSTEM_PROMPT.strip() or None

        if settings.BASE_SYSTEM_PROMPT_FILE:
            prompt_path = Path(settings.BASE_SYSTEM_PROMPT_FILE).expanduser()
            if prompt_path.exists() and prompt_path.is_file():
                content = prompt_path.read_text(encoding="utf-8").strip()
                return content or None

        default_prompt_path = Path(os.getcwd()) / "SYSTEM_PROMPT.md"
        if default_prompt_path.exists() and default_prompt_path.is_file():
            content = default_prompt_path.read_text(encoding="utf-8").strip()
            return content or None

        return None

    @staticmethod
    def load_main_agent_system_prompt(agent_id: str) -> Optional[str]:
        """加载 main agent 的独立 system prompt。"""
        if agent_id != (settings.DEFAULT_AGENT_ID or "main"):
            return None

        if settings.MAIN_AGENT_SYSTEM_PROMPT:
            return settings.MAIN_AGENT_SYSTEM_PROMPT.strip() or None

        if settings.MAIN_AGENT_SYSTEM_PROMPT_FILE:
            prompt_path = Path(settings.MAIN_AGENT_SYSTEM_PROMPT_FILE).expanduser()
            if prompt_path.exists() and prompt_path.is_file():
                content = prompt_path.read_text(encoding="utf-8").strip()
                return content or None

        main_prompt_path = Path(os.getcwd()) / "MAIN_AGENT_SYSTEM_PROMPT.md"
        if main_prompt_path.exists() and main_prompt_path.is_file():
            content = main_prompt_path.read_text(encoding="utf-8").strip()
            return content or None

        return None

    def build(self, workspace, agent_id: str) -> Optional[str]:
        """构建最终 system prompt。"""
        sections: list[str] = []
        main_agent_prompt = self.load_main_agent_system_prompt(agent_id)
        if main_agent_prompt:
            sections.append(main_agent_prompt)
        else:
            base_prompt = self.load_base_system_prompt()
            if base_prompt:
                sections.append(base_prompt)

        for name in self._PROMPT_FILE_NAMES:
            content = workspace.read_file(name)
            if content:
                sections.append(content)

        if not sections:
            return None
        return "\n\n---\n\n".join(sections)
