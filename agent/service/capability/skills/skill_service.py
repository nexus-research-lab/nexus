# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_service.py
# @Date   ：2026/4/2 11:40
# @Author ：leemysw
# 2026/4/2 11:40   Create
# =====================================================

"""Skill Marketplace 服务。"""

from __future__ import annotations

import time

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
from agent.service.capability.skills.skill_catalog import SkillCatalog, SkillCatalogRecord
from agent.service.capability.skills.skill_import_service import SkillImportService
from agent.service.capability.skills.skill_registry_store import SkillRegistryStore
from agent.service.capability.skills.skill_workspace_store import SkillWorkspaceStore
from agent.utils.logger import logger


class SkillService:
    """负责 Skill Marketplace 查询、导入、安装与更新。"""

    UPDATE_STATUS_TTL_SECONDS = 300

    def __init__(self) -> None:
        self._catalog = SkillCatalog()
        self._import_service = SkillImportService()
        self._file_store = SkillRegistryStore()
        self._workspace_store = SkillWorkspaceStore()
        self._update_status_cache: dict[str, tuple[float, bool]] = {}

    async def get_all_skills(
        self,
        agent_id: str | None = None,
        category_key: str | None = None,
        source_type: str | None = None,
        q: str | None = None,
    ) -> list[SkillInfo]:
        records = self._catalog.list_records()
        installed_names: set[str] = set()
        is_main = True
        if agent_id:
            await self._ensure_agent_system_skills(agent_id)
            installed_names = await self._workspace_store.get_deployed_skill_names(agent_id)
            is_main = MainAgentProfile.is_main_agent(agent_id)

        query = (q or "").strip().lower()
        items: list[SkillInfo] = []
        for record in records.values():
            detail = record.detail.model_copy(deep=True)
            if detail.scope == "main" and not is_main:
                continue
            detail.installed = agent_id is not None and detail.name in installed_names
            detail.locked = detail.source_type == "system"
            detail.has_update = self._has_update(detail, eager=False)
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
        if agent_id:
            await self._ensure_agent_system_skills(agent_id)
            installed_names = await self._workspace_store.get_deployed_skill_names(agent_id)
            detail.installed = detail.name in installed_names
        else:
            detail.installed = False
        detail.locked = detail.source_type == "system"
        detail.has_update = self._has_update(detail, eager=True)
        return detail

    async def get_agent_skills(self, agent_id: str) -> list[AgentSkillEntry]:
        items = await self.get_all_skills(agent_id=agent_id)
        return [AgentSkillEntry.model_validate(item.model_dump()) for item in items]

    async def install_skill(self, agent_id: str, skill_name: str) -> AgentSkillEntry:
        record = self._validate_installable(agent_id, skill_name)
        await self._workspace_store.deploy_skill(
            agent_id,
            skill_name,
            record.source_path,
        )
        logger.info(f"✅ Skill installed: {skill_name} -> agent {agent_id}")
        return AgentSkillEntry.model_validate(
            (await self.get_skill_detail(skill_name, agent_id=agent_id)).model_dump()
        )

    async def uninstall_skill(self, agent_id: str, skill_name: str) -> None:
        record = self._require_record(skill_name)
        if record.detail.source_type == "system":
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be uninstalled")
        await self._workspace_store.undeploy_skill(agent_id, skill_name)

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
        for skill_name, record in self._catalog.list_records().items():
            if record.detail.source_type != "external":
                continue
            if not self._has_update(record.detail, eager=True):
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
        updated_manifest = self._update_external_manifest(skill_name)
        self._update_status_cache.pop(skill_name, None)
        updated_record = self._catalog.get_record(updated_manifest.name)
        if not updated_record:
            raise LookupError(f"Skill not found after update: {skill_name}")

        installed_names = await self._workspace_store.get_deployed_skill_names(agent_id)
        if skill_name in installed_names:
            await self._workspace_store.deploy_skill(
                agent_id,
                skill_name,
                updated_record.source_path,
            )
        return AgentSkillEntry.model_validate(
            (await self.get_skill_detail(skill_name, agent_id=agent_id)).model_dump()
        )

    async def update_global_skill(self, skill_name: str) -> SkillDetail:
        record = self._require_record(skill_name)
        if record.detail.source_type != "external":
            raise ValueError(f"Skill '{skill_name}' does not support manual update")
        self._update_external_manifest(skill_name)
        self._update_status_cache.pop(skill_name, None)
        await self._workspace_store.sync_skill_to_installed_agents(skill_name, record.source_path)
        return await self.get_skill_detail(skill_name)

    async def import_local_path(self, local_path: str) -> SkillDetail:
        manifest = self._import_service.import_local_path(local_path)
        self._update_status_cache.pop(manifest.name, None)
        return await self.get_skill_detail(manifest.name)

    async def import_uploaded_file(self, file_name: str, payload: bytes) -> SkillDetail:
        manifest = self._import_service.import_uploaded_file(file_name, payload)
        self._update_status_cache.pop(manifest.name, None)
        return await self.get_skill_detail(manifest.name)

    async def import_git(self, url: str, branch: str | None = None) -> SkillDetail:
        manifest = self._import_service.import_git(url, branch)
        self._update_status_cache.pop(manifest.name, None)
        return await self.get_skill_detail(manifest.name)

    async def import_skills_sh(self, package_spec: str, skill_slug: str) -> SkillDetail:
        manifest = self._import_service.import_skills_sh(package_spec, skill_slug)
        self._update_status_cache.pop(manifest.name, None)
        return await self.get_skill_detail(manifest.name)

    async def delete_skill(self, skill_name: str) -> None:
        record = self._require_record(skill_name)
        if not record.detail.deletable or record.detail.source_type != "external":
            raise ValueError(f"Skill '{skill_name}' cannot be deleted")

        # 中文注释：删除外部 skill 时，要先清理所有已部署 workspace，
        # 否则 Agent 会继续持有失效副本。
        await self._workspace_store.undeploy_skill_from_all_agents(skill_name)
        self._file_store.delete_skill(skill_name)
        self._update_status_cache.pop(skill_name, None)

    def search_external_skills(self, query: str, include_readme: bool = False) -> list[ExternalSkillSearchItem]:
        return self._import_service.search_skills_sh(query, include_readme)

    def get_external_skill_preview(self, detail_url: str) -> str:
        return self._import_service.fetch_skills_sh_preview(detail_url)

    def _validate_installable(self, agent_id: str, skill_name: str) -> SkillCatalogRecord:
        record = self._require_record(skill_name)
        if record.detail.source_type == "system":
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be manually installed")
        if record.detail.scope == "main" and not MainAgentProfile.is_main_agent(agent_id):
            raise ValueError(
                f"Skill '{skill_name}' is restricted to {MainAgentProfile.display_label()}"
            )
        return record

    async def _ensure_agent_system_skills(self, agent_id: str):
        """确保查询前先把系统托管 skill 补齐到目标 workspace。"""
        from agent.service.agent.agent_manager import agent_manager

        return await agent_manager.get_agent_workspace(agent_id)

    def _require_record(self, skill_name: str) -> SkillCatalogRecord:
        record = self._catalog.get_record(skill_name)
        if not record:
            raise LookupError(f"Skill not found: {skill_name}")
        return record

    def _has_update(self, detail: SkillDetail, eager: bool = False) -> bool:
        if detail.source_type != "external":
            return False
        cached = self._update_status_cache.get(detail.name)
        now = time.time()
        if cached and now - cached[0] < self.UPDATE_STATUS_TTL_SECONDS:
            return cached[1]
        if not eager:
            return cached[1] if cached else False

        manifest = self._import_service._store.read_manifest(detail.name)
        has_update = False
        if manifest.import_mode == "git":
            has_update = self._import_service.check_git_update(manifest)
        elif manifest.import_mode == "skills_sh":
            has_update = self._import_service.check_skills_sh_update(manifest)
        elif manifest.import_mode == "well_known":
            has_update = self._import_service.check_well_known_update(manifest)
        self._update_status_cache[detail.name] = (now, has_update)
        return has_update

    def _match_query(self, detail: SkillDetail, query: str) -> bool:
        haystacks = [
            detail.name,
            detail.title,
            detail.description,
            detail.category_name,
            " ".join(detail.tags),
        ]
        return any(query in value.lower() for value in haystacks if value)

    def _update_external_manifest(self, skill_name: str):
        manifest = self._import_service._store.read_manifest(skill_name)
        if manifest.import_mode == "git":
            return self._import_service.update_git_skill(manifest)
        if manifest.import_mode == "skills_sh":
            return self._import_service.update_skills_sh_skill(manifest)
        if manifest.import_mode == "well_known":
            return self._import_service.update_well_known_skill(manifest)
        raise ValueError(f"Skill '{skill_name}' does not support remote update")


skill_service = SkillService()
