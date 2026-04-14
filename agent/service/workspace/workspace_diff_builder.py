# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_diff_builder.py
# @Date   ：2026/04/14 21:13
# @Author ：leemysw
# 2026/04/14 21:13   Create
# =====================================================

"""Workspace diff 摘要构建器。"""

from difflib import SequenceMatcher
from typing import Optional

from agent.schema.model_workspace import WorkspaceDiffStats


class WorkspaceDiffBuilder:
    """负责构建 workspace 文本 diff 摘要。"""

    MAX_DIFF_BYTES = 64 * 1024

    @classmethod
    def build(cls, before_content: str, after_content: str) -> Optional[WorkspaceDiffStats]:
        """计算基础 diff 摘要，大文件直接跳过。"""
        if len(before_content) > cls.MAX_DIFF_BYTES or len(after_content) > cls.MAX_DIFF_BYTES:
            return None

        before_lines = before_content.splitlines()
        after_lines = after_content.splitlines()
        matcher = SequenceMatcher(a=before_lines, b=after_lines)
        additions = 0
        deletions = 0

        for tag, i1, i2, j1, j2 in matcher.get_opcodes():
            if tag == "insert":
                additions += j2 - j1
            elif tag == "delete":
                deletions += i2 - i1
            elif tag == "replace":
                deletions += i2 - i1
                additions += j2 - j1

        return WorkspaceDiffStats(
            additions=additions,
            deletions=deletions,
            changed_lines=additions + deletions,
        )
