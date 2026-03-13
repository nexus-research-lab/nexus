# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_manager.py
# @Date   ：2026/3/4 15:09
# @Author ：leemysw
# 2026/3/4 15:09   Create
# =====================================================

"""
Agent 生命周期管理器

[INPUT]: 依赖 db/agent_repository，依赖 agent/workspace 的 AgentWorkspace
[OUTPUT]: 对外提供 AgentManager（Agent 创建/查询/配置构建）
[POS]: service 层的 Agent 管理中心，被 ChatService 和 API 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import re
import shutil
import unicodedata
from pathlib import Path
from typing import Dict, List, Optional

from agent.infra.storage.agent_repository import agent_repository
from agent.infra.workspace.initializer import AgentWorkspace, get_workspace_base_path
from agent.schema.model_agent import AAgent, AgentOptions, ValidateAgentNameResponse
from agent.utils.logger import logger


class AgentManager:
    """Agent 生命周期管理"""

    NAME_MIN_LEN = 2
    NAME_MAX_LEN = 40
    NAME_ALLOWED_PATTERN = re.compile(r"^[\u4e00-\u9fffA-Za-z0-9 _-]+$")

    def __init__(self):
        self._workspaces: Dict[str, AgentWorkspace] = {}

    @classmethod
    def _normalize_agent_name(cls, name: str) -> str:
        """标准化 Agent 名称（去首尾空格，压缩中间连续空格）。"""
        return " ".join((name or "").strip().split())

    @classmethod
    def _build_workspace_dir_name(cls, agent_name: str) -> str:
        """从 Agent 名称生成安全的目录名。"""
        normalized = unicodedata.normalize("NFKC", cls._normalize_agent_name(agent_name))
        normalized = normalized.replace(" ", "_")
        safe_name = re.sub(r"[^A-Za-z0-9\u4e00-\u9fff_-]", "_", normalized)
        safe_name = re.sub(r"_+", "_", safe_name).strip("._-")
        return safe_name or "agent"

    @classmethod
    def _resolve_workspace_path(cls, agent_name: str) -> Path:
        """统一计算 Agent 工作空间路径。"""
        return get_workspace_base_path() / cls._build_workspace_dir_name(agent_name)

    def _validate_name_format(self, normalized_name: str) -> Optional[str]:
        """校验名称格式并返回失败原因。"""
        if not normalized_name:
            return "名称不能为空"

        if len(normalized_name) < self.NAME_MIN_LEN:
            return f"名称至少 {self.NAME_MIN_LEN} 个字符"

        if len(normalized_name) > self.NAME_MAX_LEN:
            return f"名称不能超过 {self.NAME_MAX_LEN} 个字符"

        if not self.NAME_ALLOWED_PATTERN.fullmatch(normalized_name):
            return "仅支持中文、英文、数字、空格、下划线和连字符"

        return None

    async def _resolve_name_conflict_reason(
        self,
        normalized_name: str,
        expected_workspace: Path,
        exclude_agent_id: Optional[str] = None,
    ) -> Optional[str]:
        """检查名称与 workspace 是否冲突。"""
        name_occupied = await agent_repository.exists_active_agent_name(
            normalized_name,
            exclude_agent_id=exclude_agent_id,
        )
        if name_occupied:
            return "名称已存在，请更换一个名称"

        if not expected_workspace.exists():
            return None

        if not exclude_agent_id:
            return "同名工作区目录已存在，请更换名称"

        current_agent = await agent_repository.get_agent(exclude_agent_id)
        current_path = Path(current_agent.workspace_path).expanduser() if current_agent else None
        if not current_path or current_path != expected_workspace:
            return "同名工作区目录已存在，请更换名称"

        return None

    async def validate_agent_name(
        self,
        name: str,
        exclude_agent_id: Optional[str] = None,
    ) -> ValidateAgentNameResponse:
        """校验 Agent 名称规则、重复性和目标 workspace 冲突。"""
        normalized_name = self._normalize_agent_name(name)
        invalid_reason = self._validate_name_format(normalized_name)
        if invalid_reason:
            return ValidateAgentNameResponse.invalid(name, normalized_name, invalid_reason)

        expected_workspace = self._resolve_workspace_path(normalized_name)
        expected_workspace_str = str(expected_workspace)
        conflict_reason = await self._resolve_name_conflict_reason(
            normalized_name=normalized_name,
            expected_workspace=expected_workspace,
            exclude_agent_id=exclude_agent_id,
        )
        if conflict_reason:
            return ValidateAgentNameResponse.unavailable(
                name=name,
                normalized_name=normalized_name,
                workspace_path=expected_workspace_str,
                reason=conflict_reason,
            )

        return ValidateAgentNameResponse.available(
            name=name,
            normalized_name=normalized_name,
            workspace_path=expected_workspace_str,
        )

    async def _sync_workspace_path(self, agent: AAgent) -> str:
        """同步 Agent 的工作区路径到“名称目录”规则。"""
        expected_path = self._resolve_workspace_path(agent.name)
        current_path = Path(agent.workspace_path).expanduser() if agent.workspace_path else None
        target_path = expected_path

        if current_path and current_path != expected_path:
            if current_path.exists() and not expected_path.exists():
                expected_path.parent.mkdir(parents=True, exist_ok=True)
                try:
                    shutil.move(str(current_path), str(expected_path))
                    logger.info(
                        "✅ 工作区目录迁移完成: "
                        f"{current_path} -> {expected_path}"
                    )
                except Exception as exc:
                    logger.warning(
                        "⚠️ 工作区目录迁移失败，保留旧目录继续运行: "
                        f"{current_path}, error={exc}"
                    )
                    target_path = current_path
            elif current_path.exists() and expected_path.exists():
                logger.warning(
                    "⚠️ 目标工作区已存在，保留当前路径避免覆盖: "
                    f"{current_path} (expected={expected_path})"
                )
                target_path = current_path

        target_path_str = str(target_path)
        if agent.workspace_path != target_path_str:
            await agent_repository.update_agent_workspace_path(agent.agent_id, target_path_str)
            agent.workspace_path = target_path_str
            self._workspaces.pop(agent.agent_id, None)

        return target_path_str

    # =====================================================
    # Agent CRUD
    # =====================================================

    async def create_agent(
        self,
        name: str,
        workspace_path: Optional[str] = None,
        options: Optional[AgentOptions] = None,
    ) -> Optional[AAgent]:
        """创建 Agent，自动初始化 workspace 目录"""
        validation = await self.validate_agent_name(name)
        if not validation.is_valid or not validation.is_available:
            raise ValueError(validation.reason or "Agent 名称校验失败")

        normalized_name = validation.normalized_name
        resolved_path_str = validation.workspace_path
        if not resolved_path_str:
            raise ValueError("无法生成工作区路径")

        from uuid import uuid4
        agent_id = uuid4().hex[:12]

        # workspace_path 由系统托管，前端传值仅保留兼容
        if workspace_path and str(Path(workspace_path).expanduser()) != resolved_path_str:
            logger.warning(
                "⚠️ workspace_path 参数已忽略，统一使用: "
                f"{resolved_path_str}"
            )

        options_dict = options.model_dump(exclude_none=True) if options else None

        created_id = await agent_repository.create_agent(
            agent_id=agent_id,
            name=normalized_name,
            workspace_path=resolved_path_str,
            options=options_dict,
        )
        if not created_id:
            return None

        # 初始化 workspace 目录
        workspace = self._get_or_create_workspace(agent_id, resolved_path_str)
        workspace.ensure_initialized(agent_name=normalized_name)

        agent = await agent_repository.get_agent(agent_id)
        logger.info(f"✅ Agent 创建完成: {agent_id} ({normalized_name}), workspace={resolved_path_str}")
        return agent

    async def get_agent(self, agent_id: str) -> Optional[AAgent]:
        """获取 Agent"""
        return await agent_repository.get_agent(agent_id)

    async def get_all_agents(self) -> List[AAgent]:
        """获取所有活跃 Agent"""
        return await agent_repository.get_all_agents()

    async def update_agent(
        self,
        agent_id: str,
        name: Optional[str] = None,
        options: Optional[AgentOptions] = None,
    ) -> bool:
        """更新 Agent 配置"""
        existing = await agent_repository.get_agent(agent_id)
        if not existing:
            return False

        normalized_name = None
        if name is not None:
            validation = await self.validate_agent_name(name, exclude_agent_id=agent_id)
            if not validation.is_valid or not validation.is_available:
                raise ValueError(validation.reason or "Agent 名称校验失败")
            normalized_name = validation.normalized_name

        options_dict = options.model_dump(exclude_none=True) if options else None
        updated = await agent_repository.update_agent(
            agent_id,
            name=normalized_name,
            options=options_dict,
        )
        if not updated:
            return False

        latest = await agent_repository.get_agent(agent_id)
        if not latest:
            return False

        synced_path = await self._sync_workspace_path(latest)
        workspace = self._get_or_create_workspace(agent_id, synced_path)
        workspace.ensure_initialized(agent_name=latest.name)
        return True

    async def delete_agent(self, agent_id: str) -> bool:
        """删除 Agent（软删除）"""
        self._workspaces.pop(agent_id, None)
        return await agent_repository.delete_agent(agent_id)

    # =====================================================
    # Workspace
    # =====================================================

    def get_workspace(self, agent_id: str) -> AgentWorkspace:
        """获取 Agent 的 workspace 实例"""
        return self._get_or_create_workspace(agent_id)

    async def get_agent_workspace(self, agent_id: str) -> AgentWorkspace:
        """按 Agent 当前配置获取 workspace 实例。"""
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise ValueError(f"Agent not found: {agent_id}")

        synced_workspace = await self._sync_workspace_path(agent)
        workspace = self._get_or_create_workspace(agent_id, synced_workspace)
        workspace.ensure_initialized(agent_name=agent.name)
        return workspace

    def _get_or_create_workspace(self, agent_id: str, workspace_path: Optional[str] = None) -> AgentWorkspace:
        """惰性创建 workspace 实例"""
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

        return self._workspaces[agent_id]

    # =====================================================
    # SDK 配置构建
    # =====================================================

    # SDK 不支持的配置字段（业务层专用）
    _NON_SDK_FIELDS = {"skills_enabled"}

    async def build_sdk_options(self, agent_id: str) -> dict:
        """从 Agent 配置 + Workspace 构建 ClaudeAgentOptions 参数

        合并顺序: workspace options (cwd + system_prompt) → agent options (model + tools + ...)
        每次调用重新读取 workspace 文件，修改后立即生效。
        """
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise ValueError(f"Agent not found: {agent_id}")

        synced_workspace = await self._sync_workspace_path(agent)

        # Workspace 层: cwd + system_prompt
        workspace = self._get_or_create_workspace(agent_id, synced_workspace)
        workspace.ensure_initialized(agent_name=agent.name)
        sdk_options = workspace.build_sdk_options()

        # Agent 层: model + tools + permissions + ...（过滤掉非 SDK 字段）
        agent_opts = agent.options.model_dump(exclude_none=True)
        for field in self._NON_SDK_FIELDS:
            agent_opts.pop(field, None)
        sdk_options.update(agent_opts)

        return sdk_options


# 全局实例
agent_manager = AgentManager()
