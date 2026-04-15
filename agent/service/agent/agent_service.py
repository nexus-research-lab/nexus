# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_service.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""Agent 应用服务。"""

from typing import Any, List, Optional

from agent.schema.model_agent import AAgent, ValidateAgentNameResponse
from agent.schema.model_cost import AgentCostSummary
from agent.service.channels.ws.ws_chat_task_registry import ws_chat_task_registry
from agent.schema.model_session import ASession
from agent.service.agent.agent_manager import agent_manager
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.session.session_manager import session_manager
from agent.service.session.session_store import session_store


class AgentService:
    """负责编排 Agent 相关用例。"""

    async def get_agents(self, include_main: bool = True) -> List[AAgent]:
        """获取所有 Agent。"""
        agents = await agent_manager.get_all_agents()
        if not include_main:
            agents = MainAgentProfile.filter_regular_agents(
                agents,
                lambda agent: agent.agent_id,
            )
        return agents

    async def create_agent(
        self,
        name: str,
        options,
        avatar: Optional[str] = None,
        description: Optional[str] = None,
        vibe_tags: Optional[list[str]] = None,
    ) -> AAgent:
        """创建 Agent。"""
        agent = await agent_manager.create_agent(
            name=name,
            options=options,
            avatar=avatar,
            description=description,
            vibe_tags=vibe_tags,
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

    async def update_agent(
        self,
        agent_id: str,
        name: Optional[str],
        options,
        avatar: Optional[str] = None,
        description: Optional[str] = None,
        vibe_tags: Optional[list[str]] = None,
    ) -> AAgent:
        """更新 Agent 配置并刷新活跃会话。"""
        await self.get_agent(agent_id)
        success = await agent_manager.update_agent(
            agent_id=agent_id,
            name=name,
            options=options,
            avatar=avatar,
            description=description,
            vibe_tags=vibe_tags,
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

    async def get_agent_runtime_statuses(self) -> List[dict[str, Any]]:
        """获取全部成员的运行态快照。"""
        agents = await self.get_agents(include_main=True)
        agent_ids = [agent.agent_id for agent in agents]
        return ws_chat_task_registry.build_agent_runtime_snapshots(agent_ids)

    async def get_agent_cost_summary(self, agent_id: str) -> AgentCostSummary:
        """获取 Agent 成本汇总。"""
        await self.get_agent(agent_id)
        return await session_store.get_agent_cost_summary(agent_id)


agent_service = AgentService()
