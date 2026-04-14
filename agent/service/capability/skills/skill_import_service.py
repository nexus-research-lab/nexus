# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_import_service.py
# @Date   ：2026/3/30 20:38
# @Author ：Codex
# 2026/3/30 20:38   Create
# =====================================================

"""Skill 导入与更新服务。"""

from __future__ import annotations

import hashlib
import json
import shutil
import tempfile
import zipfile
from pathlib import Path

from agent.config.config import settings
from agent.schema.model_skill import ExternalSkillManifest, ExternalSkillSearchItem
from agent.service.capability.skills.skill_cli_runner import SkillCliRunner
from agent.service.capability.skills.skill_external_search_service import SkillExternalSearchService
from agent.service.capability.skills.skill_frontmatter import SkillFrontmatterParser
from agent.service.capability.skills.skill_registry_store import SkillRegistryStore
from agent.service.capability.skills.skill_well_known_loader import SkillWellKnownLoader


class SkillImportService:
    """处理外部 Skill 的导入与更新。"""

    def __init__(self) -> None:
        self._store = SkillRegistryStore()
        self._project_root = Path(__file__).resolve().parents[4]
        self._cli = SkillCliRunner()
        self._external_search = SkillExternalSearchService(self._cli)
        self._well_known = SkillWellKnownLoader(settings.HTTP_TIMEOUT)

    def import_local_path(self, local_path: str) -> ExternalSkillManifest:
        source_path = Path(local_path).expanduser()
        if not source_path.exists():
            raise FileNotFoundError(f"本地路径不存在: {local_path}")
        with tempfile.TemporaryDirectory(prefix="nexus-skill-local-") as temp_dir:
            temp_root = Path(temp_dir)
            prepared_dir = self._prepare_local_source(source_path, temp_root)
            return self._persist_external_skill(
                prepared_dir,
                source_ref=str(source_path),
                import_mode="local_path",
            )

    def import_uploaded_file(self, file_name: str, payload: bytes) -> ExternalSkillManifest:
        with tempfile.TemporaryDirectory(prefix="nexus-skill-upload-") as temp_dir:
            temp_root = Path(temp_dir)
            archive_path = temp_root / file_name
            archive_path.write_bytes(payload)
            prepared_dir = self._prepare_local_source(archive_path, temp_root)
            return self._persist_external_skill(
                prepared_dir,
                source_ref=file_name,
                import_mode="upload",
            )

    def import_git(self, url: str, branch: str | None = None) -> ExternalSkillManifest:
        with tempfile.TemporaryDirectory(prefix="nexus-skill-git-") as temp_dir:
            temp_root = Path(temp_dir)
            repo_dir = temp_root / "repo"
            clone_cmd = ["git", "clone", "--depth", "1"]
            if branch:
                clone_cmd.extend(["--branch", branch])
            clone_cmd.extend([url, str(repo_dir)])
            self._cli.run_git(clone_cmd)
            commit = self._resolve_git_head(repo_dir)
            skill_root = self._resolve_skill_root(repo_dir)
            manifest = self._persist_external_skill(
                skill_root,
                source_ref=url,
                import_mode="git",
                persist=False,
            )
            manifest.git_url = url
            manifest.git_branch = branch or self._resolve_git_branch(repo_dir)
            manifest.git_commit = commit
            manifest.skill_subdir = str(skill_root.relative_to(repo_dir))
            self._store.write_skill(manifest, skill_root)
            return manifest

    def import_skills_sh(self, package_spec: str, skill_slug: str) -> ExternalSkillManifest:
        normalized_spec = (package_spec or "").strip()
        derived_slug = (skill_slug or "").strip()
        if "@" in normalized_spec:
            spec_base, derived = normalized_spec.split("@", 1)
            spec_base = spec_base.strip()
            if not derived_slug:
                derived_slug = derived.strip()
            if spec_base and self._well_known.is_well_known_spec(spec_base):
                try:
                    return self._import_well_known_skill(spec_base, derived_slug)
                except ValueError:
                    # 中文注释：well-known 兜底失败后继续走 skills CLI。
                    pass
        else:
            normalized_spec = SkillWellKnownLoader.normalize_source(normalized_spec)

        if not derived_slug:
            raise ValueError("skills.sh 导入缺少 skill_slug")

        if "@" not in normalized_spec and self._well_known.is_well_known_spec(normalized_spec):
            return self._import_well_known_skill(normalized_spec, derived_slug)

        with tempfile.TemporaryDirectory(prefix="nexus-skill-skills-sh-") as temp_dir:
            temp_root = Path(temp_dir)
            self._run_skills_cli_add(temp_root, normalized_spec, derived_slug)
            skill_root = temp_root / ".agents" / "skills" / derived_slug
            if not (skill_root / "SKILL.md").exists():
                raise ValueError(f"skills.sh 导入失败，未找到 skill: {derived_slug}")
            manifest = self._persist_external_skill(
                skill_root,
                source_ref=normalized_spec,
                import_mode="skills_sh",
                persist=False,
            )
            manifest.package_spec = normalized_spec
            manifest.skill_slug = derived_slug
            manifest.recommendation = "来自 skills.sh 的外部技能。"
            self._store.write_skill(manifest, skill_root)
            return manifest

    def _import_well_known_skill(self, source: str, skill_slug: str) -> ExternalSkillManifest:
        with tempfile.TemporaryDirectory(prefix="nexus-skill-well-known-") as temp_dir:
            temp_root = Path(temp_dir)
            skill_root = self._well_known.load_skill(source, skill_slug, temp_root)
            if not (skill_root / "SKILL.md").exists():
                raise ValueError(f"well-known 导入失败，未找到 skill: {skill_slug}")
            manifest = self._persist_external_skill(
                skill_root,
                source_ref=source,
                import_mode="well_known",
                persist=False,
            )
            manifest.package_spec = source
            manifest.skill_slug = skill_slug
            manifest.recommendation = "来自 well-known 技能源。"
            self._store.write_skill(manifest, skill_root)
            return manifest

    def search_skills_sh(self, query: str, include_readme: bool = False) -> list[ExternalSkillSearchItem]:
        return self._external_search.search(query, include_readme)

    def fetch_skills_sh_preview(self, detail_url: str) -> str:
        return self._external_search.fetch_preview(detail_url)

    def check_git_update(self, manifest: ExternalSkillManifest) -> bool:
        if manifest.import_mode != "git" or not manifest.git_url:
            return False
        latest_commit = self._resolve_remote_commit(manifest.git_url, manifest.git_branch)
        return bool(latest_commit and latest_commit != manifest.git_commit)

    def check_skills_sh_update(self, manifest: ExternalSkillManifest) -> bool:
        """比较远端 skills.sh 导入结果与本地 registry 内容是否一致。"""
        if manifest.import_mode != "skills_sh" or not manifest.package_spec or not manifest.skill_slug:
            return False
        current_dir = self._store.skill_dir(manifest.name)
        if not current_dir.exists():
            return False

        try:
            with tempfile.TemporaryDirectory(prefix="nexus-skill-skills-sh-check-") as temp_dir:
                temp_root = Path(temp_dir)
                self._run_skills_cli_add(temp_root, manifest.package_spec, manifest.skill_slug)
                remote_dir = temp_root / ".agents" / "skills" / manifest.skill_slug
                if not (remote_dir / "SKILL.md").exists():
                    return False
                return self._hash_skill_directory(remote_dir) != self._hash_skill_directory(current_dir)
        except ValueError:
            return False

    def check_well_known_update(self, manifest: ExternalSkillManifest) -> bool:
        """比较 well-known 导入结果与本地 registry 内容是否一致。"""
        if manifest.import_mode != "well_known" or not manifest.package_spec or not manifest.skill_slug:
            return False
        current_dir = self._store.skill_dir(manifest.name)
        if not current_dir.exists():
            return False
        try:
            with tempfile.TemporaryDirectory(prefix="nexus-skill-well-known-check-") as temp_dir:
                temp_root = Path(temp_dir)
                remote_dir = self._well_known.load_skill(manifest.package_spec, manifest.skill_slug, temp_root)
                if not (remote_dir / "SKILL.md").exists():
                    return False
                return self._hash_skill_directory(remote_dir) != self._hash_skill_directory(current_dir)
        except ValueError:
            return False

    def update_git_skill(self, manifest: ExternalSkillManifest) -> ExternalSkillManifest:
        if manifest.import_mode != "git" or not manifest.git_url:
            raise ValueError(f"Skill '{manifest.name}' 不支持远程更新")
        updated = self.import_git(manifest.git_url, manifest.git_branch)
        if updated.name != manifest.name:
            raise ValueError("更新后的 skill 名称发生变化，已拒绝覆盖")
        return updated

    def update_skills_sh_skill(self, manifest: ExternalSkillManifest) -> ExternalSkillManifest:
        if manifest.import_mode != "skills_sh" or not manifest.package_spec or not manifest.skill_slug:
            raise ValueError(f"Skill '{manifest.name}' 不支持 skills.sh 更新")
        updated = self.import_skills_sh(manifest.package_spec, manifest.skill_slug)
        if updated.name != manifest.name:
            raise ValueError("更新后的 skill 名称发生变化，已拒绝覆盖")
        return updated

    def update_well_known_skill(self, manifest: ExternalSkillManifest) -> ExternalSkillManifest:
        if manifest.import_mode != "well_known" or not manifest.package_spec or not manifest.skill_slug:
            raise ValueError(f"Skill '{manifest.name}' 不支持 well-known 更新")
        updated = self._import_well_known_skill(manifest.package_spec, manifest.skill_slug)
        if updated.name != manifest.name:
            raise ValueError("更新后的 skill 名称发生变化，已拒绝覆盖")
        return updated

    def _prepare_local_source(self, source_path: Path, temp_root: Path) -> Path:
        if source_path.is_dir():
            return self._resolve_skill_root(source_path)
        if source_path.suffix.lower() != ".zip":
            raise ValueError("本地导入仅支持目录或 .zip 压缩包")
        extract_dir = temp_root / "unzipped"
        with zipfile.ZipFile(source_path, "r") as archive:
            # 校验 zip 成员路径不逃出目标目录，防止 Zip Slip 攻击
            dest_resolved = extract_dir.resolve()
            for member in archive.infolist():
                member_path = (extract_dir / member.filename).resolve()
                if not member_path.is_relative_to(dest_resolved):
                    raise ValueError(f"Zip 成员路径逃逸: {member.filename}")
            archive.extractall(extract_dir)
        return self._resolve_skill_root(extract_dir)

    def _persist_external_skill(
        self,
        skill_root: Path,
        source_ref: str,
        import_mode: str,
        persist: bool = True,
    ) -> ExternalSkillManifest:
        parsed = SkillFrontmatterParser.parse(skill_root / "SKILL.md")
        manifest = ExternalSkillManifest(
            name=str(parsed["name"]),
            title=str(parsed.get("title") or parsed["name"]),
            description=str(parsed.get("description") or ""),
            scope=str(parsed.get("scope") or "any"),
            tags=list(parsed.get("tags") or []),
            category_key=str(parsed.get("category_key") or "custom-imports"),
            category_name=str(parsed.get("category_name") or "自定义导入"),
            version=str(parsed.get("version") or "external"),
            source_ref=source_ref,
            import_mode=import_mode,
            recommendation="用户导入的自定义 Skill。",
        )
        self._ensure_name_available(manifest.name)
        # 中文注释：git / skills.sh 导入会在补充远端元数据后再统一落盘，
        # 避免先 copy 一次再覆盖 copy 一次，导致导入耗时明显增加。
        if persist:
            self._store.write_skill(manifest, skill_root)
        return manifest

    def _resolve_skill_root(self, base_dir: Path) -> Path:
        if (base_dir / "SKILL.md").exists():
            return base_dir
        candidates = sorted(path.parent for path in base_dir.rglob("SKILL.md"))
        candidates = [path for path in candidates if ".git" not in path.parts]
        if len(candidates) == 1:
            return candidates[0]
        if not candidates:
            raise ValueError("导入内容中未找到 SKILL.md")
        raise ValueError("导入内容中找到多个 SKILL.md，请确保只包含一个 skill")

    def _resolve_git_head(self, repo_dir: Path) -> str:
        return self._cli.run_git(
            ["git", "-C", str(repo_dir), "rev-parse", "HEAD"],
            strip=True,
        )

    def _resolve_git_branch(self, repo_dir: Path) -> str:
        return self._cli.run_git(
            ["git", "-C", str(repo_dir), "rev-parse", "--abbrev-ref", "HEAD"],
            strip=True,
        )

    def _resolve_remote_commit(self, git_url: str, branch: str | None) -> str:
        ref = branch or "HEAD"
        output = self._cli.run_git(["git", "ls-remote", git_url, ref], strip=True)
        if not output:
            return ""
        return output.split()[0]

    def _run_skills_cli_add(self, workdir: Path, package_spec: str, skill_slug: str) -> None:
        self._cli.run_skills_cli_add(workdir, package_spec, skill_slug)

    def _ensure_name_available(self, skill_name: str) -> None:
        """避免外部导入覆盖系统或内置 catalog。"""
        protected_roots = [
            self._project_root / "skills",
            Path.home() / ".codex" / "skills",
            Path.home() / ".agents" / "skills",
            Path.home() / ".cc-switch" / "skills",
        ]
        for root in protected_roots:
            if (root / skill_name / "SKILL.md").exists():
                raise ValueError(f"Skill '{skill_name}' 与系统/内置 skill 重名，不能导入覆盖")

    def _hash_skill_directory(self, skill_root: Path) -> str:
        """生成 skill 目录摘要，用于判断远端内容是否变化。"""
        digest = hashlib.sha256()
        for path in sorted(skill_root.rglob("*")):
            if path.is_dir() or path.name == ".nexus-skill.json":
                continue
            digest.update(path.relative_to(skill_root).as_posix().encode("utf-8"))
            digest.update(path.read_bytes())
        return digest.hexdigest()
