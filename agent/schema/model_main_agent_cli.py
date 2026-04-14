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
from pydantic import model_validator

from agent.infra.schemas.model_cython import AModel
from agent.schema.model_automation import (
    AutomationSessionTarget,
    AutomationSessionTargetKind,
    AutomationSessionWakeMode,
)


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


class ListScheduledTasksCommand(AModel):
    """列出定时任务命令。"""

    agent_id: str | None = Field(default=None, description="可选的 agent_id 过滤")


class CreateScheduledTaskCommand(AModel):
    """创建定时任务命令。"""

    name: str = Field(..., description="任务名称")
    agent_id: str = Field(..., description="目标 agent_id")
    instruction: str = Field(..., description="任务提示词")
    session_target_kind: AutomationSessionTargetKind | None = Field(
        default=None,
        description="会话目标类型 bound / named / main / isolated",
    )
    session_key: str | None = Field(default=None, description="bound 模式的 session_key")
    named_session_key: str | None = Field(default=None, description="named 模式的命名会话 key")
    wake_mode: AutomationSessionWakeMode = Field(
        default="next-heartbeat",
        description="main 模式的唤醒方式",
    )
    schedule_kind: str = Field(..., description="调度类型 every / cron / at")
    interval_seconds: int | None = Field(default=None, description="every 模式的秒数")
    cron_expression: str | None = Field(default=None, description="cron 表达式")
    run_at: str | None = Field(default=None, description="单次执行时间")
    timezone: str = Field(default="Asia/Shanghai", description="IANA 时区")
    enabled: bool = Field(default=True, description="创建后是否启用")

    @model_validator(mode="after")
    def validate_session_target(self) -> "CreateScheduledTaskCommand":
        """把 CLI 输入统一收敛成 AutomationSessionTarget 语义。"""
        self.session_key = (self.session_key or "").strip() or None
        self.named_session_key = (self.named_session_key or "").strip() or None

        if self.session_target_kind is None:
            if self.session_key is not None and self.named_session_key is not None:
                raise ValueError("session_key and named_session_key cannot be provided together")
            if self.named_session_key is not None:
                self.session_target_kind = "named"
            elif self.session_key is not None:
                self.session_target_kind = "bound"
            else:
                self.session_target_kind = "isolated"

        self.build_session_target()
        return self

    def build_session_target(self) -> AutomationSessionTarget:
        return AutomationSessionTarget(
            kind=self.session_target_kind or "isolated",
            bound_session_key=self.session_key,
            named_session_key=self.named_session_key,
            wake_mode=self.wake_mode,
        )


class DeleteScheduledTaskCommand(AModel):
    """删除定时任务命令。"""

    job_id: str = Field(..., description="job_id")


class SetScheduledTaskEnabledCommand(AModel):
    """启用或禁用定时任务命令。"""

    job_id: str = Field(..., description="job_id")
    enabled: bool = Field(..., description="是否启用")


class RunScheduledTaskCommand(AModel):
    """立即运行定时任务命令。"""

    job_id: str = Field(..., description="job_id")


class GetScheduledTaskRunsCommand(AModel):
    """读取定时任务运行记录命令。"""

    job_id: str = Field(..., description="job_id")
