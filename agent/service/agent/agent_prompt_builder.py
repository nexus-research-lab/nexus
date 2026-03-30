# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_prompt_builder.py
# @Date   ：2026/3/17 20:27
# @Author ：leemysw
# 2026/3/17 20:27   Create
# =====================================================

"""Agent 系统提示词构建器。"""
from typing import Optional

from agent.config.config import settings
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.workspace.workspace_templates import (
    BASE_SYSTEM_PROMPT,
    MAIN_AGENT_SYSTEM_PROMPT,
)


class AgentPromptBuilder:
    """负责从基础提示词和 workspace 文件组装 system prompt。"""

    _PROMPT_FILE_NAMES = ("agents", "user", "memory", "runbook")

    @staticmethod
    def load_base_system_prompt() -> Optional[str]:
        """加载独立于 workspace 的基础 system prompt。"""
        if settings.BASE_SYSTEM_PROMPT:
            return settings.BASE_SYSTEM_PROMPT.strip() or None
        return BASE_SYSTEM_PROMPT.strip() or None

    @staticmethod
    def load_main_agent_system_prompt(agent_id: str) -> Optional[str]:
        """加载 main agent 的独立 system prompt。"""
        if not MainAgentProfile.is_main_agent(agent_id):
            return None
        if settings.MAIN_AGENT_SYSTEM_PROMPT:
            return settings.MAIN_AGENT_SYSTEM_PROMPT.strip() or None
        return MAIN_AGENT_SYSTEM_PROMPT.strip() or None

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

        # 注入工作区路径，避免模型写错路径
        workspace_path = getattr(workspace, "path", None)
        if workspace_path:
            sections.append(f"当前工作区绝对路径: {workspace_path}")

        for name in self._PROMPT_FILE_NAMES:
            content = workspace.read_file(name)
            if content:
                sections.append(content)

        if not sections:
            return None
        return "\n\n---\n\n".join(sections)
