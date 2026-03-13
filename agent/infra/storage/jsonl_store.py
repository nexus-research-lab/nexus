# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：jsonl_store.py
# @Date   ：2026/3/13 14:45
# @Author ：leemysw
# 2026/3/13 14:45   Create
# =====================================================

"""JSONL 存储组件。"""

from pathlib import Path
from typing import Any, Dict, List

from agent.infra.storage.json_store import JsonFileStore


class JsonlStore:
    """负责 JSONL 文件的追加、覆盖与读取。"""

    @staticmethod
    def append(path: Path, payload: Dict[str, Any]) -> None:
        """向 JSONL 文件追加一行。"""
        JsonFileStore.append_jsonl(path, payload)

    @staticmethod
    def write(path: Path, rows: List[Dict[str, Any]]) -> None:
        """覆写整个 JSONL 文件。"""
        JsonFileStore.write_jsonl(path, rows)

    @staticmethod
    def read(path: Path) -> List[Dict[str, Any]]:
        """读取 JSONL 文件。"""
        return JsonFileStore.read_jsonl(path)
