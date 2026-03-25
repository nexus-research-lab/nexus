# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main_agent_profile.py
# @Date   ：2026/03/25 23:28
# @Author ：leemysw
# 2026/03/25 23:28   Create
# =====================================================

"""main agent 固定配置。"""

from pathlib import Path
from typing import Any, Dict

from agent.config.config import settings


class MainAgentProfile:
    """负责描述系统保留 main agent 的固定身份与默认运行参数。"""

    AGENT_ID = "main"
    AGENT_NAME = "main"

    @classmethod
    def is_main_agent(cls, agent_id: str) -> bool:
        """判断是否为系统保留的 main agent。"""
        return agent_id == (settings.DEFAULT_AGENT_ID or cls.AGENT_ID)

    @classmethod
    def build_default_options(cls) -> Dict[str, Any]:
        """构建 main agent 的默认运行参数。"""
        options: Dict[str, Any] = {
            "permission_mode": "default",
            "skills_enabled": True,
            "setting_sources": ["user", "project"],
        }
        if settings.MAIN_AGENT_MODEL:
            options["model"] = settings.MAIN_AGENT_MODEL
        return options

    @classmethod
    def build_storage_record(cls, workspace_path: Path) -> Dict[str, Any]:
        """构建 main agent 的存储记录。"""
        return {
            "agent_id": cls.AGENT_ID,
            "name": cls.AGENT_NAME,
            "workspace_path": str(workspace_path),
            "options": cls.build_default_options(),
            "status": "active",
        }

    @classmethod
    def merge_options(cls, current_options: Any) -> Dict[str, Any]:
        """为 main agent 补齐缺失的默认运行参数。"""
        merged_options = dict(current_options) if isinstance(current_options, dict) else {}
        for key, value in cls.build_default_options().items():
            merged_options.setdefault(key, value)
        return merged_options
