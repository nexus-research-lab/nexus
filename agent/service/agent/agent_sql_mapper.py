# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_sql_mapper.py
# @Date   ：2026/03/26 00:09
# @Author ：leemysw
# 2026/03/26 00:09   Create
# =====================================================

"""Agent SQL 映射器。"""

from __future__ import annotations

import hashlib
import json
from datetime import datetime
from pathlib import Path
from typing import Dict, Optional

from agent.schema.model_agent import AAgent, AgentOptions
from agent.schema.model_agent_persistence import (
    AgentAggregate,
    AgentRecord,
    CreateAgentAggregate,
    ProfileRecord,
    RuntimeRecord,
)


class AgentSqlMapper:
    """负责 Agent 应用模型与 SQL 聚合模型转换。"""

    @classmethod
    def build_create_payload(
        cls,
        agent_id: str,
        name: str,
        workspace_path: str,
        options: Optional[Dict],
        status: str,
        created_at: Optional[datetime] = None,
    ) -> CreateAgentAggregate:
        """构造创建聚合载荷。"""
        from agent.service.agent.agent_name_policy import AgentNamePolicy

        normalized_options = options or {}
        # 从 options 中提取身份标识字段（avatar / description / vibe_tags）
        avatar = normalized_options.pop("avatar", None)
        description = normalized_options.pop("description", None) or ""
        vibe_tags = normalized_options.pop("vibe_tags", None)
        return CreateAgentAggregate(
            agent=AgentRecord(
                id=agent_id,
                slug=AgentNamePolicy.build_workspace_dir_name(name),
                name=name,
                description=description,
                definition="",
                status=status,
                workspace_path=str(Path(workspace_path).expanduser()),
                avatar=avatar,
                vibe_tags=vibe_tags,
                created_at=created_at,
            ),
            profile=ProfileRecord(
                id=cls.build_stable_id("profile", agent_id),
                agent_id=agent_id,
                display_name=name,
                headline="",
                profile_markdown="",
                created_at=created_at,
            ),
            runtime=RuntimeRecord(
                id=cls.build_stable_id("runtime", agent_id),
                agent_id=agent_id,
                created_at=created_at,
                runtime_version=1,
                **cls.build_runtime_fields(normalized_options),
            ),
        )

    @staticmethod
    def build_stable_id(prefix: str, raw_value: str) -> str:
        """生成稳定派生 ID。"""
        digest = hashlib.sha1(raw_value.encode("utf-8")).hexdigest()[:20]
        return f"{prefix}_{digest}"

    @staticmethod
    def parse_datetime(value: object) -> Optional[datetime]:
        """解析旧记录中的时间。"""
        if not value:
            return None
        if isinstance(value, datetime):
            return value
        if isinstance(value, str):
            try:
                return datetime.fromisoformat(value)
            except ValueError:
                return None
        return None

    @classmethod
    def merge_options(
        cls,
        aggregate: AgentAggregate,
        options: Optional[Dict],
    ) -> Dict:
        """将增量更新与现有运行参数合并。"""
        merged = cls.runtime_record_to_options(aggregate.runtime).model_dump(exclude_none=True)
        if options:
            merged.update(options)
        return merged

    @classmethod
    def to_model(cls, aggregate: AgentAggregate) -> AAgent:
        """将聚合转换为应用层 Agent。"""
        return AAgent(
            agent_id=aggregate.agent.id,
            name=aggregate.agent.name,
            workspace_path=aggregate.agent.workspace_path,
            options=cls.runtime_record_to_options(aggregate.runtime),
            created_at=aggregate.agent.created_at or datetime.now(),
            status=aggregate.agent.status,
            # 身份标识字段
            avatar=aggregate.agent.avatar,
            description=aggregate.agent.description or None,
            vibe_tags=aggregate.agent.vibe_tags,
        )

    @classmethod
    def runtime_record_to_options(cls, runtime: RuntimeRecord) -> AgentOptions:
        """将 RuntimeRecord 还原为 AgentOptions。"""
        return AgentOptions(
            model=runtime.model,
            permission_mode=runtime.permission_mode,
            allowed_tools=cls.parse_json_list(runtime.allowed_tools_json),
            disallowed_tools=cls.parse_json_list(runtime.disallowed_tools_json),
            max_turns=runtime.max_turns,
            max_thinking_tokens=runtime.max_thinking_tokens,
            mcp_servers=cls.parse_json_dict(runtime.mcp_servers_json),
            skills_enabled=runtime.skills_enabled,
            installed_skills=cls.parse_json_list(runtime.installed_skills_json),
            setting_sources=cls.parse_json_list(runtime.setting_sources_json),
        )

    @classmethod
    def build_runtime_fields(cls, options: Dict) -> Dict:
        """构造运行时字段。"""
        return {
            "model": options.get("model"),
            "permission_mode": options.get("permission_mode"),
            "allowed_tools_json": cls.to_json(options.get("allowed_tools") or []),
            "disallowed_tools_json": cls.to_json(options.get("disallowed_tools") or []),
            "mcp_servers_json": cls.to_json(options.get("mcp_servers") or {}),
            "max_turns": options.get("max_turns"),
            "max_thinking_tokens": options.get("max_thinking_tokens"),
            "skills_enabled": bool(options.get("skills_enabled", False)),
            "installed_skills_json": cls.to_json(options.get("installed_skills") or []),
            "setting_sources_json": cls.to_json(options.get("setting_sources") or []),
        }

    @staticmethod
    def to_json(value: object) -> str:
        """序列化为 JSON。"""
        return json.dumps(value, ensure_ascii=False)

    @staticmethod
    def parse_json_list(value: str) -> Optional[list]:
        """解析 JSON 数组。"""
        try:
            parsed = json.loads(value or "[]")
        except json.JSONDecodeError:
            return None
        return parsed if isinstance(parsed, list) else None

    @staticmethod
    def parse_json_dict(value: str) -> Optional[dict]:
        """解析 JSON 对象。"""
        try:
            parsed = json.loads(value or "{}")
        except json.JSONDecodeError:
            return None
        return parsed if isinstance(parsed, dict) else None
