# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main_agent_profile.py
# @Date   ：2026/03/25 23:28
# @Author ：leemysw
# 2026/03/25 23:28   Create
# =====================================================

"""主智能体固定配置。"""

from collections.abc import Callable, Iterable
from pathlib import Path
from typing import Any, Dict, TypeVar

from agent.config.config import settings

_T = TypeVar("_T")


class MainAgentProfile:
    """负责描述系统保留主智能体的固定身份与默认运行参数。"""

    AGENT_ID = settings.DEFAULT_AGENT_ID
    ALLOWED_TOOLS = [
        "AskUserQuestion",
        "Bash",
        "Edit",
        "Glob",
        "Grep",
        "LS",
        "Read",
        "Skill",
        "TodoWrite",
        "WebFetch",
        "WebSearch",
        "Write",
    ]
    REGULAR_AGENT_ALLOWED_TOOLS = ALLOWED_TOOLS.copy()
    SETTING_SOURCES = ["project"]

    @classmethod
    def is_main_agent(cls, agent_id: str) -> bool:
        """判断是否为系统保留的主智能体。"""
        return agent_id == cls.AGENT_ID

    @classmethod
    def is_regular_agent(cls, agent_id: str) -> bool:
        """判断是否为普通成员智能体。"""
        return bool(agent_id) and not cls.is_main_agent(agent_id)

    @classmethod
    def display_name(cls) -> str:
        """返回当前配置的主智能体标识。"""
        return cls.AGENT_ID

    @classmethod
    def display_label(cls) -> str:
        """返回用于提示的主智能体名称。"""
        return f"主智能体（{cls.display_name()}）"

    @classmethod
    def ensure_not_main_agent(cls, agent_id: str, action: str) -> None:
        """阻止把主智能体当作普通成员处理。"""
        if cls.is_main_agent(agent_id):
            raise ValueError(f"{cls.display_label()} {action}")

    @classmethod
    def filter_regular_agents(
        cls,
        items: Iterable[_T],
        get_agent_id: Callable[[_T], str],
    ) -> list[_T]:
        """从集合中过滤出普通成员智能体。"""
        return [item for item in items if cls.is_regular_agent(get_agent_id(item))]

    @classmethod
    def build_default_options(cls) -> Dict[str, Any]:
        """构建主智能体的默认运行参数。"""
        options: Dict[str, Any] = {
            "allowed_tools": cls.ALLOWED_TOOLS,
            "permission_mode": "default",
            "setting_sources": cls.SETTING_SOURCES,
        }
        return options

    @classmethod
    def build_storage_record(cls, workspace_path: Path) -> Dict[str, Any]:
        """构建主智能体的存储记录。"""
        return {
            "agent_id": cls.AGENT_ID,
            "name": cls.AGENT_ID,
            "workspace_path": str(workspace_path),
            "options": cls.build_default_options(),
            "status": "active",
        }

    @classmethod
    def merge_options(cls, current_options: Any) -> Dict[str, Any]:
        """为主智能体补齐缺失的默认运行参数。"""
        merged_options = dict(current_options) if isinstance(current_options, dict) else {}
        default_options = cls.build_default_options()
        merged_options["allowed_tools"] = default_options["allowed_tools"]
        merged_options["permission_mode"] = default_options["permission_mode"]
        merged_options["setting_sources"] = default_options["setting_sources"]
        return merged_options

    @classmethod
    def build_regular_agent_options(cls, model: str | None = None) -> Dict[str, Any]:
        """构建普通成员 agent 的默认运行参数。"""
        options: Dict[str, Any] = {
            "allowed_tools": cls.REGULAR_AGENT_ALLOWED_TOOLS.copy(),
            "permission_mode": "default",
            "setting_sources": cls.SETTING_SOURCES,
        }
        if model:
            options["model"] = model
        return options
