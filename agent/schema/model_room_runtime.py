# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_room_runtime.py
# @Date   ：2026/03/26
# @Author ：OpenAI
# =====================================================

"""Meeting-room Room Runtime 模型。"""

from __future__ import annotations

from datetime import datetime
from typing import Any, Optional

from pydantic import BaseModel, Field

from agent.schema.model_chat_persistence import MemberRecord, RoomRecord
from agent.schema.model_protocol import ProtocolRunDetail, ProtocolRunListItem


class RoomMemberSpec(BaseModel):
    """Room 成员规格。"""

    existing_agent_id: Optional[str] = Field(default=None, description="已有 agent")
    create_spec: Optional[dict[str, Any]] = Field(default=None, description="临时成员创建参数")
    role_hint: Optional[str] = Field(default=None, description="角色提示")
    source: str = Field(default="existing", description="成员来源")
    workspace_binding: bool = Field(default=True, description="是否绑定 workspace")


class RoomEventRecord(BaseModel):
    """Room 时间线事件。"""

    id: str = Field(..., description="事件 ID")
    room_id: str = Field(..., description="所属房间")
    run_id: Optional[str] = Field(default=None, description="关联运行实例")
    event_type: str = Field(..., description="事件类型")
    actor_member_id: Optional[str] = Field(default=None, description="事件发起成员")
    actor_agent_id: Optional[str] = Field(default=None, description="事件发起 agent")
    channel_id: Optional[str] = Field(default=None, description="关联频道")
    visibility: str = Field(default="public", description="可见性")
    audience_member_ids: list[str] = Field(default_factory=list, description="可见成员")
    title: Optional[str] = Field(default=None, description="标题")
    body: Optional[str] = Field(default=None, description="正文")
    metadata: dict[str, Any] = Field(default_factory=dict, description="元数据")
    created_at: datetime = Field(default_factory=datetime.now, description="创建时间")


class RoomArtifactRecord(BaseModel):
    """Room 产物卡片。"""

    id: str = Field(..., description="产物 ID")
    owner_member_id: Optional[str] = Field(default=None, description="所属成员")
    owner_agent_id: Optional[str] = Field(default=None, description="所属 agent")
    kind: str = Field(default="note", description="产物类型")
    title: str = Field(..., description="产物标题")
    summary: str = Field(default="", description="产物摘要")
    workspace_ref: Optional[str] = Field(default=None, description="关联 workspace 路径")
    source_ref: Optional[str] = Field(default=None, description="来源引用")
    created_at: datetime = Field(default_factory=datetime.now, description="创建时间")


class RoomTaskRecord(BaseModel):
    """Room 任务记录。"""

    id: str = Field(..., description="任务 ID")
    title: str = Field(..., description="任务标题")
    summary: str = Field(default="", description="任务描述")
    status: str = Field(default="open", description="任务状态")
    assignee_member_id: Optional[str] = Field(default=None, description="受派成员")
    assignee_agent_id: Optional[str] = Field(default=None, description="受派 agent")
    created_by: Optional[str] = Field(default=None, description="创建者")
    workspace_path: Optional[str] = Field(default=None, description="任务落盘路径")
    metadata: dict[str, Any] = Field(default_factory=dict, description="附加信息")
    created_at: datetime = Field(default_factory=datetime.now, description="创建时间")
    updated_at: datetime = Field(default_factory=datetime.now, description="更新时间")


class RoomRuntimeView(BaseModel):
    """统一 room 运行时视图。"""

    room: RoomRecord = Field(..., description="房间实体")
    members: list[MemberRecord] = Field(default_factory=list, description="成员列表")
    events: list[RoomEventRecord] = Field(default_factory=list, description="时间线事件")
    artifacts: list[RoomArtifactRecord] = Field(default_factory=list, description="产物列表")
    tasks: list[RoomTaskRecord] = Field(default_factory=list, description="任务列表")
    protocol_runs: list[ProtocolRunListItem] = Field(default_factory=list, description="协议运行列表")
    protocol_detail: Optional[ProtocolRunDetail] = Field(default=None, description="协议运行详情")
    viewer_member_id: Optional[str] = Field(default=None, description="成员视角")
