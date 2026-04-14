# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main_agent_orchestration_service.py
# @Date   ：2026/03/26 01:28
# @Author ：leemysw
# 2026/03/26 01:28   Create
# =====================================================

"""主智能体编排服务。"""

from __future__ import annotations

from typing import Any, Optional

from agent.schema.model_agent import AgentOptions
from agent.service.agent.agent_service import agent_service
from agent.service.capability.skills.skill_service import skill_service
from agent.service.room.room_service import room_service
from agent.service.workspace.workspace_service import workspace_service


class MainAgentOrchestrationService:
    """为主智能体提供创建成员与组建 room 的高层动作。"""

    async def list_agents(self, include_main: bool = False) -> list[dict[str, Any]]:
        """列出可协作成员。"""
        agents = await agent_service.get_agents(include_main=include_main)
        agent_items: list[dict[str, Any]] = []

        for agent in agents:
            agent_items.append({
                "agent_id": agent.agent_id,
                "name": agent.name,
                "status": agent.status,
                "workspace_path": agent.workspace_path,
                "provider": agent.options.provider,
            })

        return agent_items

    async def create_agent(
        self,
        name: str,
        provider: Optional[str] = None,
    ) -> dict[str, Any]:
        """创建新的普通成员 agent。"""
        created_agent = await agent_service.create_agent(
            name=name,
            options=AgentOptions(
                provider=provider,
                permission_mode="default",
                setting_sources=["project"],
            ),
        )
        return {
            "agent_id": created_agent.agent_id,
            "name": created_agent.name,
            "workspace_path": created_agent.workspace_path,
            "provider": created_agent.options.provider,
            "status": created_agent.status,
        }

    async def get_agent(self, agent_id: str) -> dict[str, Any]:
        """读取单个成员信息。"""
        agent = await agent_service.get_agent(agent_id)
        return agent.model_dump(mode="json")

    async def delete_agent(self, agent_id: str) -> dict[str, Any]:
        """删除成员。"""
        await agent_service.delete_agent(agent_id)
        return {"success": True, "agent_id": agent_id}

    async def validate_agent_name(self, name: str) -> dict[str, Any]:
        """校验成员名称是否可用。"""
        validation = await agent_service.validate_agent_name(name)
        return validation.model_dump(mode="json")

    async def get_agent_sessions(self, agent_id: str) -> list[dict[str, Any]]:
        """读取成员下的全部会话。"""
        sessions = await agent_service.get_agent_sessions(agent_id)
        return [session.model_dump(mode="json") for session in sessions]

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

    async def get_room(self, room_id: str) -> dict[str, Any]:
        """读取单个房间。"""
        room = await room_service.get_room(room_id)
        return room.model_dump(mode="json")

    async def get_room_contexts(self, room_id: str) -> list[dict[str, Any]]:
        """读取房间上下文。"""
        contexts = await room_service.get_room_contexts(room_id)
        return [context.model_dump(mode="json") for context in contexts]

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

    async def update_room(
        self,
        room_id: str,
        name: Optional[str] = None,
        title: Optional[str] = None,
        description: Optional[str] = None,
    ) -> dict[str, Any]:
        """更新房间信息。"""
        context = await room_service.update_room(
            room_id=room_id,
            name=name,
            description=description,
            title=title,
        )
        return context.model_dump(mode="json")

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

    async def remove_room_member(self, room_id: str, agent_id: str) -> dict[str, Any]:
        """从房间中移除成员。"""
        context = await room_service.remove_agent_member(room_id=room_id, agent_id=agent_id)
        return context.model_dump(mode="json")

    async def delete_room(self, room_id: str) -> dict[str, Any]:
        """删除房间。"""
        await room_service.delete_room(room_id)
        return {"success": True, "room_id": room_id}

    async def list_workspace_files(self, agent_id: str) -> list[dict[str, Any]]:
        """列出成员工作区文件。"""
        return await workspace_service.get_workspace_files(agent_id)

    async def read_workspace_file(self, agent_id: str, path: str) -> dict[str, Any]:
        """读取成员工作区文件。"""
        content = await workspace_service.get_workspace_file(agent_id, path)
        return {"agent_id": agent_id, "path": path, "content": content}

    async def update_workspace_file(
        self,
        agent_id: str,
        path: str,
        content: str,
    ) -> dict[str, Any]:
        """更新成员工作区文件。"""
        saved_path = await workspace_service.update_workspace_file(agent_id, path, content)
        return {"agent_id": agent_id, "path": saved_path}

    async def create_workspace_entry(
        self,
        agent_id: str,
        path: str,
        entry_type: str,
        content: str = "",
    ) -> dict[str, Any]:
        """创建成员工作区条目。"""
        created_path = await workspace_service.create_workspace_entry(
            agent_id=agent_id,
            path=path,
            entry_type=entry_type,
            content=content,
        )
        return {"agent_id": agent_id, "path": created_path}

    async def rename_workspace_entry(
        self,
        agent_id: str,
        path: str,
        new_path: str,
    ) -> dict[str, Any]:
        """重命名成员工作区条目。"""
        old_path, renamed_path = await workspace_service.rename_workspace_entry(
            agent_id=agent_id,
            path=path,
            new_path=new_path,
        )
        return {"agent_id": agent_id, "path": old_path, "new_path": renamed_path}

    async def delete_workspace_entry(self, agent_id: str, path: str) -> dict[str, Any]:
        """删除成员工作区条目。"""
        deleted_path = await workspace_service.delete_workspace_entry(agent_id, path)
        return {"agent_id": agent_id, "path": deleted_path}

    async def list_skills(self) -> list[dict[str, Any]]:
        """列出可安装技能。"""
        skills = await skill_service.get_all_skills()
        return [skill.model_dump(mode="json") for skill in skills]

    async def get_agent_skills(self, agent_id: str) -> list[dict[str, Any]]:
        """读取成员技能状态。"""
        skills = await skill_service.get_agent_skills(agent_id)
        return [skill.model_dump(mode="json") for skill in skills]

    async def install_skill(self, agent_id: str, skill_name: str) -> dict[str, Any]:
        """为成员安装技能。"""
        entry = await skill_service.install_skill(agent_id, skill_name)
        return entry.model_dump(mode="json")

    async def uninstall_skill(self, agent_id: str, skill_name: str) -> dict[str, Any]:
        """为成员卸载技能。"""
        await skill_service.uninstall_skill(agent_id, skill_name)
        return {"success": True, "agent_id": agent_id, "skill_name": skill_name}


main_agent_orchestration_service = MainAgentOrchestrationService()
