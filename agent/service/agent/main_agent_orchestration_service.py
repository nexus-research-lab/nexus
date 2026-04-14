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

from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationCronSource,
    AutomationDeliveryTarget,
    AutomationSessionTarget,
)
from agent.schema.model_agent import AgentOptions
from agent.service.agent.agent_service import agent_service
from agent.service.capability.scheduled.scheduled_task_service import (
    scheduled_task_service,
)
from agent.service.capability.skills.skill_service import skill_service
from agent.service.room.room_service import room_service
from agent.service.session.session_store import session_store
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
                "model": agent.options.model,
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
            options=AgentOptions(
                model=model,
                permission_mode="default",
                setting_sources=["project"],
            ),
        )
        return {
            "agent_id": created_agent.agent_id,
            "name": created_agent.name,
            "workspace_path": created_agent.workspace_path,
            "model": created_agent.options.model,
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

    async def list_scheduled_tasks(self, agent_id: str | None = None) -> list[dict[str, Any]]:
        """列出定时任务。"""
        tasks = await scheduled_task_service.list_tasks(agent_id=agent_id)
        return [task.model_dump(mode="json") for task in tasks]

    async def create_scheduled_task(
        self,
        *,
        name: str,
        agent_id: str,
        instruction: str,
        session_target: AutomationSessionTarget | None = None,
        source: AutomationCronSource | None = None,
        session_key: str | None = None,
        schedule_kind: str,
        interval_seconds: int | None = None,
        cron_expression: str | None = None,
        run_at: str | None = None,
        timezone: str = "Asia/Shanghai",
        enabled: bool = True,
    ) -> dict[str, Any]:
        """为指定 agent 创建定时任务。"""
        if session_target is not None and session_key is not None:
            raise ValueError("session_target and session_key cannot be provided together")

        resolved_session_target = session_target
        if resolved_session_target is None:
            resolved_session_target = (
                AutomationSessionTarget(
                    kind="bound",
                    bound_session_key=session_key,
                    wake_mode="next-heartbeat",
                )
                if session_key is not None
                else AutomationSessionTarget()
            )

        if resolved_session_target.kind == "bound":
            bound_session_key = resolved_session_target.bound_session_key
            session = await session_store.get_session_info(bound_session_key)
            # 中文注释：只有 bound 目标真正依赖现存会话，因此仅在该模式下做存在性与归属校验。
            if session is None:
                raise ValueError(f"Session not found: {bound_session_key}")
            if (session.agent_id or "").strip() != agent_id:
                raise ValueError(f"Session {bound_session_key} does not belong to agent {agent_id}")

        schedule = AutomationCronSchedule(
            kind=schedule_kind,
            interval_seconds=interval_seconds,
            cron_expression=cron_expression,
            run_at=run_at,
            timezone=timezone,
        )
        payload = AutomationCronJobCreate(
            name=name,
            agent_id=agent_id,
            schedule=schedule,
            instruction=instruction,
            session_target=resolved_session_target,
            delivery=AutomationDeliveryTarget(mode="none"),
            source=source or AutomationCronSource(
                kind="agent",
                context_type="agent",
                context_id=agent_id,
            ),
            enabled=enabled,
        )
        task = await scheduled_task_service.create_task(payload)
        return task.model_dump(mode="json")

    async def delete_scheduled_task(self, job_id: str) -> dict[str, Any]:
        """删除定时任务。"""
        await scheduled_task_service.delete_task(job_id)
        return {"success": True, "job_id": job_id}

    async def set_scheduled_task_enabled(self, job_id: str, *, enabled: bool) -> dict[str, Any]:
        """启用或禁用定时任务。"""
        task = await scheduled_task_service.set_task_enabled(job_id, enabled=enabled)
        return task.model_dump(mode="json")

    async def run_scheduled_task(self, job_id: str) -> dict[str, Any]:
        """立即运行定时任务。"""
        result = await scheduled_task_service.run_task_now(job_id)
        return result.model_dump(mode="json")

    async def get_scheduled_task_runs(self, job_id: str) -> list[dict[str, Any]]:
        """读取定时任务运行记录。"""
        runs = await scheduled_task_service.list_task_runs(job_id)
        return [run.model_dump(mode="json") for run in runs]


main_agent_orchestration_service = MainAgentOrchestrationService()
