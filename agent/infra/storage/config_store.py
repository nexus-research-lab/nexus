# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：config_store.py
# @Date   ：2026/3/13 14:45
# @Author ：leemysw
# 2026/3/13 14:45   Create
# =====================================================

"""Agent 配置存储组件。"""

from pathlib import Path
from typing import Any

from agent.infra.storage.json_store import JsonFileStore


class ConfigStore:
    """负责 JSON 配置的读取与写入。"""

    @staticmethod
    def read(path: Path, default: Any) -> Any:
        """读取配置文件。"""
        return JsonFileStore.read_json(path, default)

    @staticmethod
    def write(path: Path, payload: Any) -> None:
        """写入配置文件。"""
        JsonFileStore.write_json(path, payload)
