# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_service.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""Agent 应用服务。"""

from typing import List, Optional, Tuple

from agent.infra.agent.session_manager import session_manager
from agent.schema.model_agent import AAgent, ValidateAgentNameResponse
from agent.service.agent.agent_manager import agent_manager
from agent.schema.model_cost import AgentCostSummary
from agent.schema.model_session import ASession
from agent.service.session.session_store import session_store


class AgentService:
    """负责编排 Agent 与 Workspace 相关用例。"""

    async def get_agents(self) -> List[AAgent]:
        """获取所有 Agent。"""
        return await agent_manager.get_all_agents()

    async def create_agent(self, name: str, workspace_path: Optional[str], options) -> AAgent:
        """创建 Agent。"""
        agent = await agent_manager.create_agent(
            name=name,
            workspace_path=workspace_path,
            options=options,
        )
        if not agent:
            raise RuntimeError("Failed to create agent")
        return agent

    async def get_agent(self, agent_id: str) -> AAgent:
        """获取 Agent。"""
        agent = await agent_manager.get_agent(agent_id)
        if not agent:
            raise LookupError("Agent not found")
        return agent

    async def update_agent(self, agent_id: str, name: Optional[str], options) -> AAgent:
        """更新 Agent 配置并刷新活跃会话。"""
        await self.get_agent(agent_id)
        success = await agent_manager.update_agent(
            agent_id=agent_id,
            name=name,
            options=options,
        )
        if not success:
            raise RuntimeError("Failed to update agent")

        await session_manager.refresh_agent_sessions(agent_id)
        updated = await agent_manager.get_agent(agent_id)
        if not updated:
            raise RuntimeError("Failed to retrieve updated agent")
        return updated

    async def delete_agent(self, agent_id: str) -> None:
        """删除 Agent。"""
        success = await agent_manager.delete_agent(agent_id)
        if not success:
            raise LookupError("Agent not found")

    async def validate_agent_name(
        self,
        name: str,
        exclude_agent_id: Optional[str] = None,
    ) -> ValidateAgentNameResponse:
        """校验 Agent 名称。"""
        return await agent_manager.validate_agent_name(name, exclude_agent_id=exclude_agent_id)

    async def get_agent_sessions(self, agent_id: str) -> List[ASession]:
        """获取 Agent 下的所有会话。"""
        await self.get_agent(agent_id)
        all_sessions = await session_store.get_all_sessions()
        return [session for session in all_sessions if session.agent_id == agent_id]

    async def get_agent_cost_summary(self, agent_id: str) -> AgentCostSummary:
        """获取 Agent 成本汇总。"""
        await self.get_agent(agent_id)
        return await session_store.get_agent_cost_summary(agent_id)

    async def get_workspace_files(self, agent_id: str) -> list[dict]:
        """获取 workspace 文件列表。"""
        workspace = await self._get_workspace(agent_id)
        return workspace.list_files()

    async def get_workspace_file(self, agent_id: str, path: str) -> str:
        """读取 workspace 文件。"""
        workspace = await self._get_workspace(agent_id)
        return workspace.read_relative_file(path)

    async def update_workspace_file(self, agent_id: str, path: str, content: str) -> str:
        """更新 workspace 文件并刷新活跃会话。"""
        workspace = await self._get_workspace(agent_id)
        saved_path = workspace.write_relative_file(path, content, source="api")
        await session_manager.refresh_agent_sessions(agent_id)
        return saved_path

    async def create_workspace_entry(
        self,
        agent_id: str,
        path: str,
        entry_type: str,
        content: str = "",
    ) -> str:
        """创建 workspace 条目。"""
        workspace = await self._get_workspace(agent_id)
        created_path = workspace.create_entry(
            relative_path=path,
            entry_type=entry_type,
            content=content,
        )
        await session_manager.refresh_agent_sessions(agent_id)
        return created_path

    async def rename_workspace_entry(self, agent_id: str, path: str, new_path: str) -> Tuple[str, str]:
        """重命名 workspace 条目。"""
        workspace = await self._get_workspace(agent_id)
        renamed_paths = workspace.rename_entry(
            relative_path=path,
            new_relative_path=new_path,
        )
        await session_manager.refresh_agent_sessions(agent_id)
        return renamed_paths

    async def delete_workspace_entry(self, agent_id: str, path: str) -> str:
        """删除 workspace 条目。"""
        workspace = await self._get_workspace(agent_id)
        deleted_path = workspace.delete_entry(path)
        await session_manager.refresh_agent_sessions(agent_id)
        return deleted_path

    async def _get_workspace(self, agent_id: str):
        """获取 Agent 绑定的 workspace。"""
        await self.get_agent(agent_id)
        return await agent_manager.get_agent_workspace(agent_id)


agent_service = AgentService()
