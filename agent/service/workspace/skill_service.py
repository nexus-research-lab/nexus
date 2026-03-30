# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_service.py
# @Date   ：2026/3/30 20:40
# @Author ：Codex
# 2026/3/30 20:40   Create
# =====================================================

"""Skill Marketplace 服务。"""

from __future__ import annotations

from pathlib import Path

from agent.schema.model_skill import (
    AgentSkillEntry,
    BatchInstallSkillsResponse,
    ExternalSkillSearchItem,
    SkillActionFailure,
    SkillDetail,
    SkillInfo,
    UpdateInstalledSkillsResponse,
)
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.workspace.skill_catalog import SkillCatalog
from agent.service.workspace.skill_import_service import SkillImportService
from agent.service.workspace.skill_registry_store import SkillRegistryStore
from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer
from agent.storage.agent_repository import agent_repository
from agent.utils.logger import logger


class SkillService:
    """负责 Skill Marketplace 查询、导入、安装与更新。"""

    BASE_SKILL_NAMES = ("memory-manager",)
    MAIN_AGENT_SKILL_NAMES = ("nexus-manager",)

    def __init__(self) -> None:
        self._catalog = SkillCatalog()
        self._import_service = SkillImportService()
        self._store = SkillRegistryStore()

    async def get_all_skills(
        self,
        agent_id: str | None = None,
        category_key: str | None = None,
        source_type: str | None = None,
        q: str | None = None,
    ) -> list[SkillInfo]:
        records = self._catalog.list_records()
        installed_names = set()
        is_main = True
        if agent_id:
            agent = await self._resolve_agent(agent_id)
            installed_names = set(getattr(agent.options, "installed_skills", None) or [])
            is_main = MainAgentProfile.is_main_agent(agent_id)

        query = (q or "").strip().lower()
        items: list[SkillInfo] = []
        for record in records.values():
            detail = record.detail.model_copy(deep=True)
            if detail.scope == "main" and not is_main:
                continue
            detail.installed = self._is_installed(
                detail.name,
                installed_names,
                detail.source_type,
                resource_pool_mode=agent_id is None,
            )
            detail.locked = detail.source_type == "system"
            detail.has_update = self._has_update(detail, record, detail.installed)
            if category_key and detail.category_key != category_key:
                continue
            if source_type and detail.source_type != source_type:
                continue
            if query and not self._match_query(detail, query):
                continue
            items.append(SkillInfo.model_validate(detail.model_dump()))
        return sorted(items, key=lambda item: (item.category_name, item.title.lower()))

    async def get_skill_detail(self, skill_name: str, agent_id: str | None = None) -> SkillDetail:
        record = self._require_record(skill_name)
        detail = record.detail.model_copy(deep=True)
        installed_names = set()
        if agent_id:
            agent = await self._resolve_agent(agent_id)
            installed_names = set(getattr(agent.options, "installed_skills", None) or [])
        detail.installed = self._is_installed(
            detail.name,
            installed_names,
            detail.source_type,
            resource_pool_mode=agent_id is None,
        )
        detail.locked = detail.source_type == "system"
        detail.has_update = self._has_update(detail, record, detail.installed)
        return detail

    async def get_agent_skills(self, agent_id: str) -> list[AgentSkillEntry]:
        items = await self.get_all_skills(agent_id=agent_id)
        return [
            AgentSkillEntry.model_validate(item.model_dump())
            for item in items
            if item.global_enabled or item.installed or item.locked
        ]

    async def install_skill(self, agent_id: str, skill_name: str) -> AgentSkillEntry:
        record = self._validate_installable(agent_id, skill_name)
        agent = await self._resolve_agent(agent_id)
        deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
        deployer.deploy_skill(skill_name, source_dir=record.source_path)
        current_skills = list(getattr(agent.options, "installed_skills", None) or [])
        if skill_name not in current_skills:
            current_skills.append(skill_name)
        await agent_repository.update_agent(agent_id, options={"installed_skills": current_skills})
        logger.info(f"✅ Skill installed: {skill_name} → agent {agent_id}")
        return AgentSkillEntry.model_validate(
            (await self.get_skill_detail(skill_name, agent_id=agent_id)).model_dump()
        )

    async def uninstall_skill(self, agent_id: str, skill_name: str) -> None:
        record = self._require_record(skill_name)
        if record.detail.source_type == "system":
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be uninstalled")
        agent = await self._resolve_agent(agent_id)
        deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
        deployer.undeploy_skill(skill_name)
        current_skills = list(getattr(agent.options, "installed_skills", None) or [])
        if skill_name in current_skills:
            current_skills.remove(skill_name)
        await agent_repository.update_agent(agent_id, options={"installed_skills": current_skills})

    async def batch_install_skills(self, agent_id: str, skill_names: list[str]) -> BatchInstallSkillsResponse:
        successes: list[str] = []
        failures: list[SkillActionFailure] = []
        for skill_name in skill_names:
            try:
                await self.install_skill(agent_id, skill_name)
                successes.append(skill_name)
            except (LookupError, ValueError, FileNotFoundError) as exc:
                failures.append(SkillActionFailure(skill_name=skill_name, error=str(exc)))
        return BatchInstallSkillsResponse(successes=successes, failures=failures)

    async def update_installed_skills(self, agent_id: str) -> UpdateInstalledSkillsResponse:
        entries = await self.get_agent_skills(agent_id)
        updated_skills: list[str] = []
        skipped_skills: list[str] = []
        failures: list[SkillActionFailure] = []
        for entry in entries:
            if not entry.installed or entry.source_type != "external":
                continue
            if not entry.has_update:
                skipped_skills.append(entry.name)
                continue
            try:
                await self.update_skill(agent_id, entry.name)
                updated_skills.append(entry.name)
            except (LookupError, ValueError, FileNotFoundError) as exc:
                failures.append(SkillActionFailure(skill_name=entry.name, error=str(exc)))
        return UpdateInstalledSkillsResponse(
            updated_skills=updated_skills,
            skipped_skills=skipped_skills,
            failures=failures,
        )

    async def update_global_skills(self) -> UpdateInstalledSkillsResponse:
        updated_skills: list[str] = []
        skipped_skills: list[str] = []
        failures: list[SkillActionFailure] = []
        for skill_name in self._catalog.list_records().keys():
            try:
                record = self._require_record(skill_name)
            except LookupError:
                continue
            if record.detail.source_type != "external":
                continue
            if not self._has_update(record.detail, record, False):
                skipped_skills.append(skill_name)
                continue
            try:
                await self.update_global_skill(skill_name)
                updated_skills.append(skill_name)
            except (LookupError, ValueError, FileNotFoundError) as exc:
                failures.append(SkillActionFailure(skill_name=skill_name, error=str(exc)))
        return UpdateInstalledSkillsResponse(
            updated_skills=updated_skills,
            skipped_skills=skipped_skills,
            failures=failures,
        )

    async def update_skill(self, agent_id: str, skill_name: str) -> AgentSkillEntry:
        record = self._require_record(skill_name)
        if record.detail.source_type != "external":
            raise ValueError(f"Skill '{skill_name}' does not support manual update")
        manifest = self._import_service._store.read_manifest(skill_name)
        if manifest.import_mode == "git":
            updated_manifest = self._import_service.update_git_skill(manifest)
        elif manifest.import_mode == "skills_sh":
            updated_manifest = self._import_service.update_skills_sh_skill(manifest)
        else:
            raise ValueError(f"Skill '{skill_name}' does not support remote update")
        updated_record = self._catalog.get_record(updated_manifest.name)
        if not updated_record:
            raise LookupError(f"Skill not found after update: {skill_name}")
        agent = await self._resolve_agent(agent_id)
        if skill_name in list(getattr(agent.options, "installed_skills", None) or []):
            deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
            deployer.deploy_skill(skill_name, source_dir=updated_record.source_path)
        return AgentSkillEntry.model_validate(
            (await self.get_skill_detail(skill_name, agent_id=agent_id)).model_dump()
        )

    async def update_global_skill(self, skill_name: str) -> SkillDetail:
        record = self._require_record(skill_name)
        if record.detail.source_type != "external":
            raise ValueError(f"Skill '{skill_name}' does not support manual update")
        manifest = self._import_service._store.read_manifest(skill_name)
        if manifest.import_mode == "git":
            self._import_service.update_git_skill(manifest)
        elif manifest.import_mode == "skills_sh":
            self._import_service.update_skills_sh_skill(manifest)
        else:
            raise ValueError(f"Skill '{skill_name}' does not support remote update")
        return await self.get_skill_detail(skill_name)

    async def import_local_path(self, local_path: str) -> SkillDetail:
        manifest = self._import_service.import_local_path(local_path)
        return await self.get_skill_detail(manifest.name)

    async def import_uploaded_file(self, file_name: str, payload: bytes) -> SkillDetail:
        manifest = self._import_service.import_uploaded_file(file_name, payload)
        return await self.get_skill_detail(manifest.name)

    async def import_git(self, url: str, branch: str | None = None) -> SkillDetail:
        manifest = self._import_service.import_git(url, branch)
        return await self.get_skill_detail(manifest.name)

    async def import_skills_sh(self, package_spec: str, skill_slug: str) -> SkillDetail:
        manifest = self._import_service.import_skills_sh(package_spec, skill_slug)
        return await self.get_skill_detail(manifest.name)

    def search_external_skills(self, query: str) -> list[ExternalSkillSearchItem]:
        return self._import_service.search_skills_sh(query)

    def _validate_installable(self, agent_id: str, skill_name: str):
        record = self._require_record(skill_name)
        if record.detail.source_type == "system":
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be manually installed")
        if not record.detail.global_enabled:
            raise ValueError(f"Skill '{skill_name}' is globally disabled")
        is_main = MainAgentProfile.is_main_agent(agent_id)
        if record.detail.scope == "main" and not is_main:
            raise ValueError(f"Skill '{skill_name}' is restricted to main agent")
        return record

    async def set_global_enabled(self, skill_name: str, enabled: bool) -> SkillDetail:
        record = self._require_record(skill_name)
        if record.detail.locked:
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be globally disabled")
        self._store.write_global_state(skill_name, enabled)
        return await self.get_skill_detail(skill_name)

    async def delete_from_pool(self, skill_name: str) -> None:
        record = self._require_record(skill_name)
        if not record.detail.deletable:
            raise ValueError(f"Skill '{skill_name}' cannot be deleted from the pool")

        # 中文注释：删除资源池中的外部 skill 时，需要同时把所有 Agent 的启用关系和 workspace 副本清理掉，
        # 否则会出现设置页仍保留脏数据、但实际 skill 文件已不存在的问题。
        agents = await agent_repository.get_all_agents()
        for agent in agents:
            current_skills = list(getattr(agent.options, "installed_skills", None) or [])
            if skill_name not in current_skills:
                continue
            current_skills.remove(skill_name)
            deployer = WorkspaceSkillDeployer(agent.agent_id, Path(agent.workspace_path))
            deployer.undeploy_skill(skill_name)
            await agent_repository.update_agent(
                agent.agent_id,
                options={"installed_skills": current_skills},
            )

        self._store.delete_skill(skill_name)

    async def _resolve_agent(self, agent_id: str):
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise LookupError(f"Agent not found: {agent_id}")
        return agent

    def _require_record(self, skill_name: str):
        record = self._catalog.get_record(skill_name)
        if not record:
            raise LookupError(f"Skill not found: {skill_name}")
        return record

    def _is_installed(
        self,
        skill_name: str,
        installed_names: set[str],
        source_type: str,
        resource_pool_mode: bool = False,
    ) -> bool:
        if resource_pool_mode:
            return True
        if skill_name in self.BASE_SKILL_NAMES:
            return True
        if skill_name in self.MAIN_AGENT_SKILL_NAMES:
            return True
        return source_type != "system" and skill_name in installed_names

    def _has_update(self, detail: SkillDetail, record, installed: bool) -> bool:
        if detail.source_type != "external":
            return False
        manifest = self._import_service._store.read_manifest(detail.name)
        if manifest.import_mode == "git":
            return self._import_service.check_git_update(manifest)
        if manifest.import_mode == "skills_sh":
            return True
        return False

    def _match_query(self, detail: SkillDetail, query: str) -> bool:
        haystacks = [
            detail.name.lower(),
            detail.title.lower(),
            detail.description.lower(),
            detail.category_name.lower(),
            " ".join(detail.tags).lower(),
        ]
        return any(query in item for item in haystacks)


skill_service = SkillService()
