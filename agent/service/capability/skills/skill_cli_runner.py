# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_cli_runner.py
# @Date   ：2026/04/14 09:00
# @Author ：leemysw
# 2026/04/14 09:00   Create
# =====================================================

"""Skills CLI 与 Git 命令执行封装。"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
from pathlib import Path

from agent.config.config import settings


class SkillCliRunner:
    """封装外部 CLI 调用，输出清晰错误信息。"""

    def run_git(self, command: list[str], strip: bool = False) -> str:
        result = subprocess.run(command, capture_output=True, text=True, check=False)
        if result.returncode != 0:
            raise ValueError(self._build_error("Git 操作失败", command, result))
        return result.stdout.strip() if strip else result.stdout

    def run_skills_cli_find(self, query: str) -> str:
        self._ensure_npm_tools()
        command = ["npx", "-y", "skills", "find", query]
        result = subprocess.run(command, capture_output=True, text=True, check=False, env=self._npm_env())
        if result.returncode != 0:
            raise ValueError(self._build_error("skills.sh 搜索失败", command, result))
        return result.stdout

    def run_skills_cli_add(self, workdir: Path, package_spec: str, skill_slug: str) -> None:
        self._ensure_npm_tools()
        normalized_spec = self._normalize_package_spec(package_spec)
        init_cmd = ["npm", "init", "-y"]
        init_result = subprocess.run(
            init_cmd,
            cwd=workdir,
            capture_output=True,
            text=True,
            check=False,
            env=self._npm_env(),
        )
        if init_result.returncode != 0:
            raise ValueError(self._build_error("npm 初始化失败", init_cmd, init_result))

        command = [
            "npx", "-y", "skills", "add", normalized_spec,
            "--skill", skill_slug, "-y", "--copy",
        ]
        result = subprocess.run(
            command,
            cwd=workdir,
            capture_output=True,
            text=True,
            check=False,
            env=self._npm_env(),
        )
        if result.returncode != 0:
            raise ValueError(self._build_error("skills.sh 导入失败", command, result))

    def fetch_skill_markdown(self, detail_url: str) -> str:
        result = subprocess.run(
            ["curl", "-L", "--max-time", "20", detail_url],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            return ""
        html = result.stdout
        marker = '"dangerouslySetInnerHTML":{"__html":"'
        if marker not in html:
            return ""
        fragment = html.split(marker, 1)[1].split('"}}', 1)[0]
        decoded = bytes(fragment, "utf-8").decode("unicode_escape")
        # 中文注释：简单把 HTML 文本还原为可读 markdown-ish 文本。
        decoded = re.sub(r"<pre.*?><code.*?>", "```text\n", decoded)
        decoded = decoded.replace("</code></pre>", "\n```")
        decoded = re.sub(r"<h1>(.*?)</h1>", r"# \1\n", decoded)
        decoded = re.sub(r"<h2>(.*?)</h2>", r"## \1\n", decoded)
        decoded = re.sub(r"<h3>(.*?)</h3>", r"### \1\n", decoded)
        decoded = re.sub(r"<li>(.*?)</li>", r"- \1", decoded)
        decoded = re.sub(r"<p>(.*?)</p>", r"\1\n", decoded)
        decoded = re.sub(r"<[^>]+>", "", decoded)
        decoded = decoded.replace("&#x3C;", "<")
        return decoded.strip()

    @staticmethod
    def _npm_env() -> dict[str, str]:
        env = dict(os.environ)
        env.setdefault("NPM_CONFIG_UPDATE_NOTIFIER", "false")
        env.setdefault("NPM_CONFIG_FUND", "false")
        env.setdefault("NPM_CONFIG_AUDIT", "false")
        if settings.NPM_REGISTRY:
            env.setdefault("NPM_CONFIG_REGISTRY", settings.NPM_REGISTRY)
        return env

    @staticmethod
    def _ensure_npm_tools() -> None:
        if not shutil.which("npm") or not shutil.which("npx"):
            raise ValueError("缺少 npm/npx，无法使用 skills.sh 导入")

    @staticmethod
    def _normalize_package_spec(package_spec: str) -> str:
        """把无协议的域名源转换为 https 形式，避免 git clone 失败。"""
        spec = (package_spec or "").strip()
        if not spec:
            return spec
        if "://" in spec:
            return spec
        # 形如 skills.volces.com 或 skills.sh/path
        if re.match(r"^[A-Za-z0-9.-]+\\.[A-Za-z]{2,}(/|$)", spec):
            return f"https://{spec}"
        return spec

    @staticmethod
    def _build_error(title: str, command: list[str], result: subprocess.CompletedProcess[str]) -> str:
        stdout = (result.stdout or "").strip()
        stderr = (result.stderr or "").strip()
        detail = stderr or stdout
        if not detail:
            detail = "未知错误"
        command_str = " ".join(command)
        return f"{title}: {detail} (cmd={command_str})"
