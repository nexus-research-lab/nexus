# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_well_known_loader.py
# @Date   ：2026/04/14 09:00
# @Author ：leemysw
# 2026/04/14 09:00   Create
# =====================================================

"""Well-known skills 索引解析与下载。"""

from __future__ import annotations

import hashlib
import json
import os
import re
import tarfile
import zipfile
from pathlib import Path
from urllib.parse import urljoin, urlparse

import requests

from agent.config.config import settings


class SkillWellKnownLoader:
    """基于 .well-known 规范加载技能。"""

    _DOMAIN_PATTERN = re.compile(r"^[A-Za-z0-9.-]+\.[A-Za-z]{2,}(/|$)")
    _INDEX_PATHS = (
        "/.well-known/agent-skills/index.json",
        "/.well-known/skills/index.json",
    )

    def __init__(self, timeout_seconds: int):
        self._timeout = timeout_seconds

    @classmethod
    def is_well_known_spec(cls, spec: str) -> bool:
        normalized = (spec or "").strip()
        if not normalized:
            return False
        candidate = normalized
        if not candidate.startswith(("http://", "https://")):
            if not cls._DOMAIN_PATTERN.match(candidate):
                return False
            candidate = f"https://{candidate}"
        parsed = urlparse(candidate)
        if parsed.path in ("", "/") or parsed.path.startswith("/.well-known/"):
            return True
        return False

    @classmethod
    def normalize_source(cls, spec: str) -> str:
        normalized = (spec or "").strip()
        if not normalized:
            return normalized
        if "://" in normalized:
            return normalized
        if cls._DOMAIN_PATTERN.match(normalized):
            return f"https://{normalized}"
        return normalized

    def load_skill(self, source: str, skill_slug: str, temp_root: Path) -> Path:
        index_url, index = self._fetch_index(source)
        entry = self._find_entry(index, skill_slug)
        return self._materialize_skill(entry, index_url, temp_root)

    def _fetch_index(self, source: str) -> tuple[str, dict]:
        normalized = self.normalize_source(source)
        candidates = self._build_index_candidates(normalized)
        last_error = None
        for url in candidates:
            try:
                payload = self._get_json(url)
                if isinstance(payload, dict):
                    return url, payload
            except Exception as exc:
                last_error = exc
                continue
        if last_error:
            raise ValueError(f"无法加载 skills index.json: {last_error}") from last_error
        raise ValueError("未找到可用的 skills index.json")

    def _build_index_candidates(self, source: str) -> list[str]:
        if "/.well-known/" in source and source.endswith("/index.json"):
            return [source]
        if "/.well-known/agent-skills" in source or "/.well-known/skills" in source:
            base = source.rstrip("/")
            if base.endswith("/.well-known/agent-skills") or base.endswith("/.well-known/skills"):
                return [f"{base}/index.json"]
        base = source.rstrip("/")
        return [f"{base}{path}" for path in self._INDEX_PATHS]

    def _find_entry(self, index: dict, skill_slug: str) -> dict:
        skills = index.get("skills") or []
        for entry in skills:
            if str(entry.get("name")) == skill_slug:
                return entry
        names = [str(entry.get("name")) for entry in skills if entry.get("name")]
        preview = ", ".join(names[:8])
        raise ValueError(f"skills 索引中未找到 skill: {skill_slug} (已提供: {preview})")

    def _materialize_skill(self, entry: dict, index_url: str, temp_root: Path) -> Path:
        entry_type = str(entry.get("type") or "").strip().lower()
        entry_url = str(entry.get("url") or "").strip()
        files = entry.get("files") or []
        digest = str(entry.get("digest") or "").strip()

        if not entry_type:
            if entry_url:
                entry_type = "skill-md" if entry_url.endswith("SKILL.md") else "archive"
            elif files:
                entry_type = "files"

        if entry_type == "skill-md":
            return self._download_skill_md(entry_url, digest, index_url, temp_root)
        if entry_type == "archive":
            return self._download_archive(entry_url, digest, index_url, temp_root)
        if entry_type == "files":
            return self._download_files(files, index_url, temp_root)
        raise ValueError(f"未知的 skill 分发类型: {entry_type or 'unknown'}")

    def _download_skill_md(self, url: str, digest: str, index_url: str, temp_root: Path) -> Path:
        if not url:
            raise ValueError("skills index 缺少 url")
        resolved = urljoin(index_url, url)
        skill_dir = temp_root / "skill"
        skill_dir.mkdir(parents=True, exist_ok=True)
        target = skill_dir / "SKILL.md"
        self._download_to_path(resolved, target, digest)
        return skill_dir

    def _download_files(self, files: list[str], index_url: str, temp_root: Path) -> Path:
        if not files:
            raise ValueError("skills index 缺少 files")
        skill_dir = temp_root / "skill"
        skill_dir.mkdir(parents=True, exist_ok=True)
        for relative in files:
            relative_path = self._safe_relative_path(str(relative))
            target = (skill_dir / relative_path).resolve()
            if not target.is_relative_to(skill_dir.resolve()):
                raise ValueError(f"skills 文件路径非法: {relative}")
            target.parent.mkdir(parents=True, exist_ok=True)
            resolved = urljoin(index_url, str(relative))
            self._download_to_path(resolved, target, "")
        if not (skill_dir / "SKILL.md").exists():
            raise ValueError("skills files 缺少 SKILL.md")
        return skill_dir

    def _download_archive(self, url: str, digest: str, index_url: str, temp_root: Path) -> Path:
        if not url:
            raise ValueError("skills index 缺少 url")
        resolved = urljoin(index_url, url)
        archive_path = temp_root / "skill-archive"
        self._download_to_path(resolved, archive_path, digest)
        extract_dir = temp_root / "archive"
        extract_dir.mkdir(parents=True, exist_ok=True)

        if resolved.endswith((".tar.gz", ".tgz")):
            self._extract_tar(archive_path, extract_dir)
        elif resolved.endswith(".zip"):
            self._extract_zip(archive_path, extract_dir)
        else:
            raise ValueError("skills archive 只支持 .tar.gz 或 .zip")

        return self._resolve_skill_root(extract_dir)

    def _extract_zip(self, archive_path: Path, extract_dir: Path) -> None:
        with zipfile.ZipFile(archive_path, "r") as archive:
            dest_resolved = extract_dir.resolve()
            for member in archive.infolist():
                member_path = (extract_dir / member.filename).resolve()
                if not member_path.is_relative_to(dest_resolved):
                    raise ValueError(f"Zip 成员路径逃逸: {member.filename}")
            archive.extractall(extract_dir)

    def _extract_tar(self, archive_path: Path, extract_dir: Path) -> None:
        with tarfile.open(archive_path, "r:*") as archive:
            dest_resolved = extract_dir.resolve()
            for member in archive.getmembers():
                if member.issym() or member.islnk():
                    raise ValueError(f"tar 包含软/硬链接: {member.name}")
                member_path = (extract_dir / member.name).resolve()
                if not member_path.is_relative_to(dest_resolved):
                    raise ValueError(f"tar 成员路径逃逸: {member.name}")
            archive.extractall(extract_dir)

    def _download_to_path(self, url: str, target: Path, digest: str) -> None:
        target.parent.mkdir(parents=True, exist_ok=True)
        sha256 = hashlib.sha256()
        response = self._get_stream(url)
        with target.open("wb") as f:
            for chunk in response.iter_content(chunk_size=1024 * 64):
                if not chunk:
                    continue
                f.write(chunk)
                sha256.update(chunk)
        self._verify_digest(sha256.hexdigest(), digest)

    def _verify_digest(self, actual_hex: str, digest: str) -> None:
        if not digest:
            return
        if ":" in digest:
            algo, expected = digest.split(":", 1)
            if algo.lower() != "sha256":
                return
        else:
            expected = digest
        if actual_hex.lower() != expected.strip().lower():
            raise ValueError("skills 文件摘要校验失败")

    def _get_json(self, url: str) -> dict:
        response = self._get_stream(url)
        text = response.text
        try:
            return json.loads(text)
        except json.JSONDecodeError as exc:
            raise ValueError(f"skills index.json 解析失败: {exc}") from exc

    def _get_stream(self, url: str) -> requests.Response:
        proxies = None
        proxy = os.getenv("HTTP_PROXY")
        if proxy and url.startswith("https"):
            proxies = {"http": proxy, "https": proxy}
        response = requests.get(
            url,
            timeout=self._timeout,
            proxies=proxies,
            stream=True,
        )
        if response.status_code != 200:
            raise ValueError(f"HTTP {response.status_code}: {url}")
        return response

    @staticmethod
    def _safe_relative_path(raw: str) -> Path:
        normalized = raw.strip().lstrip("/").replace("\\", "/")
        if not normalized or normalized.startswith("..") or "/../" in normalized:
            raise ValueError("skills 文件路径非法")
        return Path(normalized)

    @staticmethod
    def _resolve_skill_root(base_dir: Path) -> Path:
        if (base_dir / "SKILL.md").exists():
            return base_dir
        candidates = sorted(path.parent for path in base_dir.rglob("SKILL.md"))
        candidates = [path for path in candidates if ".git" not in path.parts]
        if len(candidates) == 1:
            return candidates[0]
        if not candidates:
            raise ValueError("导入内容中未找到 SKILL.md")
        raise ValueError("导入内容中找到多个 SKILL.md，请确保只包含一个 skill")
