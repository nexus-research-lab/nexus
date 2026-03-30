# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_skill.py
# @Date   ：2026/3/30 20:46
# @Author ：Codex
# 2026/3/30 20:46   Create
# =====================================================

"""Skill Marketplace API。"""

from __future__ import annotations

from typing import List, Optional

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from agent.infra.server.common import resp
from agent.schema.model_skill import (
    AgentSkillEntry,
    BatchInstallSkillsRequest,
    BatchInstallSkillsResponse,
    ImportSkillsShSkillRequest,
    ImportGitSkillRequest,
    InstallSkillRequest,
    SearchExternalSkillsResponse,
    SkillDetail,
    SkillInfo,
    UpdateSkillGlobalEnabledRequest,
    UpdateInstalledSkillsResponse,
)
from agent.service.capability.skills.skill_service import skill_service

router = APIRouter(tags=["skill"])


@router.get("/skills", response_model=List[SkillInfo])
async def get_available_skills(
    agent_id: Optional[str] = None,
    category_key: Optional[str] = None,
    source_type: Optional[str] = None,
    q: Optional[str] = None,
):
    """获取 Skill Marketplace 列表。"""
    try:
        skills = await skill_service.get_all_skills(
            agent_id=agent_id,
            category_key=category_key,
            source_type=source_type,
            q=q,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump() for item in skills]))


@router.get("/skills/{skill_name}", response_model=SkillDetail)
async def get_skill_detail(skill_name: str, agent_id: Optional[str] = None):
    """获取单个 Skill 详情。"""
    try:
        detail = await skill_service.get_skill_detail(skill_name, agent_id=agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump()))


@router.post("/skills/import/local", response_model=SkillDetail)
async def import_local_skill(
    file: UploadFile | None = File(default=None),
    local_path: str | None = Form(default=None),
):
    """导入本地 Skill。"""
    try:
        if file is not None:
            detail = await skill_service.import_uploaded_file(file.filename or "skill.zip", await file.read())
        elif local_path:
            detail = await skill_service.import_local_path(local_path)
        else:
            raise ValueError("请提供本地 zip 上传文件或 local_path")
    except (FileNotFoundError, ValueError) as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump()))


@router.post("/skills/import/git", response_model=SkillDetail)
async def import_git_skill(request: ImportGitSkillRequest):
    """通过 Git 链接导入 Skill。"""
    try:
        detail = await skill_service.import_git(request.url, request.branch)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump()))


@router.post("/skills/{skill_name}/install", response_model=SkillDetail)
async def install_skill_to_pool(skill_name: str):
    """将技能安装到全局资源池。"""
    try:
        detail = await skill_service.install_to_pool(skill_name)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump()))


@router.get("/skills/search/external", response_model=SearchExternalSkillsResponse)
async def search_external_skills(q: str):
    """搜索 skills.sh 外部技能。"""
    try:
        results = skill_service.search_external_skills(q)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    payload = SearchExternalSkillsResponse(query=q, results=results)
    return resp.ok(resp.Resp(data=payload.model_dump()))


@router.post("/skills/import/skills-sh", response_model=SkillDetail)
async def import_skills_sh_skill(request: ImportSkillsShSkillRequest):
    """从 skills.sh 搜索结果导入指定 skill。"""
    try:
        detail = await skill_service.import_skills_sh(request.package_spec, request.skill_slug)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump()))


@router.post("/skills/update-imported", response_model=UpdateInstalledSkillsResponse)
async def update_imported_skills():
    """更新全局已导入的外部 Skill。"""
    result = await skill_service.update_global_skills()
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.post("/skills/{skill_name}/update", response_model=SkillDetail)
async def update_global_skill(skill_name: str):
    """更新单个全局 Skill。"""
    try:
        result = await skill_service.update_global_skill(skill_name)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.patch("/skills/{skill_name}/global-enabled", response_model=SkillDetail)
async def set_skill_global_enabled(skill_name: str, request: UpdateSkillGlobalEnabledRequest):
    """设置 Skill 的全局启用状态。"""
    try:
        result = await skill_service.set_global_enabled(skill_name, request.enabled)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.delete("/skills/{skill_name}")
async def delete_skill_from_pool(skill_name: str):
    """从全局技能池删除 skill。"""
    try:
        await skill_service.delete_from_pool(skill_name)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data={"success": True}))


@router.get("/agents/{agent_id}/skills", response_model=List[AgentSkillEntry])
async def get_agent_skills(agent_id: str):
    """获取 Agent 的 Skill 列表（含安装状态）。"""
    try:
        skills = await skill_service.get_agent_skills(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump() for item in skills]))


@router.post("/agents/{agent_id}/skills")
async def install_agent_skill(agent_id: str, request: InstallSkillRequest):
    """为 Agent 安装单个 Skill。"""
    try:
        entry = await skill_service.install_skill(agent_id, request.skill_name)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=entry.model_dump()))


@router.delete("/agents/{agent_id}/skills/{skill_name}")
async def uninstall_agent_skill(agent_id: str, skill_name: str):
    """从 Agent 卸载 Skill。"""
    try:
        await skill_service.uninstall_skill(agent_id, skill_name)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data={"success": True}))


@router.post("/agents/{agent_id}/skills/batch-install", response_model=BatchInstallSkillsResponse)
async def batch_install_agent_skills(agent_id: str, request: BatchInstallSkillsRequest):
    """为 Agent 批量安装 Skill。"""
    try:
        result = await skill_service.batch_install_skills(agent_id, request.skill_names)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.post("/agents/{agent_id}/skills/update-installed", response_model=UpdateInstalledSkillsResponse)
async def update_installed_agent_skills(agent_id: str):
    """更新当前 Agent 已安装的外部 Skill。"""
    try:
        result = await skill_service.update_installed_skills(agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=result.model_dump()))


@router.post("/agents/{agent_id}/skills/{skill_name}/update", response_model=AgentSkillEntry)
async def update_single_agent_skill(agent_id: str, skill_name: str):
    """更新单个已安装 Skill。"""
    try:
        result = await skill_service.update_skill(agent_id, skill_name)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=result.model_dump()))
