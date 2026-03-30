# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_registry_store.py
# @Date   ：2026/3/30 20:34
# @Author ：Codex
# 2026/3/30 20:34   Create
# =====================================================

"""外部 Skill 注册表与清单文件存储。"""

from __future__ import annotations

import json
import shutil
from pathlib import Path

from agent.config.config import settings
from agent.schema.model_skill import ExternalSkillManifest


class SkillRegistryStore:
    """管理受控 Skill 仓库与清单文件。"""

    def __init__(self) -> None:
        self._root = Path(settings.CACHE_FILE_DIR).expanduser() / "skills" / "registry"
        self._root.mkdir(parents=True, exist_ok=True)

    @property
    def root(self) -> Path:
        return self._root

    def skill_dir(self, skill_name: str) -> Path:
        return self._root / skill_name

    def manifest_path(self, skill_name: str) -> Path:
        return self.skill_dir(skill_name) / ".nexus-skill.json"

    def list_external_manifests(self) -> list[ExternalSkillManifest]:
        manifests: list[ExternalSkillManifest] = []
        for skill_dir in sorted(self._root.iterdir()):
            if not skill_dir.is_dir():
                continue
            manifest_path = self.manifest_path(skill_dir.name)
            if not manifest_path.exists():
                continue
            manifests.append(self.read_manifest(skill_dir.name))
        return manifests

    def read_manifest(self, skill_name: str) -> ExternalSkillManifest:
        payload = json.loads(self.manifest_path(skill_name).read_text(encoding="utf-8"))
        return ExternalSkillManifest.model_validate(payload)

    def write_skill(self, manifest: ExternalSkillManifest, source_dir: Path) -> Path:
        target_dir = self.skill_dir(manifest.name)
        if target_dir.exists():
            shutil.rmtree(target_dir)
        shutil.copytree(source_dir, target_dir)
        self.manifest_path(manifest.name).write_text(
            manifest.model_dump_json(indent=2),
            encoding="utf-8",
        )
        return target_dir

    def delete_skill(self, skill_name: str) -> None:
        target_dir = self.skill_dir(skill_name)
        if target_dir.exists():
            shutil.rmtree(target_dir)
