# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main_agent_orchestration_service.py
# @Date   ：2026/03/26 01:28
# @Author ：leemysw
# 2026/03/26 01:28   Create
# =====================================================

"""main agent 编排服务。"""

from __future__ import annotations

from typing import Any, Optional

from agent.schema.model_agent import AgentOptions
from agent.service.agent.agent_service import agent_service
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.room.room_service import room_service


class MainAgentOrchestrationService:
    """为 main agent 提供创建成员与组建 room 的高层动作。"""

    async def list_agents(self, include_main: bool = False) -> list[dict[str, Any]]:
        """列出可协作成员。"""
        agents = await agent_service.get_agents()
        agent_items: list[dict[str, Any]] = []

        for agent in agents:
            if not include_main and MainAgentProfile.is_main_agent(agent.agent_id):
                continue
            agent_items.append({
                "agent_id": agent.agent_id,
                "name": agent.name,
                "status": agent.status,
                "workspace_path": agent.workspace_path,
                "model": agent.options.model,
                "skills_enabled": agent.options.skills_enabled,
            })

        return agent_items

    async def create_agent(
        self,
        name: str,
        model: Optional[str] = None,
    ) -> dict[str, Any]:
        """创建新的普通成员 agent。"""
        created_agent = await agent_service.create_agent(
            name=name,
            workspace_path=None,
            options=AgentOptions(
                model=model,
                permission_mode="default",
                skills_enabled=True,
                setting_sources=["user", "project", "local"],
            ),
        )
        return {
            "agent_id": created_agent.agent_id,
            "name": created_agent.name,
            "workspace_path": created_agent.workspace_path,
            "model": created_agent.options.model,
            "skills_enabled": created_agent.options.skills_enabled,
            "status": created_agent.status,
        }

    async def validate_agent_name(self, name: str) -> dict[str, Any]:
        """校验成员名称是否可用。"""
        validation = await agent_service.validate_agent_name(name)
        return validation.model_dump(mode="json")

    async def list_rooms(self, limit: int = 20) -> list[dict[str, Any]]:
        """列出最近房间。"""
        rooms = await room_service.list_rooms(limit=limit)
        room_items: list[dict[str, Any]] = []

        for item in rooms:
            room_items.append({
                "room_id": item.room.id,
                "room_type": item.room.room_type,
                "name": item.room.name,
                "description": item.room.description,
                "member_agent_ids": [
                    member.member_agent_id
                    for member in item.members
                    if member.member_type == "agent" and member.member_agent_id
                ],
                "updated_at": item.room.updated_at.isoformat() if item.room.updated_at else None,
            })

        return room_items

    async def create_room(
        self,
        agent_ids: list[str],
        name: Optional[str] = None,
        title: Optional[str] = None,
        description: str = "",
    ) -> dict[str, Any]:
        """创建新的 room。"""
        context = await room_service.create_room(
            agent_ids=agent_ids,
            name=name,
            description=description,
            title=title,
        )
        return {
            "room_id": context.room.id,
            "room_type": context.room.room_type,
            "room_name": context.room.name,
            "conversation_id": context.conversation.id,
            "conversation_title": context.conversation.title,
            "member_agent_ids": [
                member.member_agent_id
                for member in context.members
                if member.member_type == "agent" and member.member_agent_id
            ],
        }

    async def add_room_member(self, room_id: str, agent_id: str) -> dict[str, Any]:
        """向已有多人 room 追加成员。"""
        context = await room_service.add_agent_member(room_id=room_id, agent_id=agent_id)
        return {
            "room_id": context.room.id,
            "room_name": context.room.name,
            "conversation_id": context.conversation.id,
            "member_agent_ids": [
                member.member_agent_id
                for member in context.members
                if member.member_type == "agent" and member.member_agent_id
            ],
        }


main_agent_orchestration_service = MainAgentOrchestrationService()
