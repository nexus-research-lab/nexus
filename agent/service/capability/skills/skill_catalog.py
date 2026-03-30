# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_catalog.py
# @Date   ：2026/3/30 20:36
# @Author ：Codex
# 2026/3/30 20:36   Create
# =====================================================

"""Skill catalog 聚合器。"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

from agent.schema.model_skill import ExternalSkillManifest, SkillDetail, SkillInfo
from agent.service.capability.skills.skill_frontmatter import SkillFrontmatterParser
from agent.service.capability.skills.skill_registry_store import SkillRegistryStore


@dataclass
class SkillCatalogRecord:
    """Skill 聚合记录。"""

    detail: SkillDetail
    source_path: Path
    git_url: str | None = None
    git_branch: str | None = None
    git_commit: str | None = None


class SkillCatalog:
    """聚合系统、内置和外部 Skill。"""

    SYSTEM_SKILL_NAMES = {"memory-manager", "nexus-manager"}

    def __init__(self) -> None:
        self._project_root = Path(__file__).resolve().parents[4]
        self._repo_skills_root = self._project_root / "skills"
        self._manifest_path = Path(__file__).resolve().parent / "data" / "curated_skill_catalog.json"
        self._store = SkillRegistryStore()

    def list_records(
        self,
        global_states: dict[str, bool] | None = None,
        pool_installed_states: dict[str, bool] | None = None,
    ) -> dict[str, SkillCatalogRecord]:
        """聚合所有 skill 记录，状态由外部传入（来自 DB）。"""
        gs = global_states or {}
        ps = pool_installed_states or {}
        records: dict[str, SkillCatalogRecord] = {}
        records.update(self._load_system_records())
        records.update(self._load_builtin_records(gs, ps))
        records.update(self._load_external_records(gs))
        return records

    def get_record(
        self,
        skill_name: str,
        global_states: dict[str, bool] | None = None,
        pool_installed_states: dict[str, bool] | None = None,
    ) -> SkillCatalogRecord | None:
        return self.list_records(global_states, pool_installed_states).get(skill_name)

    def _load_system_records(self) -> dict[str, SkillCatalogRecord]:
        records: dict[str, SkillCatalogRecord] = {}
        for skill_name in sorted(self.SYSTEM_SKILL_NAMES):
            skill_dir = self._repo_skills_root / skill_name
            skill_md = skill_dir / "SKILL.md"
            if not skill_md.exists():
                continue
            parsed = SkillFrontmatterParser.parse(skill_md)
            records[skill_name] = SkillCatalogRecord(
                detail=SkillDetail(
                    name=parsed["name"],
                    title=str(parsed.get("title") or parsed["name"]),
                    description=str(parsed.get("description") or ""),
                    scope=str(parsed.get("scope") or "any"),
                    tags=list(parsed.get("tags") or []),
                    category_key="system-builtins",
                    category_name="系统内置",
                    source_type="system",
                    source_ref=str(skill_dir),
                    version="system",
                    locked=True,
                    global_enabled=True,
                    deletable=False,
                    installed=True,
                    readme_markdown=str(parsed.get("readme_markdown") or ""),
                    recommendation="系统内置能力，安装状态由平台托管。",
                ),
                source_path=skill_dir,
            )
        return records

    def _load_builtin_records(
        self,
        global_states: dict[str, bool],
        pool_installed_states: dict[str, bool],
    ) -> dict[str, SkillCatalogRecord]:
        manifest = json.loads(self._manifest_path.read_text(encoding="utf-8"))
        records: dict[str, SkillCatalogRecord] = {}
        for entry in manifest.get("skills", []):
            name = entry["name"]
            if name in records or name in self.SYSTEM_SKILL_NAMES:
                continue
            skill_dir = self._find_builtin_skill_dir(name)
            skill_md = skill_dir / "SKILL.md" if skill_dir else None
            if not skill_dir or not skill_md or not skill_md.exists():
                continue
            parsed = SkillFrontmatterParser.parse(skill_md)
            records[name] = SkillCatalogRecord(
                detail=SkillDetail(
                    name=parsed["name"],
                    title=str(parsed.get("title") or parsed["name"]),
                    description=str(parsed.get("description") or ""),
                    scope=str(parsed.get("scope") or "any"),
                    tags=list(parsed.get("tags") or []),
                    category_key=entry["category_key"],
                    category_name=entry["category_name"],
                    source_type="builtin",
                    source_ref=str(skill_dir),
                    version=str(parsed.get("version") or "builtin"),
                    locked=False,
                    global_enabled=global_states.get(name, True),
                    deletable=pool_installed_states.get(name, False),
                    installed=pool_installed_states.get(name, False),
                    readme_markdown=str(parsed.get("readme_markdown") or ""),
                    recommendation=str(entry.get("recommendation") or ""),
                ),
                source_path=skill_dir,
            )
        return records

    def _load_external_records(self, global_states: dict[str, bool]) -> dict[str, SkillCatalogRecord]:
        records: dict[str, SkillCatalogRecord] = {}
        for manifest in self._store.list_external_manifests():
            skill_dir = self._store.skill_dir(manifest.name)
            skill_md = skill_dir / "SKILL.md"
            if not skill_md.exists():
                continue
            parsed = SkillFrontmatterParser.parse(skill_md)
            records[manifest.name] = SkillCatalogRecord(
                detail=self._build_external_detail(manifest, parsed, skill_dir, global_states),
                source_path=skill_dir,
                git_url=manifest.git_url,
                git_branch=manifest.git_branch,
                git_commit=manifest.git_commit,
            )
        return records

    def _build_external_detail(
        self,
        manifest: ExternalSkillManifest,
        parsed: dict[str, object],
        skill_dir: Path,
        global_states: dict[str, bool],
    ) -> SkillDetail:
        return SkillDetail(
            name=manifest.name,
            title=manifest.title or str(parsed.get("title") or manifest.name),
            description=manifest.description or str(parsed.get("description") or ""),
            scope=manifest.scope,
            tags=manifest.tags or list(parsed.get("tags") or []),
            category_key=manifest.category_key,
            category_name=manifest.category_name,
            source_type="external",
            source_ref=manifest.source_ref or str(skill_dir),
            version=manifest.version,
            locked=False,
            global_enabled=global_states.get(manifest.name, True),
            deletable=True,
            installed=True,
            readme_markdown=str(parsed.get("readme_markdown") or ""),
            recommendation=manifest.recommendation,
        )

    def _find_builtin_skill_dir(self, skill_name: str) -> Path | None:
        search_roots = [
            self._repo_skills_root,
            Path.home() / ".codex" / "skills",
            Path.home() / ".agents" / "skills",
            Path.home() / ".cc-switch" / "skills",
        ]
        for root in search_roots:
            skill_dir = root / skill_name
            if (skill_dir / "SKILL.md").exists():
                return skill_dir
        return None
