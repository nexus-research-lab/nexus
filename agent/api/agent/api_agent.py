# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_agent.py
# @Date   ：2026/3/4 15:09
# @Author ：leemysw
# 2026/3/4 15:09   Create
# =====================================================

"""
Agent API

[INPUT]: 依赖 agent_manager，依赖 session_store
[OUTPUT]: 对外提供 /agents CRUD 端点 + Agent 下属 Session 查询
[POS]: api 层的 Agent 管理端点，被前端消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import List, Optional

from fastapi import APIRouter, HTTPException

from agent.service.agent_manager import agent_manager
from agent.service.schema.model_agent import (
    AAgent,
    CreateWorkspaceEntryRequest,
    CreateAgentRequest,
    RenameWorkspaceEntryRequest,
    UpdateWorkspaceFileRequest,
    UpdateAgentRequest,
    WorkspaceFileContentResponse,
    WorkspaceFileEntry,
    WorkspaceEntryMutationResponse,
    WorkspaceEntryRenameResponse,
)
from agent.service.schema.model_cost import AgentCostSummary
from agent.infra.runtime.session_manager import session_manager
from agent.service.session_store import session_store
from agent.shared.server.common import resp

router = APIRouter(tags=["agent"])


# =====================================================
# Agent CRUD
# =====================================================

@router.get("/agents", response_model=List[AAgent])
async def get_agents():
    """获取所有 Agent 列表"""
    agents = await agent_manager.get_all_agents()
    data = [a.model_dump() for a in agents]
    return resp.ok(resp.Resp(data=data))


@router.post("/agents")
async def create_agent(request: CreateAgentRequest):
    """创建新 Agent"""
    try:
        agent = await agent_manager.create_agent(
            name=request.name,
            workspace_path=request.workspace_path,
            options=request.options,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    if not agent:
        raise HTTPException(status_code=500, detail="Failed to create agent")
    return resp.ok(resp.Resp(data=agent.model_dump()))


@router.get("/agents/{agent_id}")
async def get_agent(agent_id: str):
    """获取 Agent 配置"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")
    return resp.ok(resp.Resp(data=agent.model_dump()))


@router.patch("/agents/{agent_id}")
async def update_agent(agent_id: str, request: UpdateAgentRequest):
    """更新 Agent 配置"""
    existing = await agent_manager.get_agent(agent_id)
    if not existing:
        raise HTTPException(status_code=404, detail="Agent not found")

    try:
        success = await agent_manager.update_agent(
            agent_id=agent_id,
            name=request.name,
            options=request.options,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    if not success:
        raise HTTPException(status_code=500, detail="Failed to update agent")

    await session_manager.refresh_agent_sessions(agent_id)
    agent = await agent_manager.get_agent(agent_id)
    return resp.ok(resp.Resp(data=agent.model_dump()))


@router.delete("/agents/{agent_id}")
async def delete_agent(agent_id: str):
    """删除 Agent（软删除）"""
    success = await agent_manager.delete_agent(agent_id)
    if not success:
        raise HTTPException(status_code=404, detail="Agent not found")
    return resp.ok(resp.Resp(data={"success": True}))


@router.get("/agents/validate/name")
async def validate_agent_name(name: str, exclude_agent_id: Optional[str] = None):
    """校验 Agent 名称是否合法、是否重复。"""
    result = await agent_manager.validate_agent_name(name, exclude_agent_id=exclude_agent_id)
    return resp.ok(resp.Resp(data=result))


# =====================================================
# Agent 下属 Session
# =====================================================

@router.get("/agents/{agent_id}/sessions")
async def get_agent_sessions(agent_id: str):
    """获取 Agent 下的所有会话"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    all_sessions = await session_store.get_all_sessions()
    # 过滤出属于此 Agent 的 session
    agent_sessions = [s for s in all_sessions if s.agent_id == agent_id]
    data = [s.model_dump() for s in agent_sessions]
    return resp.ok(resp.Resp(data=data))


@router.get("/agents/{agent_id}/cost/summary", response_model=AgentCostSummary)
async def get_agent_cost_summary(agent_id: str):
    """获取 Agent 成本汇总。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    summary = await session_store.get_agent_cost_summary(agent_id)
    return resp.ok(resp.Resp(data=summary.model_dump(mode="json")))


@router.get("/agents/{agent_id}/workspace/files")
async def get_workspace_files(agent_id: str):
    """获取 Agent workspace 文件列表。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    workspace = await agent_manager.get_agent_workspace(agent_id)
    files = [WorkspaceFileEntry(**item).model_dump() for item in workspace.list_files()]
    return resp.ok(resp.Resp(data=files))


@router.get("/agents/{agent_id}/workspace/file", response_model=WorkspaceFileContentResponse)
async def get_workspace_file(agent_id: str, path: str):
    """读取 Agent workspace 文件内容。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    workspace = await agent_manager.get_agent_workspace(agent_id)

    try:
        content = workspace.read_relative_file(path)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    data = WorkspaceFileContentResponse(path=path, content=content).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.put("/agents/{agent_id}/workspace/file")
async def update_workspace_file(agent_id: str, request: UpdateWorkspaceFileRequest):
    """更新 Agent workspace 文件内容。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    workspace = await agent_manager.get_agent_workspace(agent_id)

    try:
        saved_path = workspace.write_relative_file(request.path, request.content, source="api")
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    await session_manager.refresh_agent_sessions(agent_id)
    data = WorkspaceFileContentResponse(path=saved_path, content=request.content).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.post("/agents/{agent_id}/workspace/entry", response_model=WorkspaceEntryMutationResponse)
async def create_workspace_entry(agent_id: str, request: CreateWorkspaceEntryRequest):
    """创建 Agent workspace 条目。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    workspace = await agent_manager.get_agent_workspace(agent_id)

    try:
        created_path = workspace.create_entry(
            relative_path=request.path,
            entry_type=request.entry_type,
            content=request.content,
        )
    except FileExistsError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    await session_manager.refresh_agent_sessions(agent_id)
    data = WorkspaceEntryMutationResponse(path=created_path).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.patch("/agents/{agent_id}/workspace/entry", response_model=WorkspaceEntryRenameResponse)
async def rename_workspace_entry(agent_id: str, request: RenameWorkspaceEntryRequest):
    """重命名 Agent workspace 条目。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    workspace = await agent_manager.get_agent_workspace(agent_id)

    try:
        old_path, new_path = workspace.rename_entry(
            relative_path=request.path,
            new_relative_path=request.new_path,
        )
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileExistsError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    await session_manager.refresh_agent_sessions(agent_id)
    data = WorkspaceEntryRenameResponse(path=old_path, new_path=new_path).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.delete("/agents/{agent_id}/workspace/entry", response_model=WorkspaceEntryMutationResponse)
async def delete_workspace_entry(agent_id: str, path: str):
    """删除 Agent workspace 条目。"""
    agent = await agent_manager.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    workspace = await agent_manager.get_agent_workspace(agent_id)

    try:
        deleted_path = workspace.delete_entry(path)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    await session_manager.refresh_agent_sessions(agent_id)
    data = WorkspaceEntryMutationResponse(path=deleted_path).model_dump()
    return resp.ok(resp.Resp(data=data))
