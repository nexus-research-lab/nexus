# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_repository.py
# @Date   ：2026/3/9 22:31
# @Author ：leemysw
# 2026/3/9 22:31   Create
# =====================================================

"""
Agent 数据仓库

[INPUT]: 依赖文件存储层和 schema/model_agent 的 AAgent
[OUTPUT]: 对外提供 AgentRepository（Agent CRUD）
[POS]: db 模块的 Agent 持久化层，被 agent_manager 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from pathlib import Path
from threading import Lock
from typing import Dict, List, Optional

from agent.infra.storage.config_store import ConfigStore
from agent.infra.storage.storage_bootstrap import FileStorageBootstrap
from agent.infra.storage.storage_paths import FileStoragePaths
from agent.schema.model_agent import AAgent, AgentOptions
from agent.utils.logger import logger


class AgentRepository:
    """Agent 文件数据仓库。"""

    def __init__(self) -> None:
        self._bootstrap = FileStorageBootstrap()
        self._paths = FileStoragePaths()
        self._lock = Lock()
        self._bootstrap.ensure_ready()

    def _load_records(self) -> List[Dict]:
        """读取 Agent 索引。"""
        self._bootstrap.ensure_ready()
        records = ConfigStore.read(self._paths.agents_index_path, [])
        return records if isinstance(records, list) else []

    def _write_records(self, records: List[Dict]) -> None:
        """写回 Agent 索引。"""
        ConfigStore.write(self._paths.agents_index_path, records)

    def _write_agent_snapshot(self, record: Dict) -> None:
        """将 Agent 快照同步到各自 workspace。"""
        workspace_path = Path(record["workspace_path"]).expanduser()
        workspace_path.mkdir(parents=True, exist_ok=True)
        self._paths.migrate_workspace_runtime_layout(workspace_path)
        ConfigStore.write(self._paths.get_agent_file_path(workspace_path), record)

    @staticmethod
    def _to_model(record: Dict) -> AAgent:
        """将字典记录转换为 AAgent。"""
        return AAgent(
            agent_id=record["agent_id"],
            name=record["name"],
            workspace_path=record["workspace_path"],
            options=AgentOptions(**(record.get("options") or {})),
            created_at=record.get("created_at") or datetime.now().isoformat(),
            status=record.get("status") or "active",
        )

    async def create_agent(
        self,
        agent_id: str,
        name: str,
        workspace_path: str,
        options: Optional[Dict] = None,
    ) -> Optional[str]:
        """创建 Agent，返回 agent_id。"""
        with self._lock:
            records = self._load_records()
            if any(record.get("agent_id") == agent_id for record in records):
                logger.warning(f"⚠️ Agent 已存在，跳过创建: {agent_id}")
                return None

            record = {
                "agent_id": agent_id,
                "name": name,
                "workspace_path": str(Path(workspace_path).expanduser()),
                "options": options or {},
                "created_at": datetime.now().isoformat(),
                "status": "active",
            }
            records.append(record)
            self._write_records(records)
            self._write_agent_snapshot(record)
            logger.info(f"✅ Agent 创建成功: {agent_id} ({name})")
            return agent_id

    async def get_agent(self, agent_id: str) -> Optional[AAgent]:
        """按 agent_id 获取 Agent。"""
        records = self._load_records()
        for record in records:
            if record.get("agent_id") == agent_id:
                return self._to_model(record)
        return None

    async def get_all_agents(self) -> List[AAgent]:
        """获取所有活跃 Agent。"""
        records = self._load_records()
        active_records = [record for record in records if record.get("status", "active") == "active"]
        active_records.sort(key=lambda item: str(item.get("created_at") or ""), reverse=True)
        return [self._to_model(record) for record in active_records]

    async def exists_active_agent_name(
        self,
        name: str,
        exclude_agent_id: Optional[str] = None,
    ) -> bool:
        """检查活跃 Agent 名称是否已存在。"""
        normalized = name.lower()
        for record in self._load_records():
            if record.get("status", "active") != "active":
                continue
            if exclude_agent_id and record.get("agent_id") == exclude_agent_id:
                continue
            if str(record.get("name", "")).lower() == normalized:
                return True
        return False

    async def update_agent(
        self,
        agent_id: str,
        name: Optional[str] = None,
        options: Optional[Dict] = None,
    ) -> bool:
        """更新 Agent。"""
        with self._lock:
            records = self._load_records()
            for record in records:
                if record.get("agent_id") != agent_id:
                    continue

                if name is not None:
                    record["name"] = name
                if options is not None:
                    merged_options = dict(record.get("options") or {})
                    merged_options.update(options)
                    record["options"] = merged_options

                self._write_records(records)
                self._write_agent_snapshot(record)
                logger.info(f"✅ Agent 更新成功: {agent_id}")
                return True

        return False

    async def update_agent_workspace_path(self, agent_id: str, workspace_path: str) -> bool:
        """更新 Agent 的工作空间路径。"""
        target_path = str(Path(workspace_path).expanduser())
        with self._lock:
            records = self._load_records()
            for record in records:
                if record.get("agent_id") != agent_id:
                    continue

                record["workspace_path"] = target_path
                self._write_records(records)
                self._write_agent_snapshot(record)
                logger.info(f"✅ Agent workspace_path 已更新: {agent_id} -> {target_path}")
                return True

        return False

    async def delete_agent(self, agent_id: str) -> bool:
        """软删除 Agent。"""
        with self._lock:
            records = self._load_records()
            for record in records:
                if record.get("agent_id") != agent_id:
                    continue

                record["status"] = "archived"
                self._write_records(records)
                self._write_agent_snapshot(record)
                logger.info(f"🗑️ Agent 已归档: {agent_id}")
                return True

        return False


agent_repository = AgentRepository()
