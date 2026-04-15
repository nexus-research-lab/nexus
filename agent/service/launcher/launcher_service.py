#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：launcher_service.py
# @Date   ：2026/3/30 00:00
# @Author ：leemysw
# 2026/3/30 00:00   Create
# =====================================================

"""Launcher 服务 — 处理协作入口的查询和建议。"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import TYPE_CHECKING

from agent.infra.database.get_db import get_db
from agent.schema.model_agent_persistence import AgentAggregate
from agent.schema.model_chat_persistence import RoomAggregate
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.repository.repository_service import persistence_service
from agent.utils.logger import logger

if TYPE_CHECKING:
    from agent.service.channels.ws.websocket_sender import WebSocketSender


@dataclass
class LauncherAction:
    """Launcher 动作结果。"""

    action_type: str  # "open_agent_dm", "open_room"
    target_id: str
    initial_message: str | None = None


class LauncherService:
    """Launcher 协作启动服务。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")
        self._websocket_sender: WebSocketSender | None = None

    def set_websocket_sender(self, sender: WebSocketSender) -> None:
        """设置 WebSocket 发送器。"""
        self._websocket_sender = sender

    async def handle_launcher_query(
        self,
        query: str,
        user_id: str = "local-user",
    ) -> LauncherAction:
        """解析 Launcher 查询并返回对应操作。

        支持的语法：
        - "@agent-name": 启动与指定 Agent 的 DM 对话
        - "#room-name": 打开指定 Room
        - "agent-name": 启动与指定 Agent 的对话（没有@）
        - "room-name": 搜索并打开 Room（没有#）
        - 纯文本：打开主智能体 DM 对话

        Args:
            query: 用户输入的查询字符串
            user_id: 用户 ID

        Returns:
            LauncherAction: 包含动作类型、目标 ID 和初始消息
        """
        trimmed_query = query.strip()
        if not trimmed_query:
            logger.debug("Empty launcher query, returning open_agent_dm action")
            return LauncherAction(
                action_type="open_agent_dm",
                target_id=MainAgentProfile.AGENT_ID,
                initial_message=trimmed_query or None,
            )

        # 匹配 @agent-name 格式
        agent_match = re.match(r"^@([^\s#]+)\s*(.*)?$", trimmed_query)
        if agent_match:
            agent_keyword = agent_match.group(1).strip()
            initial_msg = agent_match.group(2).strip() or None
            agent = await self._find_agent_by_keyword(agent_keyword)

            if agent:
                logger.info(f"Found agent by keyword: {agent_keyword} -> {agent.agent.id}")
                return LauncherAction(
                    action_type="open_agent_dm",
                    target_id=agent.agent.id,
                    initial_message=initial_msg,
                )
            else:
                logger.info(f"Agent keyword '{agent_keyword}' not found, treating as main agent DM query")
                return LauncherAction(
                    action_type="open_agent_dm",
                    target_id=MainAgentProfile.AGENT_ID,
                    initial_message=trimmed_query,
                )

        # 匹配 #room-name 格式
        room_match = re.match(r"^#([^\s@]+)\s*(.*)?$", trimmed_query)
        if room_match:
            room_keyword = room_match.group(1).strip()
            initial_msg = room_match.group(2).strip() or None
            room = await self._find_room_by_keyword(room_keyword)

            if room:
                logger.info(f"Found room by keyword: {room_keyword} -> {room.room.id}")
                return LauncherAction(
                    action_type="open_room",
                    target_id=room.room.id,
                    initial_message=initial_msg,
                )
            else:
                logger.info(f"Room keyword '{room_keyword}' not found, treating as main agent DM query")
                return LauncherAction(
                    action_type="open_agent_dm",
                    target_id=MainAgentProfile.AGENT_ID,
                    initial_message=trimmed_query,
                )

        # 匹配纯 agent-name（没有 @，打开 DM 对话）
        agent = await self._find_agent_by_keyword(trimmed_query)
        if agent:
            logger.info(f"Found agent by name: {trimmed_query} -> {agent.agent.id}")
            return LauncherAction(
                action_type="open_agent_dm",
                target_id=agent.agent.id,
                initial_message=None,
            )

        # 匹配纯 room-name（没有 #，搜索并打开 Room）
        room = await self._find_room_by_keyword(trimmed_query)
        if room:
            logger.info(f"Found room by name: {trimmed_query} -> {room.room.id}")
            return LauncherAction(
                action_type="open_room",
                target_id=room.room.id,
                initial_message=None,
            )

        # 默认：直接打开主智能体 DM，避免再维护第二套入口语义。
        logger.debug(f"No match found for query: '{trimmed_query}', returning open_agent_dm action")
        return LauncherAction(
            action_type="open_agent_dm",
            target_id=MainAgentProfile.AGENT_ID,
            initial_message=trimmed_query,
        )

    async def _find_agent_by_keyword(
        self,
        keyword: str,
    ) -> AgentAggregate | None:
        """通过关键词查找 Agent。"""
        agents = await persistence_service.list_agents()
        keyword_lower = keyword.lower()

        # 精确匹配
        for agent in agents:
            if agent.agent.name.lower() == keyword_lower:
                return agent

        # 模糊匹配（包含）
        for agent in agents:
            if keyword_lower in agent.agent.name.lower():
                return agent

        return None

    async def _find_room_by_keyword(
        self,
        keyword: str,
    ) -> RoomAggregate | None:
        """通过关键词查找 Room。"""
        rooms = await persistence_service.list_rooms(limit=100)
        collaborative_rooms = [
            room
            for room in rooms
            if room.room.room_type == "room"
        ]
        keyword_lower = keyword.lower()

        # 精确匹配名称
        for room in collaborative_rooms:
            if room.room.name and room.room.name.lower() == keyword_lower:
                return room

        # 模糊匹配名称
        for room in collaborative_rooms:
            if room.room.name and keyword_lower in room.room.name.lower():
                return room

        return None

    async def get_suggested_rooms(
        self,
        limit: int = 5,
        user_id: str = "local-user",
    ) -> list[RoomAggregate]:
        """获取用户最近参与的 Room，用于 Launcher 推荐。

        Args:
            limit: 返回数量限制
            user_id: 用户 ID

        Returns:
            最近参与的 Room 列表
        """
        rooms = await persistence_service.list_rooms(limit=limit * 3)
        collaborative_rooms = [
            room
            for room in rooms
            if room.room.room_type == "room"
        ]
        suggested_rooms = collaborative_rooms[:limit]
        logger.debug(f"Returning {len(suggested_rooms)} suggested rooms")
        return suggested_rooms

    async def get_suggested_agents(
        self,
        limit: int = 8,
        user_id: str = "local-user",
    ) -> list[AgentAggregate]:
        """获取用户最近交互的 Agent，用于 Launcher 推荐。

        Args:
            limit: 返回数量限制
            user_id: 用户 ID

        Returns:
            最近的 Agent 列表
        """
        agents = await persistence_service.list_agents()
        # 目前返回所有 Agent，未来可以根据使用频率排序
        logger.debug(f"Returning {len(agents)} suggested agents")
        return agents[:limit]


launcher_service = LauncherService()
