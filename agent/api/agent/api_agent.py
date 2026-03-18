# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_agent.py
# @Date   ：2026/3/13 14:36
# @Author ：leemysw
# 2026/3/13 14:36   Create
# =====================================================

"""Agent API。"""

from typing import List, Optional

from fastapi import APIRouter, HTTPException

from agent.schema.model_agent import AAgent, CreateAgentRequest, CreateWorkspaceEntryRequest, \
    RenameWorkspaceEntryRequest, UpdateAgentRequest, \
    UpdateWorkspaceFileRequest, WorkspaceEntryMutationResponse, \
    WorkspaceEntryRenameResponse, WorkspaceFileContentResponse, \
    WorkspaceFileEntry
from agent.schema.model_cost import AgentCostSummary
from agent.service.agent.agent_service import agent_service
from agent.service.workspace.workspace_service import workspace_service
from agent.infra.server.common import resp

router = APIRouter(tags=["agent"])


@router.get("/agents", response_model=List[AAgent])
async def get_agents():
    """获取所有 Agent 列表。"""
    agents = await agent_service.get_agents()
    return resp.ok(resp.Resp(data=[agent.model_dump() for agent in agents]))


@router.post("/agents")
async def create_agent(request: CreateAgentRequest):
    """创建新 Agent。"""
    try:
        agent = await agent_service.create_agent(
            name=request.name,
            workspace_path=request.workspace_path,
            options=request.options,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    return resp.ok(resp.Resp(data=agent.model_dump()))


@router.get("/agents/{agent_id}")
async def get_agent(agent_id: str):
    """获取 Agent 配置。"""
    try:
        agent = await agent_service.get_agent(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=agent.model_dump()))


@router.patch("/agents/{agent_id}")
async def update_agent(agent_id: str, request: UpdateAgentRequest):
    """更新 Agent 配置。"""
    try:
        agent = await agent_service.update_agent(
            agent_id=agent_id,
            name=request.name,
            options=request.options,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=agent.model_dump()))


@router.delete("/agents/{agent_id}")
async def delete_agent(agent_id: str):
    """删除 Agent。"""
    try:
        await agent_service.delete_agent(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data={"success": True}))


@router.get("/agents/validate/name")
async def validate_agent_name(name: str, exclude_agent_id: Optional[str] = None):
    """校验 Agent 名称是否合法、是否重复。"""
    result = await agent_service.validate_agent_name(name, exclude_agent_id=exclude_agent_id)
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.get("/agents/{agent_id}/sessions")
async def get_agent_sessions(agent_id: str):
    """获取 Agent 下的所有会话。"""
    try:
        sessions = await agent_service.get_agent_sessions(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[session.model_dump() for session in sessions]))


@router.get("/agents/{agent_id}/cost/summary", response_model=AgentCostSummary)
async def get_agent_cost_summary(agent_id: str):
    """获取 Agent 成本汇总。"""
    try:
        summary = await agent_service.get_agent_cost_summary(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=summary.model_dump(mode="json")))


@router.get("/agents/{agent_id}/workspace/files")
async def get_workspace_files(agent_id: str):
    """获取 Agent workspace 文件列表。"""
    try:
        files = await workspace_service.get_workspace_files(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    data = [WorkspaceFileEntry(**item).model_dump() for item in files]
    return resp.ok(resp.Resp(data=data))


@router.get("/agents/{agent_id}/workspace/file", response_model=WorkspaceFileContentResponse)
async def get_workspace_file(agent_id: str, path: str):
    """读取 Agent workspace 文件内容。"""
    try:
        content = await workspace_service.get_workspace_file(agent_id, path)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    data = WorkspaceFileContentResponse(path=path, content=content).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.put("/agents/{agent_id}/workspace/file")
async def update_workspace_file(agent_id: str, request: UpdateWorkspaceFileRequest):
    """更新 Agent workspace 文件内容。"""
    try:
        saved_path = await workspace_service.update_workspace_file(
            agent_id=agent_id,
            path=request.path,
            content=request.content,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    data = WorkspaceFileContentResponse(path=saved_path, content=request.content).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.post("/agents/{agent_id}/workspace/entry", response_model=WorkspaceEntryMutationResponse)
async def create_workspace_entry(agent_id: str, request: CreateWorkspaceEntryRequest):
    """创建 Agent workspace 条目。"""
    try:
        created_path = await workspace_service.create_workspace_entry(
            agent_id=agent_id,
            path=request.path,
            entry_type=request.entry_type,
            content=request.content,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileExistsError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=WorkspaceEntryMutationResponse(path=created_path).model_dump()))


@router.patch("/agents/{agent_id}/workspace/entry", response_model=WorkspaceEntryRenameResponse)
async def rename_workspace_entry(agent_id: str, request: RenameWorkspaceEntryRequest):
    """重命名 Agent workspace 条目。"""
    try:
        old_path, new_path = await workspace_service.rename_workspace_entry(
            agent_id=agent_id,
            path=request.path,
            new_path=request.new_path,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileExistsError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    data = WorkspaceEntryRenameResponse(path=old_path, new_path=new_path).model_dump()
    return resp.ok(resp.Resp(data=data))


@router.delete("/agents/{agent_id}/workspace/entry", response_model=WorkspaceEntryMutationResponse)
async def delete_workspace_entry(agent_id: str, path: str):
    """删除 Agent workspace 条目。"""
    try:
        deleted_path = await workspace_service.delete_workspace_entry(agent_id, path)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=WorkspaceEntryMutationResponse(path=deleted_path).model_dump()))
