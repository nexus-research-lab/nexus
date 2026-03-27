# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_service.py
# @Date   ：2026/3/27 20:00
# @Author ：leemysw
# 2026/3/27 20:00   Create
# =====================================================

"""Skill 注册与部署服务。"""

import re
from pathlib import Path
from typing import Optional

from agent.schema.model_agent import AgentSkillEntry, SkillInfo
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer
from agent.storage.agent_repository import agent_repository
from agent.utils.logger import logger


class SkillService:
    """负责 skill 注册表扫描、agent skill 查询和安装/卸载。"""

    # 基础 skill — 所有 agent 默认拥有，不可卸载
    BASE_SKILL_NAMES = ("memory-manager",)
    # 主智能体专属 skill — 仅 main agent 拥有，不可卸载
    MAIN_AGENT_SKILL_NAMES = ("nexus-manager",)

    def __init__(self):
        self._project_root = Path(__file__).resolve().parents[3]
        self._repo_skills_root = self._project_root / "skills"

    # =====================================================
    # Registry — 扫描所有可用 skill
    # =====================================================

    def get_all_skills(self) -> list[SkillInfo]:
        """扫描 repo/skills/ 目录，返回所有已注册 skill 的元信息。"""
        skills: list[SkillInfo] = []
        if not self._repo_skills_root.exists():
            return skills

        for skill_dir in sorted(self._repo_skills_root.iterdir()):
            if not skill_dir.is_dir():
                continue
            skill_md = skill_dir / "SKILL.md"
            if not skill_md.exists():
                continue
            info = self._parse_skill_frontmatter(skill_md)
            if info:
                skills.append(info)

        return skills

    def get_skill(self, skill_name: str) -> Optional[SkillInfo]:
        """获取单个 skill 的元信息。"""
        skill_dir = self._repo_skills_root / skill_name
        skill_md = skill_dir / "SKILL.md"
        if not skill_md.exists():
            return None
        return self._parse_skill_frontmatter(skill_md)

    # =====================================================
    # Agent Skills — 查询 agent 的 skill 安装状态
    # =====================================================

    async def get_agent_skills(self, agent_id: str) -> list[AgentSkillEntry]:
        """获取 agent 的完整 skill 列表（已安装 + 可安装）。"""
        all_skills = self.get_all_skills()
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise LookupError(f"Agent not found: {agent_id}")

        is_main = MainAgentProfile.is_main_agent(agent_id)

        # 从 agent options 取已手动安装的 skill 列表
        installed_names: set[str] = set()
        if hasattr(agent.options, "installed_skills") and agent.options.installed_skills:
            installed_names = set(agent.options.installed_skills)

        entries: list[AgentSkillEntry] = []
        for skill in all_skills:
            # scope=main 的 skill 只对 main agent 可见
            if skill.scope == "main" and not is_main:
                continue

            is_base = skill.name in self.BASE_SKILL_NAMES
            is_main_only = skill.name in self.MAIN_AGENT_SKILL_NAMES

            if is_base:
                entries.append(AgentSkillEntry(
                    name=skill.name,
                    description=skill.description,
                    scope=skill.scope,
                    tags=skill.tags,
                    installed=True,
                    locked=True,
                ))
            elif is_main_only:
                if is_main:
                    entries.append(AgentSkillEntry(
                        name=skill.name,
                        description=skill.description,
                        scope=skill.scope,
                        tags=skill.tags,
                        installed=True,
                        locked=True,
                    ))
            else:
                entries.append(AgentSkillEntry(
                    name=skill.name,
                    description=skill.description,
                    scope=skill.scope,
                    tags=skill.tags,
                    installed=skill.name in installed_names,
                    locked=False,
                ))

        return entries

    # =====================================================
    # Install / Uninstall
    # =====================================================

    async def install_skill(self, agent_id: str, skill_name: str) -> AgentSkillEntry:
        """为 agent 安装一个 skill。"""
        skill_info = self.get_skill(skill_name)
        if not skill_info:
            raise LookupError(f"Skill not found: {skill_name}")

        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise LookupError(f"Agent not found: {agent_id}")

        is_main = MainAgentProfile.is_main_agent(agent_id)
        if skill_info.scope == "main" and not is_main:
            raise ValueError(f"Skill '{skill_name}' is restricted to main agent")

        if skill_name in self.BASE_SKILL_NAMES or skill_name in self.MAIN_AGENT_SKILL_NAMES:
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be manually installed")

        # 部署到 workspace
        workspace_path = Path(agent.workspace_path)
        deployer = WorkspaceSkillDeployer(agent_id, workspace_path)
        deployer.deploy_skill(skill_name)

        # 更新 installed_skills
        current_skills = list(getattr(agent.options, "installed_skills", None) or [])
        if skill_name not in current_skills:
            current_skills.append(skill_name)
        await agent_repository.update_agent(
            agent_id,
            options={"installed_skills": current_skills},
        )

        logger.info(f"✅ Skill installed: {skill_name} → agent {agent_id}")
        return AgentSkillEntry(
            name=skill_info.name,
            description=skill_info.description,
            scope=skill_info.scope,
            tags=skill_info.tags,
            installed=True,
            locked=False,
        )

    async def uninstall_skill(self, agent_id: str, skill_name: str) -> None:
        """从 agent 卸载一个 skill。"""
        if skill_name in self.BASE_SKILL_NAMES or skill_name in self.MAIN_AGENT_SKILL_NAMES:
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be uninstalled")

        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise LookupError(f"Agent not found: {agent_id}")

        # 从 workspace 移除
        workspace_path = Path(agent.workspace_path)
        deployer = WorkspaceSkillDeployer(agent_id, workspace_path)
        deployer.undeploy_skill(skill_name)

        # 更新 installed_skills
        current_skills = list(getattr(agent.options, "installed_skills", None) or [])
        if skill_name in current_skills:
            current_skills.remove(skill_name)
        await agent_repository.update_agent(
            agent_id,
            options={"installed_skills": current_skills},
        )

        logger.info(f"🗑️ Skill uninstalled: {skill_name} ← agent {agent_id}")

    # =====================================================
    # Frontmatter 解析
    # =====================================================

    @staticmethod
    def _parse_skill_frontmatter(skill_md: Path) -> Optional[SkillInfo]:
        """从 SKILL.md 解析 YAML frontmatter。"""
        try:
            content = skill_md.read_text(encoding="utf-8")
        except OSError:
            return None

        # 匹配 --- 包裹的 frontmatter
        match = re.match(r"^---\s*\n(.*?)\n---", content, re.DOTALL)
        if not match:
            return None

        frontmatter = match.group(1)
        data: dict[str, str] = {}
        for line in frontmatter.strip().splitlines():
            if ":" in line:
                key, _, value = line.partition(":")
                data[key.strip()] = value.strip()

        name = data.get("name", skill_md.parent.name)
        description = data.get("description", "")
        scope = data.get("scope", "any")

        # 简单解析 tags（如 [system, orchestration]）
        tags_raw = data.get("tags", "")
        tags: list[str] = []
        if tags_raw:
            tags = [t.strip().strip("[]\"'") for t in tags_raw.split(",") if t.strip()]

        return SkillInfo(
            name=name,
            description=description,
            scope=scope,
            tags=tags,
        )


skill_service = SkillService()
