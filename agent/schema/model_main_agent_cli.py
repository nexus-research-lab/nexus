# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_main_agent_cli.py
# @Date   ：2026/03/26 11:10
# @Author ：leemysw
# 2026/03/26 11:10   Create
# =====================================================

"""主智能体 CLI 参数类型。"""

from __future__ import annotations

from pydantic import Field

from agent.infra.schemas.model_cython import AModel


class ListAgentsCommand(AModel):
    """列出成员命令。"""

    include_main: bool = Field(default=False, description="是否包含主智能体")


class ValidateAgentNameCommand(AModel):
    """校验成员名称命令。"""

    name: str = Field(..., description="待校验的 agent 名称")


class CreateAgentCommand(AModel):
    """创建成员命令。"""

    name: str = Field(..., description="agent 名称")
    provider: str | None = Field(default=None, description="Provider 标识")


class GetAgentCommand(AModel):
    """读取成员命令。"""

    agent_id: str = Field(..., description="agent_id")


class DeleteAgentCommand(AModel):
    """删除成员命令。"""

    agent_id: str = Field(..., description="agent_id")


class GetAgentSessionsCommand(AModel):
    """读取成员会话命令。"""

    agent_id: str = Field(..., description="agent_id")


class ListRoomsCommand(AModel):
    """列出房间命令。"""

    limit: int = Field(default=20, description="数量上限")


class GetRoomCommand(AModel):
    """读取房间命令。"""

    room_id: str = Field(..., description="room_id")


class GetRoomContextsCommand(AModel):
    """读取房间上下文命令。"""

    room_id: str = Field(..., description="room_id")


class CreateRoomCommand(AModel):
    """创建 room 命令。"""

    agent_ids: list[str] = Field(..., description="成员 agent_id 列表")
    name: str | None = Field(default=None, description="room 名称")
    title: str | None = Field(default=None, description="首条对话标题")
    description: str = Field(default="", description="room 描述")


class UpdateRoomCommand(AModel):
    """更新 room 命令。"""

    room_id: str = Field(..., description="room_id")
    name: str | None = Field(default=None, description="room 名称")
    title: str | None = Field(default=None, description="主对话标题")
    description: str | None = Field(default=None, description="room 描述")


class AddRoomMemberCommand(AModel):
    """追加 room 成员命令。"""

    room_id: str = Field(..., description="room_id")
    agent_id: str = Field(..., description="待追加的 agent_id")


class RemoveRoomMemberCommand(AModel):
    """移除 room 成员命令。"""

    room_id: str = Field(..., description="room_id")
    agent_id: str = Field(..., description="待移除的 agent_id")


class DeleteRoomCommand(AModel):
    """删除 room 命令。"""

    room_id: str = Field(..., description="room_id")


class ListWorkspaceFilesCommand(AModel):
    """列出工作区文件命令。"""

    agent_id: str = Field(..., description="agent_id")


class ReadWorkspaceFileCommand(AModel):
    """读取工作区文件命令。"""

    agent_id: str = Field(..., description="agent_id")
    path: str = Field(..., description="工作区相对路径")


class UpdateWorkspaceFileCommand(AModel):
    """更新工作区文件命令。"""

    agent_id: str = Field(..., description="agent_id")
    path: str = Field(..., description="工作区相对路径")
    content: str = Field(..., description="文件内容")


class CreateWorkspaceEntryCommand(AModel):
    """创建工作区条目命令。"""

    agent_id: str = Field(..., description="agent_id")
    path: str = Field(..., description="工作区相对路径")
    entry_type: str = Field(..., description="file 或 dir")
    content: str = Field(default="", description="文件内容")


class RenameWorkspaceEntryCommand(AModel):
    """重命名工作区条目命令。"""

    agent_id: str = Field(..., description="agent_id")
    path: str = Field(..., description="原路径")
    new_path: str = Field(..., description="新路径")


class DeleteWorkspaceEntryCommand(AModel):
    """删除工作区条目命令。"""

    agent_id: str = Field(..., description="agent_id")
    path: str = Field(..., description="工作区相对路径")


class GetAgentSkillsCommand(AModel):
    """读取成员技能命令。"""

    agent_id: str = Field(..., description="agent_id")


class InstallSkillCommand(AModel):
    """安装技能命令。"""

    agent_id: str = Field(..., description="agent_id")
    skill_name: str = Field(..., description="skill 名称")


class UninstallSkillCommand(AModel):
    """卸载技能命令。"""

    agent_id: str = Field(..., description="agent_id")
    skill_name: str = Field(..., description="skill 名称")
