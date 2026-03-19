# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：backfill_service.py
# @Date   ：2026/3/19 00:26
# @Author ：leemysw
# 2026/3/19 00:26   Create
# =====================================================

"""旧数据回填服务。"""

from __future__ import annotations

from agent.service.persistence.agent_persistence_service import agent_persistence_service
from agent.service.persistence.legacy_sync_bridge import build_agent_aggregate_from_legacy
from agent.service.agent.agent_manager import agent_manager


class PersistenceBackfillService:
    """负责将旧文件存储数据回填到新数据库。"""

    async def sync_all_agents(self) -> int:
        """回填全部 Agent 聚合。"""
        synced_count = 0
        agents = await agent_manager.get_all_agents()
        for agent in agents:
            aggregate = build_agent_aggregate_from_legacy(agent)
            existing = await agent_persistence_service.get_agent_aggregate(agent.agent_id)
            if existing is None:
                await agent_persistence_service.create_agent_aggregate(aggregate)
                synced_count += 1
                continue

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
            synced_count += 1
        return synced_count

    async def sync_all_dm_sessions(self) -> int:
        """回填全部 DM 会话。"""
        from agent.service.session.session_store import session_store

        synced_count = 0
        sessions = await session_store.get_all_sessions()
        for session_info in sessions:
            await session_store.sync_session_to_sql(session_info)
            synced_count += 1
        return synced_count

    async def sync_all_messages(self) -> int:
        """回填全部消息索引。"""
        from agent.service.session.session_store import session_store

        synced_count = 0
        sessions = await session_store.get_all_sessions()
        for session_info in sessions:
            messages = await session_store.get_session_messages(session_info.session_key)
            await session_store.sync_session_messages_to_sql(session_info)
            synced_count += len(messages)
        return synced_count


persistence_backfill_service = PersistenceBackfillService()
