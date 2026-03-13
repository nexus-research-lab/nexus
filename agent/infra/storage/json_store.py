# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：json_store.py
# @Date   ：2026/3/12 20:39
# @Author ：leemysw
# 2026/3/12 20:39   Create
# =====================================================

"""
JSON/JSONL 文件读写工具。

[INPUT]: 依赖本地文件系统
[OUTPUT]: 对外提供 JSON/JSONL 原子读写能力
[POS]: infra/storage 的底层序列化工具层，被 repository/bootstrap 复用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import json
from pathlib import Path
from typing import Any, Dict, List

from agent.utils.logger import logger


class JsonFileStore:
    """JSON 文件读写工具。"""

    @staticmethod
    def read_json(path: Path, default: Any) -> Any:
        """读取 JSON 文件，不存在时返回默认值。"""
        if not path.exists():
            return default

        try:
            return json.loads(path.read_text(encoding="utf-8"))
        except Exception as exc:
            logger.warning(f"⚠️ 读取 JSON 失败，使用默认值: path={path}, error={exc}")
            return default

    @staticmethod
    def write_json(path: Path, payload: Any) -> None:
        """原子写入 JSON 文件。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = path.with_suffix(path.suffix + ".tmp")
        temp_path.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        temp_path.replace(path)

    @staticmethod
    def append_jsonl(path: Path, payload: Dict[str, Any]) -> None:
        """向 JSONL 文件追加一行。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("a", encoding="utf-8") as file:
            file.write(json.dumps(payload, ensure_ascii=False) + "\n")

    @staticmethod
    def write_jsonl(path: Path, rows: List[Dict[str, Any]]) -> None:
        """覆写 JSONL 文件。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = path.with_suffix(path.suffix + ".tmp")
        content = "\n".join(json.dumps(row, ensure_ascii=False) for row in rows)
        if content:
            content += "\n"
        temp_path.write_text(content, encoding="utf-8")
        temp_path.replace(path)

    @staticmethod
    def read_jsonl(path: Path) -> List[Dict[str, Any]]:
        """读取 JSONL 文件。"""
        if not path.exists():
            return []

        rows: List[Dict[str, Any]] = []
        for raw_line in path.read_text(encoding="utf-8").splitlines():
            line = raw_line.strip()
            if not line:
                continue
            try:
                parsed = json.loads(line)
            except Exception as exc:
                logger.warning(f"⚠️ 跳过损坏的 JSONL 行: path={path}, error={exc}")
                continue
            if isinstance(parsed, dict):
                rows.append(parsed)
        return rows
