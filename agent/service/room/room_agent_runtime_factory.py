# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_agent_runtime_factory.py
# @Date   ：2026/03/25 23:18
# @Author ：leemysw
# 2026/03/25 23:18   Create
# =====================================================

"""Room 运行时成员工厂。"""

from __future__ import annotations

import hashlib
import json

from agent.schema.model_agent_persistence import (
    AgentAggregate,
    AgentRecord,
    CreateAgentAggregate,
    ProfileRecord,
    RuntimeRecord,
)
from agent.schema.model_chat_persistence import SessionRecord
from agent.service.agent.agent_manager import agent_manager
from agent.service.agent.agent_name_policy import AgentNamePolicy
from agent.service.repository.agent_repository_service import (
    agent_persistence_service,
)
from agent.utils.utils import random_uuid


def _stable_id(prefix: str, raw_value: str) -> str:
    """基于稳定输入生成短 ID。"""
    digest = hashlib.sha1(raw_value.encode("utf-8")).hexdigest()[:20]
    return f"{prefix}_{digest}"


class RoomAgentRuntimeFactory:
    """负责把 room 成员转换成可运行会话。"""

    async def ensure_agent_aggregate(self, agent_id: str) -> AgentAggregate:
        """确保 agent 已同步到持久化层。"""
        agent = await agent_manager.get_agent(agent_id)
        if agent is None:
            raise LookupError(f"Agent not found: {agent_id}")

        aggregate = await agent_persistence_service.get_agent_aggregate(agent_id)
        if aggregate is not None:
            return aggregate

        options = agent.options.model_dump(exclude_none=True)
        slug = AgentNamePolicy.build_workspace_dir_name(agent.name)
        create_payload = CreateAgentAggregate(
            agent=AgentRecord(
                id=agent.agent_id,
                slug=slug,
                name=agent.name,
                description="",
                definition="",
                status=agent.status,
                workspace_path=agent.workspace_path,
            ),
            profile=ProfileRecord(
                id=_stable_id("profile", agent.agent_id),
                agent_id=agent.agent_id,
                display_name=agent.name,
                headline="",
                profile_markdown="",
            ),
            runtime=RuntimeRecord(
                id=_stable_id("runtime", agent.agent_id),
                agent_id=agent.agent_id,
                model=options.get("model"),
                permission_mode=options.get("permission_mode"),
                allowed_tools_json=json.dumps(options.get("allowed_tools") or [], ensure_ascii=False),
                disallowed_tools_json=json.dumps(options.get("disallowed_tools") or [], ensure_ascii=False),
                mcp_servers_json=json.dumps(options.get("mcp_servers") or {}, ensure_ascii=False),
                max_turns=options.get("max_turns"),
                max_thinking_tokens=options.get("max_thinking_tokens"),
                skills_enabled=bool(options.get("skills_enabled", False)),
                setting_sources_json=json.dumps(options.get("setting_sources") or [], ensure_ascii=False),
                runtime_version=1,
            ),
        )
        return await agent_persistence_service.create_agent_aggregate(create_payload)

    def build_session_record(
        self,
        conversation_id: str,
        agent: AgentAggregate,
    ) -> SessionRecord:
        """为对话构造默认主会话。"""
        return SessionRecord(
            id=random_uuid(),
            conversation_id=conversation_id,
            agent_id=agent.agent.id,
            runtime_id=agent.runtime.id,
            version_no=1,
            branch_key="main",
            is_primary=True,
            status="active",
        )


room_agent_runtime_factory = RoomAgentRuntimeFactory()
