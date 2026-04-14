# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_agent_workspace.py
# @Date   ：2026/3/30
# @Author ：leemysw
# 2026/3/30   从 api_agent.py 拆分
# =====================================================

"""Agent Workspace 文件操作 API。"""

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from agent.schema.model_agent import (
    CreateWorkspaceEntryRequest,
    RenameWorkspaceEntryRequest,
    UpdateWorkspaceFileRequest,
    UploadWorkspaceFileResponse,
    WorkspaceEntryMutationResponse,
    WorkspaceEntryRenameResponse,
    WorkspaceFileContentResponse,
    WorkspaceFileEntry,
)
from agent.service.workspace.workspace_service import workspace_service
from agent.infra.server.common import resp
from fastapi.responses import FileResponse

router = APIRouter(tags=["agent-workspace"])


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


@router.post("/agents/{agent_id}/workspace/upload", response_model=UploadWorkspaceFileResponse)
async def upload_workspace_file(
    agent_id: str,
    file: UploadFile = File(..., description="上传的文件"),
    path: str = Form(default=None, description="目标路径（可选，默认为文件名）"),
):
    """上传文件到 Agent workspace。"""
    try:
        result = await workspace_service.upload_file(agent_id, file, path)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.get("/agents/{agent_id}/workspace/download")
async def download_workspace_file(agent_id: str, path: str):
    """下载 Agent workspace 文件（支持二进制文件）。"""
    try:
        file_path, file_name = await workspace_service.get_file_for_download(agent_id, path)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    return FileResponse(
        path=str(file_path),
        filename=file_name,
        media_type="application/octet-stream",
    )
