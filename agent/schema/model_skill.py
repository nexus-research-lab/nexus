# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_skill.py
# @Date   ：2026/3/30 20:30
# @Author ：Codex
# 2026/3/30 20:30   Create
# =====================================================

"""Skill Marketplace 模型。"""

from __future__ import annotations

from typing import Literal, Optional

from pydantic import Field

from agent.infra.schemas.model_cython import AModel

SkillScope = Literal["main", "any"]
SkillSourceType = Literal["system", "builtin", "external"]
SkillImportMode = Literal["upload", "local_path", "git", "skills_sh", "well_known"]


class SkillInfo(AModel):
    """Skill Marketplace 列表项。"""

    name: str = Field(..., description="skill 唯一标识")
    title: str = Field(default="", description="展示标题")
    description: str = Field(default="", description="skill 功能描述")
    scope: SkillScope = Field(default="any", description="skill 适用范围")
    tags: list[str] = Field(default_factory=list, description="标签")
    category_key: str = Field(default="custom-imports", description="分类 key")
    category_name: str = Field(default="自定义导入", description="分类名称")
    source_type: SkillSourceType = Field(default="builtin", description="来源类型")
    source_ref: str = Field(default="", description="来源标识")
    version: str = Field(default="", description="版本号")
    installed: bool = Field(default=False, description="当前 Agent 是否已安装")
    locked: bool = Field(default=False, description="是否系统锁定")
    has_update: bool = Field(default=False, description="是否存在可更新版本")
    deletable: bool = Field(default=False, description="是否允许删除 skill 源")


class SkillDetail(SkillInfo):
    """Skill 详情。"""

    readme_markdown: str = Field(default="", description="完整 SKILL.md 内容")
    recommendation: str = Field(default="", description="推荐理由")


class AgentSkillEntry(SkillInfo):
    """Agent 维度的 Skill 安装信息。"""


class InstallSkillRequest(AModel):
    """安装 Skill 请求。"""

    skill_name: str = Field(..., description="要安装的 skill 名称")


class BatchInstallSkillsRequest(AModel):
    """批量安装 Skill 请求。"""

    skill_names: list[str] = Field(default_factory=list, description="待安装 skill 名称列表")


class SkillActionFailure(AModel):
    """Skill 操作失败项。"""

    skill_name: str = Field(..., description="skill 名称")
    error: str = Field(..., description="失败原因")


class BatchInstallSkillsResponse(AModel):
    """批量安装返回。"""

    successes: list[str] = Field(default_factory=list, description="安装成功的 skill")
    failures: list[SkillActionFailure] = Field(default_factory=list, description="安装失败的 skill")


class UpdateInstalledSkillsResponse(AModel):
    """批量更新返回。"""

    updated_skills: list[str] = Field(default_factory=list, description="更新成功的 skill")
    skipped_skills: list[str] = Field(default_factory=list, description="被跳过的 skill")
    failures: list[SkillActionFailure] = Field(default_factory=list, description="更新失败的 skill")


class ImportGitSkillRequest(AModel):
    """Git 导入请求。"""

    url: str = Field(..., description="Git 仓库链接")
    branch: Optional[str] = Field(default=None, description="分支")


class ImportLocalSkillRequest(AModel):
    """本地路径导入请求。"""

    local_path: str = Field(..., description="本地 skill 目录或 zip 路径")


class SearchExternalSkillsResponse(AModel):
    """外部 Skill 搜索结果。"""

    query: str = Field(default="", description="搜索词")
    results: list["ExternalSkillSearchItem"] = Field(default_factory=list, description="匹配结果")


class ExternalSkillSearchItem(AModel):
    """外部技能搜索项。"""

    name: str = Field(..., description="skill 名称")
    title: str = Field(default="", description="展示标题")
    description: str = Field(default="", description="简介")
    source: str = Field(default="", description="来源仓库")
    package_spec: str = Field(default="", description="skills CLI 安装包标识")
    skill_slug: str = Field(default="", description="skill slug")
    installs: int = Field(default=0, description="安装量")
    detail_url: str = Field(default="", description="skills.sh 详情页地址")
    readme_markdown: str = Field(default="", description="远端 SKILL.md 文本")


class ImportSkillsShSkillRequest(AModel):
    """从 skills.sh 导入 skill 的请求。"""

    package_spec: str = Field(..., description="如 owner/repo")
    skill_slug: str = Field(..., description="目标 skill slug")


class ExternalSkillManifest(AModel):
    """外部 Skill 持久化清单。"""

    name: str = Field(..., description="skill 名称")
    title: str = Field(default="", description="展示标题")
    description: str = Field(default="", description="描述")
    scope: SkillScope = Field(default="any", description="作用域")
    tags: list[str] = Field(default_factory=list, description="标签")
    category_key: str = Field(default="custom-imports", description="分类 key")
    category_name: str = Field(default="自定义导入", description="分类名称")
    version: str = Field(default="", description="版本")
    source_type: SkillSourceType = Field(default="external", description="来源类型")
    source_ref: str = Field(default="", description="来源引用")
    import_mode: SkillImportMode = Field(default="upload", description="导入模式")
    recommendation: str = Field(default="", description="推荐理由")
    git_url: Optional[str] = Field(default=None, description="Git 仓库地址")
    git_branch: Optional[str] = Field(default=None, description="Git 分支")
    git_commit: Optional[str] = Field(default=None, description="当前 Git commit")
    skill_subdir: str = Field(default=".", description="仓库中的 skill 相对路径")
    package_spec: Optional[str] = Field(default=None, description="skills.sh 包标识")
    skill_slug: Optional[str] = Field(default=None, description="skills.sh slug")


SearchExternalSkillsResponse.model_rebuild()
