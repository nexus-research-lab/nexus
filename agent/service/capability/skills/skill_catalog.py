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

from agent.schema.model_skill import ExternalSkillManifest, SkillDetail
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

    # 中文注释：仅对外暴露真正可见的系统 skill。
    # `nexus-manager` 只用于 main agent 内部编排，不进入公开 skill 市场。
    SYSTEM_SKILL_NAMES = {"memory-manager", "scheduled-task-manager"}
    INTERNAL_SKILL_NAMES = {"nexus-manager"}

    def __init__(self) -> None:
        self._project_root = Path(__file__).resolve().parents[4]
        self._repo_skills_root = self._project_root / "skills"
        self._manifest_path = Path(__file__).resolve().parent / "data" / "curated_skill_catalog.json"
        self._store = SkillRegistryStore()

    def list_records(self) -> dict[str, SkillCatalogRecord]:
        """聚合所有公开 skill 记录。"""
        records: dict[str, SkillCatalogRecord] = {}
        records.update(self._load_system_records())
        records.update(self._load_builtin_records())
        records.update(self._load_external_records())
        return records

    def get_record(
        self,
        skill_name: str,
    ) -> SkillCatalogRecord | None:
        return self.list_records().get(skill_name)

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
                    deletable=False,
                    installed=False,
                    readme_markdown=str(parsed.get("readme_markdown") or ""),
                    recommendation="系统内置能力，安装状态由平台托管。",
                ),
                source_path=skill_dir,
            )
        return records

    def _load_builtin_records(self) -> dict[str, SkillCatalogRecord]:
        manifest = json.loads(self._manifest_path.read_text(encoding="utf-8"))
        records: dict[str, SkillCatalogRecord] = {}
        for entry in manifest.get("skills", []):
            name = entry["name"]
            if name in records or name in self.SYSTEM_SKILL_NAMES or name in self.INTERNAL_SKILL_NAMES:
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
                    deletable=False,
                    installed=False,
                    readme_markdown=str(parsed.get("readme_markdown") or ""),
                    recommendation=str(entry.get("recommendation") or ""),
                ),
                source_path=skill_dir,
            )
        records.update(
            self._load_fallback_builtin_records(
                records,
            )
        )
        return records

    def _load_external_records(self) -> dict[str, SkillCatalogRecord]:
        records: dict[str, SkillCatalogRecord] = {}
        for manifest in self._store.list_external_manifests():
            skill_dir = self._store.skill_dir(manifest.name)
            skill_md = skill_dir / "SKILL.md"
            if not skill_md.exists():
                continue
            parsed = SkillFrontmatterParser.parse(skill_md)
            records[manifest.name] = SkillCatalogRecord(
                detail=self._build_external_detail(manifest, parsed, skill_dir),
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
            deletable=True,
            installed=False,
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

    def _load_fallback_builtin_records(
        self,
        existing_records: dict[str, SkillCatalogRecord],
    ) -> dict[str, SkillCatalogRecord]:
        """补齐未在 curated manifest 中声明、但本地实际可用的 builtin skills。"""
        records: dict[str, SkillCatalogRecord] = {}
        existing_names = set(existing_records)
        for skill_dir in self._iter_builtin_skill_dirs():
            skill_name = skill_dir.name
            if (
                skill_name in self.SYSTEM_SKILL_NAMES
                or skill_name in self.INTERNAL_SKILL_NAMES
                or skill_name in existing_names
            ):
                continue
            parsed = SkillFrontmatterParser.parse(skill_dir / "SKILL.md")
            category_key = str(parsed.get("category_key") or "builtin-misc")
            category_name = str(parsed.get("category_name") or "扩展能力")
            recommendation = str(
                parsed.get("recommendation") or "自动收录的本地可用能力。"
            )
            records[skill_name] = SkillCatalogRecord(
                detail=SkillDetail(
                    name=parsed["name"],
                    title=str(parsed.get("title") or parsed["name"]),
                    description=str(parsed.get("description") or ""),
                    scope=str(parsed.get("scope") or "any"),
                    tags=list(parsed.get("tags") or []),
                    category_key=category_key,
                    category_name=category_name,
                    source_type="builtin",
                    source_ref=str(skill_dir),
                    version=str(parsed.get("version") or "builtin"),
                    locked=False,
                    deletable=False,
                    installed=False,
                    readme_markdown=str(parsed.get("readme_markdown") or ""),
                    recommendation=recommendation,
                ),
                source_path=skill_dir,
            )
        return records

    def _iter_builtin_skill_dirs(self):
        """遍历所有本地可发现的 builtin skill 目录。"""
        search_roots = [
            self._repo_skills_root,
            Path.home() / ".codex" / "skills",
            Path.home() / ".agents" / "skills",
            Path.home() / ".cc-switch" / "skills",
        ]
        seen_names: set[str] = set()
        for root in search_roots:
            if not root.exists():
                continue
            for skill_dir in sorted(root.iterdir()):
                if not skill_dir.is_dir() or skill_dir.name.startswith("."):
                    continue
                if not (skill_dir / "SKILL.md").exists():
                    continue
                if skill_dir.name in seen_names:
                    continue
                seen_names.add(skill_dir.name)
                yield skill_dir
