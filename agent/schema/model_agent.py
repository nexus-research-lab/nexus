# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_agent.py
# @Date   ：2026/3/4 15:09
# @Author ：leemysw
# 2026/3/4 15:09   Create
# =====================================================

"""
Agent Pydantic 模型

[INPUT]: 依赖 pydantic
[OUTPUT]: 对外提供 AAgent / AgentOptions / CreateAgentRequest / UpdateAgentRequest
[POS]: schema 模块的 Agent 模型定义，被 agent_manager/agent_repository/api_agent 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from typing import Literal
from typing import Optional

from pydantic import BaseModel, Field

# =====================================================
# Agent 配置 — 映射到 ClaudeAgentOptions
# =====================================================

class AgentOptions(BaseModel):
    """Agent 级别配置，对应 ClaudeAgentOptions 的 Agent 层字段"""
    model: Optional[str] = Field(default=None, description="模型选择")
    permission_mode: Optional[str] = Field(default=None, description="权限模式")
    allowed_tools: Optional[list[str]] = Field(default=None, description="工具白名单")
    disallowed_tools: Optional[list[str]] = Field(default=None, description="工具黑名单")
    max_turns: Optional[int] = Field(default=None, description="最大轮次")
    max_thinking_tokens: Optional[int] = Field(default=None, description="思考 token 上限")
    mcp_servers: Optional[dict] = Field(default=None, description="MCP 服务器配置")
    setting_sources: Optional[list[Literal["user", "project", "local"]]] = Field(
        default=None,
        description="Claude 设置加载源",
    )

    model_config = {"from_attributes": True}


# =====================================================
# Agent 模型
# =====================================================

class AAgent(BaseModel):
    """Agent 模型 — 一个 Agent = 一个工作区"""
    agent_id: str = Field(..., description="Agent 唯一标识")
    name: str = Field(..., description="显示名称")
    workspace_path: str = Field(default="", description="工作区路径（系统托管: ~/.nexus/workspace/<agent_name_slug>）")
    options: AgentOptions = Field(default_factory=AgentOptions, description="Agent 配置")
    created_at: datetime = Field(default_factory=datetime.now, description="创建时间")
    status: str = Field(default="active", description="状态: active/archived")

    # 身份标识字段（Identity Tab）
    avatar: Optional[str] = Field(default=None, description="头像标识（emoji 或图标名称）")
    description: Optional[str] = Field(default=None, description="Agent 描述文本")
    vibe_tags: Optional[list[str]] = Field(default=None, description="氛围标签列表")
    skills_count: Optional[int] = Field(default=None, description="已安装的 Skill 数量")

    model_config = {"from_attributes": True}


# =====================================================
# 请求模型
# =====================================================

class CreateAgentRequest(BaseModel):
    """创建 Agent 请求"""
    name: str = Field(..., description="Agent 名称")
    options: Optional[AgentOptions] = Field(default=None, description="初始配置")
    # 身份标识字段
    avatar: Optional[str] = Field(default=None, description="头像标识（emoji 或图标名称）")
    description: Optional[str] = Field(default=None, description="Agent 描述文本")
    vibe_tags: Optional[list[str]] = Field(default=None, description="氛围标签列表")


class UpdateAgentRequest(BaseModel):
    """更新 Agent 请求"""
    name: Optional[str] = Field(default=None, description="名称")
    options: Optional[AgentOptions] = Field(default=None, description="配置")
    # 身份标识字段
    avatar: Optional[str] = Field(default=None, description="头像标识（emoji 或图标名称）")
    description: Optional[str] = Field(default=None, description="Agent 描述文本")
    vibe_tags: Optional[list[str]] = Field(default=None, description="氛围标签列表")


class ValidateAgentNameResponse(BaseModel):
    """Agent 名称校验结果"""
    name: str = Field(..., description="原始输入名称")
    normalized_name: str = Field(..., description="标准化后的名称")
    is_valid: bool = Field(..., description="是否符合命名规则")
    is_available: bool = Field(..., description="名称是否可用（未重复）")
    workspace_path: Optional[str] = Field(default=None, description="预期工作区路径")
    reason: Optional[str] = Field(default=None, description="不可用原因")

    @classmethod
    def invalid(
        cls,
        name: str,
        normalized_name: str,
        reason: str,
    ) -> "ValidateAgentNameResponse":
        """构造名称不合法的校验结果。"""
        return cls(
            name=name,
            normalized_name=normalized_name,
            is_valid=False,
            is_available=False,
            reason=reason,
        )

    @classmethod
    def unavailable(
        cls,
        name: str,
        normalized_name: str,
        workspace_path: str,
        reason: str,
    ) -> "ValidateAgentNameResponse":
        """构造名称不可用的校验结果。"""
        return cls(
            name=name,
            normalized_name=normalized_name,
            is_valid=True,
            is_available=False,
            workspace_path=workspace_path,
            reason=reason,
        )

    @classmethod
    def available(
        cls,
        name: str,
        normalized_name: str,
        workspace_path: str,
    ) -> "ValidateAgentNameResponse":
        """构造名称可用的校验结果。"""
        return cls(
            name=name,
            normalized_name=normalized_name,
            is_valid=True,
            is_available=True,
            workspace_path=workspace_path,
        )


class WorkspaceFileEntry(BaseModel):
    """Workspace 文件条目。"""
    path: str = Field(..., description="相对 workspace 的文件路径")
    name: str = Field(..., description="文件名")
    is_dir: bool = Field(..., description="是否目录")
    size: Optional[int] = Field(default=None, description="文件大小")
    modified_at: str = Field(..., description="最后修改时间")
    depth: int = Field(..., description="目录深度")


class WorkspaceFileContentResponse(BaseModel):
    """Workspace 文件内容响应。"""
    path: str = Field(..., description="相对路径")
    content: str = Field(..., description="文本内容")


class UpdateWorkspaceFileRequest(BaseModel):
    """更新 Workspace 文件请求。"""
    path: str = Field(..., description="相对 workspace 的文件路径")
    content: str = Field(default="", description="文件内容")


class CreateWorkspaceEntryRequest(BaseModel):
    """创建 Workspace 条目请求。"""
    path: str = Field(..., description="相对 workspace 的目标路径")
    entry_type: Literal["file", "directory"] = Field(..., description="创建类型")
    content: str = Field(default="", description="创建文件时的初始内容")


class RenameWorkspaceEntryRequest(BaseModel):
    """重命名 Workspace 条目请求。"""
    path: str = Field(..., description="当前相对路径")
    new_path: str = Field(..., description="新的相对路径")


class WorkspaceEntryMutationResponse(BaseModel):
    """Workspace 条目变更响应。"""
    path: str = Field(..., description="变更后的路径")


class WorkspaceEntryRenameResponse(BaseModel):
    """Workspace 条目重命名响应。"""
    path: str = Field(..., description="旧路径")
    new_path: str = Field(..., description="新路径")


class UploadWorkspaceFileResponse(BaseModel):
    """上传 Workspace 文件响应。"""
    path: str = Field(..., description="保存路径")
    name: str = Field(..., description="文件名")
    size: int = Field(..., description="文件大小")
