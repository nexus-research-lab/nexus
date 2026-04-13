# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_template_initializer.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Workspace 模板初始化器。"""

from datetime import datetime
from pathlib import Path

from agent.service.workspace.workspace_templates import (
    DEFAULT_DIR,
    WORKSPACE_FILES,
    get_workspace_templates,
)
from agent.service.workspace.workspace_hook_settings import WorkspaceHookSettings
from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer
from agent.service.workspace.workspace_template_renderer import WorkspaceTemplateRenderer
from agent.utils.logger import logger


class WorkspaceTemplateInitializer:
    """负责创建 workspace 目录与默认模板。"""

    def __init__(self, agent_id: str, workspace_path: Path):
        self._agent_id = agent_id
        self._workspace_path = workspace_path
        self._hook_settings = WorkspaceHookSettings(workspace_path)
        self._skill_deployer = WorkspaceSkillDeployer(agent_id, workspace_path)
        self._exists_ensured = False
        self._initialized = False

    def ensure_exists(self) -> None:
        """确保 workspace 目录和子目录存在。"""
        if self._exists_ensured and self._workspace_path.exists():
            return

        root_created = not self._workspace_path.exists()
        self._workspace_path.mkdir(parents=True, exist_ok=True)
        created_subdirs: list[str] = []

        for subdir in DEFAULT_DIR.values():
            target_dir = self._workspace_path / subdir
            if target_dir.exists():
                continue
            target_dir.mkdir(exist_ok=True)
            created_subdirs.append(subdir)

        for subdir in created_subdirs:
            logger.info(f"📁 初始化 Workspace 子目录: {subdir}")

        if root_created or created_subdirs:
            logger.info(f"📁 Workspace 就绪: {self._workspace_path}")

        self._exists_ensured = True

    def ensure_initialized(self, agent_name: str) -> None:
        """确保模板文件完成初始化。"""
        if self._initialized and self._workspace_path.exists():
            return

        self.ensure_exists()
        self._seed_templates(agent_name)
        self._initialized = True

    def _seed_templates(self, agent_name: str) -> None:
        """写入缺失的模板文件，不覆盖用户已有内容。"""
        created_at = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        project_root = Path(__file__).resolve().parents[3].as_posix()
        context = {
            "agent_id": self._agent_id,
            "agent_name": agent_name,
            "created_at": created_at,
            "project_root": project_root,
            "workspace": self._workspace_path.resolve().as_posix(),
        }
        template_map = get_workspace_templates(self._agent_id)
        renderer = WorkspaceTemplateRenderer(context)

        for key, filename in WORKSPACE_FILES.items():
            filepath = self._workspace_path / filename
            if filepath.exists():
                continue

            template = renderer.render(template_map.get(key, "")).strip()
            if not template:
                continue

            filepath.write_text(template + "\n", encoding="utf-8")
            logger.info(f"🧩 初始化模板: {filepath}")
        self._skill_deployer.ensure_deployed(context)
        self._hook_settings.ensure_memory_hooks()

        memory_readme = self._workspace_path / "memory" / "README.md"
        if not memory_readme.exists():
            memory_readme.write_text(
                "# memory/\n\n存放按天日志、摘要、调研片段、临时结论和可复用资产。\n"
                "按天日志使用 `YYYY-MM-DD.md`，其他文件名按内容自行命名。\n",
                encoding="utf-8",
            )
            logger.info(f"🧩 初始化模板: {memory_readme}")
