# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_service.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""Agent 应用服务。"""

from typing import List, Optional

from agent.service.persistence.agent_persistence_service import (
    agent_persistence_service,
)
from agent.service.persistence.legacy_sync_bridge import (
    build_agent_aggregate_from_legacy,
)
from agent.service.session.session_manager import session_manager
from agent.schema.model_agent import AAgent, ValidateAgentNameResponse
from agent.service.agent.agent_manager import agent_manager
from agent.schema.model_cost import AgentCostSummary
from agent.schema.model_session import ASession
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class AgentService:
    """负责编排 Agent 相关用例。"""

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
        await self._sync_agent_to_sql(agent)
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
        await self._sync_agent_to_sql(updated)
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

    async def _sync_agent_to_sql(self, agent: AAgent) -> None:
        """将旧 Agent 模型同步写入新数据库。"""
        try:
            existing = await agent_persistence_service.get_agent_aggregate(agent.agent_id)
            aggregate = build_agent_aggregate_from_legacy(agent)
            if existing is None:
                await agent_persistence_service.create_agent_aggregate(aggregate)
                return

            await agent_persistence_service.update_agent_entity(
                agent.agent_id,
                slug=aggregate.agent.slug,
                name=aggregate.agent.name,
                status=aggregate.agent.status,
                workspace_path=aggregate.agent.workspace_path,
            )
            await agent_persistence_service.update_agent_profile(
                agent.agent_id,
                display_name=aggregate.profile.display_name,
            )
            await agent_persistence_service.update_agent_runtime(
                agent.agent_id,
                model=aggregate.runtime.model,
                permission_mode=aggregate.runtime.permission_mode,
                allowed_tools_json=aggregate.runtime.allowed_tools_json,
                disallowed_tools_json=aggregate.runtime.disallowed_tools_json,
                mcp_servers_json=aggregate.runtime.mcp_servers_json,
                max_turns=aggregate.runtime.max_turns,
                max_thinking_tokens=aggregate.runtime.max_thinking_tokens,
                skills_enabled=aggregate.runtime.skills_enabled,
                setting_sources_json=aggregate.runtime.setting_sources_json,
            )
        except Exception as exc:
            # 新库同步失败不应阻断现有文件存储主链路。
            logger.warning(f"⚠️ Agent SQL 同步失败: agent={agent.agent_id}, error={exc}")


agent_service = AgentService()
