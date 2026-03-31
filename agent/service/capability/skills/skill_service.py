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

import time
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
from agent.service.capability.skills.skill_catalog import SkillCatalog
from agent.service.capability.skills.skill_import_service import SkillImportService
from agent.service.capability.skills.skill_registry_store import SkillRegistryStore
from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer
from agent.storage.agent_repository import agent_repository
from agent.storage.skill_repository import skill_repository
from agent.utils.logger import logger


class SkillService:
    """负责 Skill Marketplace 查询、导入、安装与更新。"""

    BASE_SKILL_NAMES = ("memory-manager",)
    MAIN_AGENT_SKILL_NAMES = ("nexus-manager",)
    UPDATE_STATUS_TTL_SECONDS = 300

    def __init__(self) -> None:
        self._catalog = SkillCatalog()
        self._import_service = SkillImportService()
        self._file_store = SkillRegistryStore()
        self._update_status_cache: dict[str, tuple[float, bool]] = {}

    async def _load_states(self) -> tuple[dict[str, bool], dict[str, bool]]:
        """从数据库加载全局启用状态和资源池安装状态。"""
        global_states = await skill_repository.get_global_states()
        pool_states = await skill_repository.get_pool_installed_states()
        return global_states, pool_states

    async def get_all_skills(
        self,
        agent_id: str | None = None,
        category_key: str | None = None,
        source_type: str | None = None,
        q: str | None = None,
    ) -> list[SkillInfo]:
        global_states, pool_states = await self._load_states()
        records = self._catalog.list_records(global_states, pool_states)
        installed_names: set[str] = set()
        is_main = True
        if agent_id:
            await self._resolve_agent(agent_id)
            installed_names = set(await skill_repository.get_agent_skill_names(agent_id))
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
                pool_states,
                resource_pool_mode=agent_id is None,
            )
            detail.locked = detail.source_type == "system"
            detail.has_update = self._has_update(detail, record, detail.installed, eager=False)
            if category_key and detail.category_key != category_key:
                continue
            if source_type and detail.source_type != source_type:
                continue
            if query and not self._match_query(detail, query):
                continue
            items.append(SkillInfo.model_validate(detail.model_dump()))
        return sorted(items, key=lambda item: (item.category_name, item.title.lower()))

    async def get_skill_detail(self, skill_name: str, agent_id: str | None = None) -> SkillDetail:
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        detail = record.detail.model_copy(deep=True)
        installed_names: set[str] = set()
        if agent_id:
            await self._resolve_agent(agent_id)
            installed_names = set(await skill_repository.get_agent_skill_names(agent_id))
        detail.installed = self._is_installed(
            detail.name,
            installed_names,
            detail.source_type,
            pool_states,
            resource_pool_mode=agent_id is None,
        )
        detail.locked = detail.source_type == "system"
        detail.has_update = self._has_update(detail, record, detail.installed, eager=True)
        return detail

    async def get_agent_skills(self, agent_id: str) -> list[AgentSkillEntry]:
        # 中文注释：分两步获取数据，避免混淆"资源池可用"和"agent 已部署"两个概念。
        # _is_installed(resource_pool_mode=False) 返回的是 agent 是否已部署该 skill，
        # 而 Agent 配置页需要的是"资源池里是否有这个 skill 且全局启用"。
        # 因此单独从 DB 拿 pool_states 用于过滤可见性。
        global_states, pool_states = await self._load_states()
        items = await self.get_all_skills(agent_id=agent_id)

        result: list[AgentSkillEntry] = []
        for item in items:
            if item.locked:
                # system skill: 永远可在 Agent 里配置
                result.append(AgentSkillEntry.model_validate(item.model_dump()))
                continue
            # 判断该 skill 是否在全局资源池中：
            # - external: 导入即入池，始终为 True
            # - builtin / system: 需要显式安装到资源池（PoolSkill 表）
            in_pool = (
                item.source_type == "external"
                or pool_states.get(item.name, False)
            )
            if in_pool and item.global_enabled:
                result.append(AgentSkillEntry.model_validate(item.model_dump()))
        return result

    async def install_skill(self, agent_id: str, skill_name: str) -> AgentSkillEntry:
        global_states, pool_states = await self._load_states()
        record = self._validate_installable(agent_id, skill_name, global_states, pool_states)
        agent = await self._resolve_agent(agent_id)
        deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
        deployer.deploy_skill(skill_name, source_dir=record.source_path)
        # 写入 DB 记录 Agent-Skill 关联
        await skill_repository.add_agent_skill(agent_id, skill_name)
        logger.info(f"✅ Skill installed: {skill_name} → agent {agent_id}")
        return AgentSkillEntry.model_validate(
            (await self.get_skill_detail(skill_name, agent_id=agent_id)).model_dump()
        )

    async def uninstall_skill(self, agent_id: str, skill_name: str) -> None:
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        if record.detail.source_type == "system":
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be uninstalled")
        agent = await self._resolve_agent(agent_id)
        deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
        deployer.undeploy_skill(skill_name)
        # 从 DB 移除 Agent-Skill 关联
        await skill_repository.remove_agent_skill(agent_id, skill_name)

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
        global_states, pool_states = await self._load_states()
        for skill_name in self._catalog.list_records(global_states, pool_states).keys():
            try:
                record = self._require_record(skill_name, global_states, pool_states)
            except LookupError:
                continue
            if record.detail.source_type != "external":
                continue
            if not self._has_update(record.detail, record, False, eager=True):
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
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        if record.detail.source_type != "external":
            raise ValueError(f"Skill '{skill_name}' does not support manual update")
        manifest = self._import_service._store.read_manifest(skill_name)
        if manifest.import_mode == "git":
            updated_manifest = self._import_service.update_git_skill(manifest)
        elif manifest.import_mode == "skills_sh":
            updated_manifest = self._import_service.update_skills_sh_skill(manifest)
        else:
            raise ValueError(f"Skill '{skill_name}' does not support remote update")
        gs2, ps2 = await self._load_states()
        updated_record = self._catalog.get_record(updated_manifest.name, gs2, ps2)
        if not updated_record:
            raise LookupError(f"Skill not found after update: {skill_name}")
        # 如果该 Agent 已安装此 skill，同步更新 workspace 文件
        agent_skills = await skill_repository.get_agent_skill_names(agent_id)
        if skill_name in agent_skills:
            agent = await self._resolve_agent(agent_id)
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
        await self._sync_skill_to_installed_agents(skill_name)
        return await self.get_skill_detail(skill_name)

    async def import_local_path(self, local_path: str) -> SkillDetail:
        manifest = self._import_service.import_local_path(local_path)
        await skill_repository.set_pool_installed(manifest.name, True)
        return await self.get_skill_detail(manifest.name)

    async def import_uploaded_file(self, file_name: str, payload: bytes) -> SkillDetail:
        manifest = self._import_service.import_uploaded_file(file_name, payload)
        await skill_repository.set_pool_installed(manifest.name, True)
        return await self.get_skill_detail(manifest.name)

    async def import_git(self, url: str, branch: str | None = None) -> SkillDetail:
        manifest = self._import_service.import_git(url, branch)
        await skill_repository.set_pool_installed(manifest.name, True)
        return await self.get_skill_detail(manifest.name)

    async def import_skills_sh(self, package_spec: str, skill_slug: str) -> SkillDetail:
        manifest = self._import_service.import_skills_sh(package_spec, skill_slug)
        await skill_repository.set_pool_installed(manifest.name, True)
        return await self.get_skill_detail(manifest.name)

    def search_external_skills(self, query: str) -> list[ExternalSkillSearchItem]:
        return self._import_service.search_skills_sh(query)

    def _validate_installable(
        self,
        agent_id: str,
        skill_name: str,
        global_states: dict[str, bool],
        pool_states: dict[str, bool],
    ):
        record = self._require_record(skill_name, global_states, pool_states)
        if record.detail.source_type == "system":
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be manually installed")
        if not record.detail.installed:
            raise ValueError(f"Skill '{skill_name}' is not installed in the pool")
        if not record.detail.global_enabled:
            raise ValueError(f"Skill '{skill_name}' is globally disabled")
        is_main = MainAgentProfile.is_main_agent(agent_id)
        if record.detail.scope == "main" and not is_main:
            raise ValueError(f"Skill '{skill_name}' is restricted to main agent")
        return record

    async def set_global_enabled(self, skill_name: str, enabled: bool) -> SkillDetail:
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        if record.detail.locked:
            raise ValueError(f"Skill '{skill_name}' is system-managed and cannot be globally disabled")
        if not record.detail.installed:
            raise ValueError(f"Skill '{skill_name}' is not installed in the pool")
        await skill_repository.set_global_enabled(skill_name, enabled)
        await self._sync_skill_to_installed_agents(skill_name)
        return await self.get_skill_detail(skill_name)

    async def delete_from_pool(self, skill_name: str) -> None:
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        if not record.detail.deletable:
            raise ValueError(f"Skill '{skill_name}' cannot be deleted from the pool")

        # 中文注释：删除技能池中的 skill 时，先从 DB 移除所有 Agent 关联，
        # 再清理各 Agent workspace 中的部署副本，最后删除技能池记录。
        affected_agent_ids = await skill_repository.remove_skill_from_all_agents(skill_name)
        for aid in affected_agent_ids:
            agent = await agent_repository.get_agent(aid)
            if agent:
                deployer = WorkspaceSkillDeployer(aid, Path(agent.workspace_path))
                deployer.undeploy_skill(skill_name)

        if record.detail.source_type == "external":
            self._file_store.delete_skill(skill_name)
        await skill_repository.delete_pool_skill(skill_name)

    async def install_to_pool(self, skill_name: str) -> SkillDetail:
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        if record.detail.locked:
            raise ValueError(f"Skill '{skill_name}' is system-managed and already available")
        if record.detail.source_type == "external":
            raise ValueError(f"Skill '{skill_name}' is already imported into the pool")
        await skill_repository.set_pool_installed(skill_name, True)
        await skill_repository.set_global_enabled(skill_name, True)
        return await self.get_skill_detail(skill_name)

    async def _resolve_agent(self, agent_id: str):
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise LookupError(f"Agent not found: {agent_id}")
        return agent

    def _require_record(
        self,
        skill_name: str,
        global_states: dict[str, bool] | None = None,
        pool_states: dict[str, bool] | None = None,
    ):
        record = self._catalog.get_record(skill_name, global_states, pool_states)
        if not record:
            raise LookupError(f"Skill not found: {skill_name}")
        return record

    def _is_installed(
        self,
        skill_name: str,
        installed_names: set[str],
        source_type: str,
        pool_states: dict[str, bool] | None = None,
        resource_pool_mode: bool = False,
    ) -> bool:
        if resource_pool_mode:
            return source_type == "system" or (pool_states or {}).get(skill_name, False)
        if skill_name in self.BASE_SKILL_NAMES:
            return True
        if skill_name in self.MAIN_AGENT_SKILL_NAMES:
            return True
        return source_type != "system" and skill_name in installed_names

    def _has_update(self, detail: SkillDetail, record, installed: bool, eager: bool = False) -> bool:
        if detail.source_type != "external":
            return False
        cached = self._update_status_cache.get(detail.name)
        now = time.time()
        if cached and now - cached[0] < self.UPDATE_STATUS_TTL_SECONDS:
            return cached[1]
        if not eager:
            return cached[1] if cached else False
        manifest = self._import_service._store.read_manifest(detail.name)
        if manifest.import_mode == "git":
            has_update = self._import_service.check_git_update(manifest)
            self._update_status_cache[detail.name] = (now, has_update)
            return has_update
        if manifest.import_mode == "skills_sh":
            has_update = self._import_service.check_skills_sh_update(manifest)
            self._update_status_cache[detail.name] = (now, has_update)
            return has_update
        return False

    async def sync_agent_skills(self, agent_id: str, desired_skill_names: list[str]) -> list[str]:
        """按目标 skill 列表同步 Agent workspace 与 DB 关联。"""
        await self._resolve_agent(agent_id)
        desired = set(desired_skill_names)
        current = set(await skill_repository.get_agent_skill_names(agent_id))

        # 中文注释：先移除不再需要的 skill，再安装新增 skill，
        # 这样可以确保 workspace 中最终只保留当前配置允许的能力。
        for skill_name in sorted(current - desired):
            await self.uninstall_skill(agent_id, skill_name)
        for skill_name in sorted(desired - current):
            await self.install_skill(agent_id, skill_name)
        return sorted(desired)

    async def _sync_skill_to_installed_agents(self, skill_name: str) -> None:
        """根据全局状态，把某个 skill 同步到所有已安装该 skill 的 Agent。"""
        global_states, pool_states = await self._load_states()
        record = self._require_record(skill_name, global_states, pool_states)
        agent_ids = await skill_repository.get_agent_ids_by_skill_name(skill_name)
        for agent_id in agent_ids:
            agent = await agent_repository.get_agent(agent_id)
            if not agent:
                continue
            deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
            if record.detail.global_enabled:
                deployer.deploy_skill(skill_name, source_dir=record.source_path)
            else:
                deployer.undeploy_skill(skill_name)

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
