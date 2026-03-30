# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_skill_deployer.py
# @Date   ：2026/3/25 11:37
# @Author ：leemysw
# 2026/3/25 11:37   Create
# =====================================================

"""Workspace skill 部署器。"""

import shutil
from pathlib import Path

from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.utils.logger import logger


class WorkspaceSkillDeployer:
    """负责把仓库内 skill 部署到 agent workspace。"""

    MAIN_AGENT_SKILL_NAMES = ("nexus-manager",)

    def __init__(self, agent_id: str, workspace_path: Path):
        self._agent_id = agent_id
        self._workspace_path = workspace_path
        self._project_root = Path(__file__).resolve().parents[3]
        self._repo_skills_root = self._project_root / "skills"
        self._workspace_agents_skills_root = self._workspace_path / ".agents" / "skills"
        self._workspace_claude_skills_root = self._workspace_path / ".claude" / "skills"

    def ensure_deployed(self, context: dict[str, str]) -> None:
        """确保主智能体 skill 已部署到 workspace。"""
        if not MainAgentProfile.is_main_agent(self._agent_id):
            return

        for skill_name in self.MAIN_AGENT_SKILL_NAMES:
            self._deploy_skill(skill_name, None, context)

    # =====================================================
    # 公共安装/卸载接口
    # =====================================================

    def deploy_skill(
            self,
            skill_name: str,
            source_dir: Path | None = None,
            context: dict[str, str] | None = None,
    ) -> None:
        """安装单个 skill 到 workspace。"""
        self._deploy_skill(skill_name, source_dir, context or {})

    def undeploy_skill(self, skill_name: str) -> None:
        """从 workspace 移除单个 skill。"""
        target_dir = self._workspace_agents_skills_root / skill_name
        link_path = self._workspace_claude_skills_root / skill_name

        if link_path.is_symlink() or link_path.exists():
            link_path.unlink()
            logger.info(f"🔗 已移除 Claude skill 链接: {link_path}")

        if target_dir.exists():
            shutil.rmtree(target_dir)
            logger.info(f"🗑️ 已移除 skill 目录: {target_dir}")

    def list_deployed(self) -> list[str]:
        """列出 workspace 内已部署的 skill 名称。"""
        if not self._workspace_agents_skills_root.exists():
            return []
        return sorted(
            d.name for d in self._workspace_agents_skills_root.iterdir()
            if d.is_dir()
        )

    def _deploy_skill(
            self,
            skill_name: str,
            source_dir: Path | None,
            context: dict[str, str],
    ) -> None:
        """部署单个 skill，并把 Claude 目录映射到内部 skill 目录。"""
        resolved_source_dir = source_dir or (self._repo_skills_root / skill_name)
        if not resolved_source_dir.exists():
            raise FileNotFoundError(f"未找到 skill 目录: {resolved_source_dir}")

        target_dir = self._workspace_agents_skills_root / skill_name
        self._sync_skill_directory(resolved_source_dir, target_dir, context)
        self._ensure_claude_skill_link(skill_name, target_dir)

    def _sync_skill_directory(
            self,
            source_dir: Path,
            target_dir: Path,
            context: dict[str, str],
    ) -> None:
        """同步 skill 目录内容。

        这里直接以仓库目录为准写入 workspace，不保留旧模板兼容逻辑。
        """
        target_dir.mkdir(parents=True, exist_ok=True)

        for source_path in source_dir.rglob("*"):
            relative_path = source_path.relative_to(source_dir)
            target_path = target_dir / relative_path

            if source_path.is_dir():
                target_path.mkdir(parents=True, exist_ok=True)
                continue

            target_path.parent.mkdir(parents=True, exist_ok=True)
            if source_path.name == "SKILL.md":
                content = source_path.read_text(encoding="utf-8").format(**context).strip()
                target_path.write_text(content + "\n", encoding="utf-8")
            else:
                shutil.copy2(source_path, target_path)

            logger.info(f"🧩 已同步 skill 文件: {target_path}")

    def _ensure_claude_skill_link(self, skill_name: str, target_dir: Path) -> None:
        """确保 .claude/skills 下的 skill 入口指向 .agents/skills。"""
        self._workspace_claude_skills_root.mkdir(parents=True, exist_ok=True)
        link_path = self._workspace_claude_skills_root / skill_name
        relative_target = Path("..") / ".." / ".agents" / "skills" / skill_name

        if link_path.is_symlink():
            if Path(link_path.readlink()) == relative_target:
                return
            link_path.unlink()
        elif link_path.exists():
            if link_path.is_dir():
                shutil.rmtree(link_path)
            else:
                link_path.unlink()

        # 使用相对软链接，保证 workspace 整体迁移时映射关系仍然成立。
        link_path.symlink_to(relative_target, target_is_directory=target_dir.is_dir())
        logger.info(f"🔗 已映射 Claude skill: {link_path} -> {relative_target}")

