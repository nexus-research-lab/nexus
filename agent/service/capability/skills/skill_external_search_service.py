# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_external_search_service.py
# @Date   ：2026/04/14 09:00
# @Author ：leemysw
# 2026/04/14 09:00   Create
# =====================================================

"""外部技能搜索与预览服务。"""

from __future__ import annotations

import time
from dataclasses import dataclass
from urllib.parse import urlparse

import requests

from agent.config.config import settings
from agent.schema.model_skill import ExternalSkillSearchItem
from agent.service.capability.skills.skill_cli_runner import SkillCliRunner
from agent.service.capability.skills.skill_skills_sh_parser import SkillsShOutputParser


@dataclass(frozen=True)
class _CacheEntry:
    timestamp: float
    payload: object


class SkillExternalSearchService:
    """skills.sh 搜索与预览能力。"""

    SEARCH_CACHE_TTL_SECONDS = 60
    README_CACHE_TTL_SECONDS = 300
    PREVIEW_README_LIMIT = 0

    def __init__(self, cli: SkillCliRunner):
        self._cli = cli
        self._search_cache: dict[str, _CacheEntry] = {}
        self._readme_cache: dict[str, _CacheEntry] = {}

    def search(self, query: str, include_readme: bool = False) -> list[ExternalSkillSearchItem]:
        normalized = (query or "").strip()
        if not normalized:
            return []
        now = time.time()
        cache = self._search_cache.get(normalized)
        if cache and now - cache.timestamp < self.SEARCH_CACHE_TTL_SECONDS:
            items = self._clone_items(cache.payload)
        else:
            items = self._search_remote(normalized)
            items = self._unique_items(items)
            self._search_cache[normalized] = _CacheEntry(now, items)
            items = self._clone_items(items)

        if include_readme and items and self.PREVIEW_README_LIMIT > 0:
            limit = min(self.PREVIEW_README_LIMIT, len(items))
            for item in items[:limit]:
                if not item.detail_url:
                    continue
                item.readme_markdown = self.fetch_preview(item.detail_url)
        return items

    def fetch_preview(self, detail_url: str) -> str:
        self._validate_detail_url(detail_url)
        now = time.time()
        cache = self._readme_cache.get(detail_url)
        if cache and now - cache.timestamp < self.README_CACHE_TTL_SECONDS:
            return str(cache.payload)
        markdown = self._cli.fetch_skill_markdown(detail_url)
        self._readme_cache[detail_url] = _CacheEntry(now, markdown)
        return markdown

    @staticmethod
    def _clone_items(payload: object) -> list[ExternalSkillSearchItem]:
        return [item.model_copy(deep=True) for item in (payload or [])]

    def _search_remote(self, query: str) -> list[ExternalSkillSearchItem]:
        try:
            return self._search_via_api(query)
        except Exception:
            output = self._cli.run_skills_cli_find(query)
            return SkillsShOutputParser.parse(output)

    def _search_via_api(self, query: str) -> list[ExternalSkillSearchItem]:
        api_base = self._skills_api_base()
        search_url = f"{api_base}/api/search"
        response = requests.get(
            search_url,
            params={"q": query, "limit": str(settings.SKILLS_API_SEARCH_LIMIT)},
            timeout=settings.HTTP_TIMEOUT,
        )
        if response.status_code != 200:
            raise ValueError(f"skills.sh 搜索失败: HTTP {response.status_code}")
        try:
            payload = response.json()
        except ValueError as exc:
            raise ValueError("skills.sh 搜索返回 JSON 解析失败") from exc
        skills = payload.get("skills") or payload.get("results") or []
        items: list[ExternalSkillSearchItem] = []
        for skill in skills:
            name = str(skill.get("name") or "")
            slug = str(skill.get("id") or skill.get("slug") or "")
            source = str(skill.get("source") or "")
            installs = self._parse_installs(skill.get("installs"))
            if not name or not slug:
                continue
            base_spec = source or slug
            package_spec = f"{base_spec}@{name}"
            detail_url = f"{api_base}/{slug}"
            description = str(skill.get("description") or "来自 skills.sh 的搜索结果")
            items.append(
                ExternalSkillSearchItem(
                    name=name,
                    title=name,
                    description=description,
                    source=source,
                    package_spec=package_spec,
                    skill_slug=name,
                    installs=installs,
                    detail_url=detail_url,
                )
            )
        items.sort(key=lambda item: item.installs, reverse=True)
        return items

    @staticmethod
    def _parse_installs(raw_value: object) -> int:
        if raw_value is None:
            return 0
        try:
            return int(float(raw_value))
        except (TypeError, ValueError):
            return 0

    @staticmethod
    def _unique_items(items: list[ExternalSkillSearchItem]) -> list[ExternalSkillSearchItem]:
        seen: set[str] = set()
        unique: list[ExternalSkillSearchItem] = []
        for item in items:
            key = f"{item.package_spec}::{item.skill_slug}"
            if key in seen:
                continue
            seen.add(key)
            unique.append(item)
        return unique

    def _skills_api_base(self) -> str:
        base = (settings.SKILLS_API_URL or "").strip()
        if not base:
            raise ValueError("skills.sh API 地址为空")
        return base.rstrip("/")

    def _validate_detail_url(self, detail_url: str) -> None:
        parsed = urlparse(detail_url)
        if parsed.scheme not in ("http", "https"):
            raise ValueError("skills 预览链接协议非法")
        allowed_hosts = {"skills.sh"}
        api_base = (settings.SKILLS_API_URL or "").strip()
        if api_base:
            host = urlparse(api_base).netloc
            if host:
                allowed_hosts.add(host)
        if parsed.netloc not in allowed_hosts:
            raise ValueError("skills 预览链接域名非法")
