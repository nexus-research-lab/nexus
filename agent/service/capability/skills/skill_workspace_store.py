# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_workspace_store.py
# @Date   ：2026/4/2 11:58
# @Author ：leemysw
# 2026/4/2 11:58   Create
# =====================================================

"""Skill workspace 读写层。"""

from __future__ import annotations

from pathlib import Path

from agent.service.agent.agent_repository import agent_repository
from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer


class SkillWorkspaceStore:
    """负责按 Agent 读写 workspace 中的 skill 部署状态。"""

    async def deploy_skill(self, agent_id: str, skill_name: str, source_dir: Path) -> None:
        agent = await self.get_agent(agent_id)
        WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path)).deploy_skill(
            skill_name,
            source_dir=source_dir,
        )

    async def undeploy_skill(self, agent_id: str, skill_name: str) -> None:
        agent = await self.get_agent(agent_id)
        WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path)).undeploy_skill(skill_name)

    async def get_deployed_skill_names(self, agent_id: str) -> set[str]:
        agent = await self.get_agent(agent_id)
        deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
        return set(deployer.list_deployed())

    async def sync_skill_to_installed_agents(self, skill_name: str, source_dir: Path) -> None:
        for agent in await agent_repository.get_all_agents():
            deployer = WorkspaceSkillDeployer(agent.agent_id, Path(agent.workspace_path))
            if skill_name not in deployer.list_deployed():
                continue
            deployer.deploy_skill(skill_name, source_dir=source_dir)

    async def undeploy_skill_from_all_agents(self, skill_name: str) -> None:
        for agent in await agent_repository.get_all_agents():
            deployer = WorkspaceSkillDeployer(agent.agent_id, Path(agent.workspace_path))
            if skill_name not in deployer.list_deployed():
                continue
            deployer.undeploy_skill(skill_name)

    async def get_agent(self, agent_id: str):
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise LookupError(f"Agent not found: {agent_id}")
        return agent
