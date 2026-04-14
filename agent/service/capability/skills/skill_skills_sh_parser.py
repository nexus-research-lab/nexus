# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_skills_sh_parser.py
# @Date   ：2026/04/14 09:00
# @Author ：leemysw
# 2026/04/14 09:00   Create
# =====================================================

"""skills.sh 搜索输出解析。"""

from __future__ import annotations

import re

from agent.schema.model_skill import ExternalSkillSearchItem


class SkillsShOutputParser:
    """解析 skills.sh CLI 输出。"""

    @classmethod
    def parse(cls, output: str) -> list[ExternalSkillSearchItem]:
        clean = re.sub(r"\x1b\[[0-9;?]*[A-Za-z]", "", output)
        items: list[ExternalSkillSearchItem] = []
        current_spec = ""
        current_installs = 0
        for raw_line in clean.splitlines():
            line = raw_line.strip()
            if not line:
                continue
            if line.startswith("#") and "SKILL" in line:
                continue
            table_match = re.match(
                r"^(?P<index>\d+)\s+(?P<slug>\S+)\s+(?P<source>\S+)\s+(?P<installs>[0-9.]+[KkMm]?)$",
                line,
            )
            if table_match:
                skill_slug = table_match.group("slug")
                source = table_match.group("source")
                installs = cls._parse_install_count(table_match.group("installs"))
                package_spec = f"{source}@{skill_slug}"
                detail_url = f"https://skills.sh/{source}/{skill_slug}"
                items.append(
                    ExternalSkillSearchItem(
                        name=skill_slug,
                        title=skill_slug,
                        description="来自 skills.sh 的搜索结果",
                        source=source,
                        package_spec=package_spec,
                        skill_slug=skill_slug,
                        installs=installs,
                        detail_url=detail_url,
                    )
                )
                current_spec = ""
                current_installs = 0
                continue
            if "@" in line and "http" not in line:
                match = re.match(
                    r"^(?P<spec>\S+@\S+)(?:\s+(?P<installs>[0-9.]+[KkMm]?))?\s*(?:installs)?$",
                    line,
                )
                if match:
                    current_spec = match.group("spec")
                    installs_text = match.group("installs")
                    current_installs = cls._parse_install_count(installs_text or "0")
                continue
            if " installs" in line and current_spec:
                installs_match = re.search(r"([0-9.]+[KkMm]?)\s+installs", line)
                if installs_match:
                    current_installs = cls._parse_install_count(installs_match.group(1))
                continue
            if line.startswith("└ https://skills.sh/") and current_spec:
                detail_url = line.replace("└ ", "").strip()
                source, skill_slug = current_spec.split("@", 1)
                items.append(
                    ExternalSkillSearchItem(
                        name=skill_slug,
                        title=skill_slug,
                        description="来自 skills.sh 的搜索结果",
                        source=source,
                        package_spec=current_spec,
                        skill_slug=skill_slug,
                        installs=current_installs,
                        detail_url=detail_url,
                    )
                )
                current_spec = ""
                current_installs = 0
        return items

    @staticmethod
    def _parse_install_count(text: str) -> int:
        if not text:
            return 0
        lower = text.lower()
        if lower.endswith("m"):
            return int(float(text[:-1]) * 1000000)
        if lower.endswith("k"):
            return int(float(text[:-1]) * 1000)
        return int(float(text))
