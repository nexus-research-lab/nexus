# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_workspace.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Agent workspace 注册与同步。"""

import shutil
from pathlib import Path
from typing import Dict, Optional

from agent.schema.model_agent import AAgent
from agent.service.workspace.agent_workspace import AgentWorkspace
from agent.service.workspace.workspace_paths import get_workspace_base_path
from agent.storage.agent_repository import agent_repository
from agent.utils.logger import logger


class AgentWorkspaceRegistry:
    """负责 workspace 缓存、路径迁移与删除校验。"""

    def __init__(self):
        self._workspaces: Dict[str, AgentWorkspace] = {}

    async def get_agent_workspace(self, agent: AAgent) -> AgentWorkspace:
        """按 Agent 当前配置获取 workspace 实例。"""
        synced_workspace = await self.sync_workspace_path(agent)
        workspace = self.get_workspace(agent.agent_id, synced_workspace)
        workspace.ensure_initialized(agent_name=agent.name)
        return workspace

    def get_workspace(self, agent_id: str, workspace_path: Optional[str] = None) -> AgentWorkspace:
        """惰性创建 workspace 实例。"""
        desired_path = (
            Path(workspace_path).expanduser()
            if workspace_path
            else get_workspace_base_path() / agent_id
        )

        cached = self._workspaces.get(agent_id)
        if cached and cached.path != desired_path:
            logger.warning(f"⚠️ workspace 缓存路径不一致，重建实例: {agent_id}, {desired_path}")
            cached = None

        if not cached:
            cached = AgentWorkspace(agent_id, desired_path)
            self._workspaces[agent_id] = cached

        return cached

    async def sync_workspace_path(self, agent: AAgent) -> str:
        """同步 Agent 的工作区路径到名称目录规则。"""
        from agent.service.agent.agent_name_policy import AgentNamePolicy

        expected_path = AgentNamePolicy.resolve_workspace_path(agent.name)
        current_path = Path(agent.workspace_path).expanduser() if agent.workspace_path else None
        target_path = expected_path

        if current_path and current_path != expected_path:
            if current_path.exists() and not expected_path.exists():
                expected_path.parent.mkdir(parents=True, exist_ok=True)
                try:
                    shutil.move(str(current_path), str(expected_path))
                    logger.info(f"✅ 工作区目录迁移完成: {current_path} -> {expected_path}")
                except Exception as exc:
                    logger.warning(f"⚠️ 工作区目录迁移失败，保留旧目录继续运行: {current_path}, error={exc}")
                    target_path = current_path
            elif current_path.exists() and expected_path.exists():
                logger.warning(f"⚠️ 目标工作区已存在，保留当前路径避免覆盖: {current_path} (expected={expected_path})")
                target_path = current_path

        target_path_str = str(target_path)
        if agent.workspace_path != target_path_str:
            await agent_repository.update_agent_workspace_path(agent.agent_id, target_path_str)
            agent.workspace_path = target_path_str
            self._workspaces.pop(agent.agent_id, None)

        return target_path_str

    def delete_workspace(self, agent: AAgent) -> None:
        """安全删除 Agent 工作区目录。"""
        workspace_path = Path(agent.workspace_path).expanduser()
        self._workspaces.pop(agent.agent_id, None)
        if not workspace_path.exists():
            logger.info(f"ℹ️ 工作区目录不存在，跳过删除: agent={agent.agent_id}, path={workspace_path}")
            return
        if not self._can_delete_workspace(agent, workspace_path):
            logger.warning(f"⚠️ 工作区归属校验失败，跳过物理删除: agent={agent.agent_id}, path={workspace_path}")
            return

        shutil.rmtree(workspace_path, ignore_errors=False)
        logger.info(f"🗑️ 工作区已删除: agent={agent.agent_id}, path={workspace_path}")

    def _can_delete_workspace(self, agent: AAgent, workspace_path: Path) -> bool:
        """校验工作区目录是否归属于当前 Agent。"""
        if not workspace_path.is_dir():
            return False
        if not self._is_path_under_workspace_base(workspace_path):
            return False
        return workspace_path == Path(agent.workspace_path).expanduser()

    @staticmethod
    def _is_path_under_workspace_base(workspace_path: Path) -> bool:
        """判断目录是否位于系统托管的 workspace 根目录下。"""
        try:
            workspace_path.resolve().relative_to(get_workspace_base_path().resolve())
            return True
        except ValueError:
            return False
