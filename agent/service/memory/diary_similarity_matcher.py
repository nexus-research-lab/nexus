# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：diary_similarity_matcher.py
# @Date   ：2026/04/04 13:16
# @Author ：leemysw
# 2026/04/04 13:16   Create
# =====================================================

"""日记相似匹配器。"""

from __future__ import annotations

import re
from difflib import SequenceMatcher

from agent.service.memory.diary_entry import DiaryEntry


class DiarySimilarityMatcher:
    """负责识别相似学习条目。"""

    def find_related(self, target_entry: DiaryEntry, candidates: list[DiaryEntry], limit: int = 5) -> list[DiaryEntry]:
        """查找相似历史。"""
        scored_entries: list[tuple[float, DiaryEntry]] = []
        for candidate in candidates:
            score = self._score(target_entry, candidate)
            if score < 0.5:
                continue
            scored_entries.append((score, candidate))

        scored_entries.sort(key=lambda item: item[0], reverse=True)
        return [entry for _, entry in scored_entries[:limit]]

    def _score(self, target_entry: DiaryEntry, candidate: DiaryEntry) -> float:
        """计算两条日记的相似度。"""
        if target_entry.kind != candidate.kind:
            return 0.0
        if target_entry.kind == "LRN" and target_entry.category != candidate.category:
            return 0.0

        target_text = self._normalize_text(target_entry)
        candidate_text = self._normalize_text(candidate)
        ratio = SequenceMatcher(None, target_text, candidate_text).ratio()
        overlap = self._token_overlap(target_text, candidate_text)
        return max(ratio, overlap)

    @staticmethod
    def _normalize_text(entry: DiaryEntry) -> str:
        """提取用于比对的核心文本。"""
        chunks = [entry.title]
        for key in ("详情", "行动", "错误", "上下文", "需求", "反思", "经验"):
            value = entry.fields.get(key, "").strip()
            if value:
                chunks.append(value)
        return re.sub(r"\s+", " ", " ".join(chunks)).strip().lower()

    @staticmethod
    def _token_overlap(left: str, right: str) -> float:
        """计算词集合重叠度。"""
        left_tokens = DiarySimilarityMatcher._tokenize(left)
        right_tokens = DiarySimilarityMatcher._tokenize(right)
        if not left_tokens or not right_tokens:
            return 0.0
        common = len(left_tokens & right_tokens)
        return common / max(1, min(len(left_tokens), len(right_tokens)))

    @staticmethod
    def _tokenize(text: str) -> set[str]:
        """切分英文词和中文单字。"""
        tokens: set[str] = set()
        ascii_tokens = re.findall(r"[0-9a-zA-Z_]+", text)
        tokens.update(ascii_tokens)
        tokens.update(char for char in text if "\u4e00" <= char <= "\u9fff")
        return tokens
