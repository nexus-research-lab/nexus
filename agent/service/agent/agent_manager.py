# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_manager.py
# @Date   ：2026/3/4 15:09
# @Author ：leemysw
# 2026/3/4 15:09   Create
# =====================================================

"""Agent 生命周期管理器。"""

from typing import List, Optional

from agent.schema.model_agent import AAgent, AgentOptions, ValidateAgentNameResponse
from agent.service.agent.agent_name_policy import AgentNamePolicy
from agent.service.agent.agent_prompt_builder import AgentPromptBuilder
from agent.service.agent.agent_repository import agent_repository
from agent.service.agent.main_agent_scheduled_task_sdk_server import (
    SERVER_NAME as BUILTIN_AUTOMATION_SERVER_NAME,
    get_main_agent_scheduled_task_sdk_server,
)
from agent.service.agent.agent_workspace import AgentWorkspaceRegistry
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.settings.provider_config_service import provider_config_service
from agent.service.session.session_manager import session_manager
from agent.utils.logger import logger


class AgentManager:
    """Agent 生命周期管理"""

    def __init__(self):
        self._name_policy = AgentNamePolicy()
        self._prompt_builder = AgentPromptBuilder()
        self._workspace_registry = AgentWorkspaceRegistry()

    async def validate_agent_name(
            self,
            name: str,
            exclude_agent_id: Optional[str] = None,
    ) -> ValidateAgentNameResponse:
        """校验 Agent 名称规则、重复性和目标 workspace 冲突。"""
        return await self._name_policy.validate_name(name, exclude_agent_id)

    # =====================================================
    # Agent CRUD
    # =====================================================

    async def create_agent(
            self,
            name: str,
            options: Optional[AgentOptions] = None,
            avatar: Optional[str] = None,
            description: Optional[str] = None,
            vibe_tags: Optional[list[str]] = None,
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

        options_dict = options.model_dump(exclude_none=True) if options else None

        created_id = await agent_repository.create_agent(
            agent_id=agent_id,
            name=normalized_name,
            workspace_path=resolved_path_str,
            options=options_dict,
            avatar=avatar,
            description=description,
            vibe_tags=vibe_tags,
        )
        if not created_id:
            return None

        # 初始化 workspace 目录
        workspace = self._workspace_registry.get_workspace(agent_id, resolved_path_str)
        workspace.ensure_initialized(agent_name=normalized_name)

        agent = await agent_repository.get_agent(agent_id)
        logger.info(f"✅ Agent 创建完成: {agent_id} ({normalized_name}), workspace={resolved_path_str}")
        return agent

    @staticmethod
    async def get_agent(agent_id: str) -> Optional[AAgent]:
        """获取 Agent"""
        return await agent_repository.get_agent(agent_id)

    @staticmethod
    async def get_all_agents() -> List[AAgent]:
        """获取所有活跃 Agent"""
        return await agent_repository.get_all_agents()

    async def update_agent(
            self,
            agent_id: str,
            name: Optional[str] = None,
            options: Optional[AgentOptions] = None,
            avatar: Optional[str] = None,
            description: Optional[str] = None,
            vibe_tags: Optional[list[str]] = None,
    ) -> bool:
        """更新 Agent 配置"""
        existing = await agent_repository.get_agent(agent_id)
        if not existing:
            return False
        if name is not None and name != existing.name:
            MainAgentProfile.ensure_not_main_agent(agent_id, "名称不可修改")

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
            avatar=avatar,
            description=description,
            vibe_tags=vibe_tags,
        )
        if not updated:
            return False

        latest = await agent_repository.get_agent(agent_id)
        if not latest:
            return False

        synced_path = await self._workspace_registry.sync_workspace_path(latest)
        workspace = self._workspace_registry.get_workspace(agent_id, synced_path)
        workspace.ensure_initialized(agent_name=latest.name)
        return True

    async def delete_agent(self, agent_id: str) -> bool:
        """删除 Agent，并同步清理运行态与工作区。"""
        MainAgentProfile.ensure_not_main_agent(agent_id, "不可删除")

        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            return False

        await session_manager.remove_agent_sessions(agent_id)
        self._workspace_registry.delete_workspace(agent)
        return await agent_repository.delete_agent(agent_id)

    # =====================================================
    # Workspace
    # =====================================================

    async def get_agent_workspace(self, agent_id: str):
        """按 Agent 当前配置获取 workspace 实例。"""
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise ValueError(f"Agent not found: {agent_id}")
        return await self._workspace_registry.get_agent_workspace(agent)

    # =====================================================
    # SDK 配置构建
    # =====================================================

    async def build_sdk_options(self, agent_id: str) -> dict:
        """从 Agent 配置 + Workspace 构建 ClaudeAgentOptions 参数

        合并顺序: workspace options (cwd + system_prompt) → agent options (provider + tools + ...)
        每次调用重新读取 workspace 文件，修改后立即生效。
        """
        agent = await agent_repository.get_agent(agent_id)
        if not agent:
            raise ValueError(f"Agent not found: {agent_id}")

        workspace = await self._workspace_registry.get_agent_workspace(agent)
        base_options = {"include_partial_messages": True, "cwd": str(workspace.path)}
        system_prompt = self._prompt_builder.build(workspace, agent.agent_id)
        if system_prompt:
            base_options["system_prompt"] = system_prompt

        agent_options = agent.options.model_dump(exclude_none=True)
        # 中文注释：仅在用户未配置工具白名单时补默认值；
        # 显式传入空数组表示不预授权任何工具，不能被默认值覆盖。
        if "allowed_tools" not in agent_options:
            default_tools = (
                MainAgentProfile.ALLOWED_TOOLS
                if MainAgentProfile.is_main_agent(agent_id)
                else MainAgentProfile.REGULAR_AGENT_ALLOWED_TOOLS
            )
            agent_options["allowed_tools"] = default_tools.copy()

        provider = agent_options.pop("provider", None)
        runtime_config = await provider_config_service.resolve_runtime_config(provider)
        # 中文注释：Provider 配置的 base_url/model 以用户输入为准。
        # 这里仅把同一份模型值投影到 Claude Code 会读取的多个环境变量，
        # 不对用户输入做语义改写。
        runtime_env = {
            "ANTHROPIC_AUTH_TOKEN": runtime_config.auth_token,
            "ANTHROPIC_BASE_URL": runtime_config.base_url,
            "ANTHROPIC_MODEL": runtime_config.model,
        }
        if "kimi" in runtime_config.model.lower():
            runtime_env["ENABLE_TOOL_SEARCH"]="false"

        base_options.update({
            "model": runtime_config.model,
            "env": runtime_env,
        })
        connector_mcp_servers = await self._load_connector_mcp_servers()
        builtin_mcp_servers = self._load_builtin_mcp_servers()
        if connector_mcp_servers:
            existing_mcp_servers = agent_options.get("mcp_servers") or {}
            agent_options["mcp_servers"] = {
                **builtin_mcp_servers,
                **connector_mcp_servers,
                **existing_mcp_servers,
            }
        elif builtin_mcp_servers:
            existing_mcp_servers = agent_options.get("mcp_servers") or {}
            agent_options["mcp_servers"] = {
                **builtin_mcp_servers,
                **existing_mcp_servers,
            }
        base_options.update(agent_options)
        return base_options

    async def _load_connector_mcp_servers(self) -> dict:
        """加载全局已连接 connector 对应的 MCP 配置。"""
        from agent.service.capability.connectors.connector_service import connector_service

        return await connector_service.build_runtime_mcp_servers()

    @staticmethod
    def _load_builtin_mcp_servers() -> dict:
        """为所有 agent 注入内置 orchestration tools。"""
        return {
            BUILTIN_AUTOMATION_SERVER_NAME: get_main_agent_scheduled_task_sdk_server(),
        }


# 全局实例
agent_manager = AgentManager()
