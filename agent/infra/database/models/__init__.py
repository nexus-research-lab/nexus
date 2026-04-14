# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""数据库 ORM 模型集合。"""

from agent.infra.database.models.agent import Agent
from agent.infra.database.models.automation_cron_job import AutomationCronJob
from agent.infra.database.models.automation_cron_run import AutomationCronRun
from agent.infra.database.models.automation_delivery_route import AutomationDeliveryRoute
from agent.infra.database.models.automation_heartbeat_state import AutomationHeartbeatState
from agent.infra.database.models.automation_system_event import AutomationSystemEvent
from agent.infra.database.models.auth_session import AuthSession
from agent.infra.database.models.contact import Contact
from agent.infra.database.models.conversation import Conversation
from agent.infra.database.models.member import Member
from agent.infra.database.models.message import Message
from agent.infra.database.models.provider_config import ProviderConfig
from agent.infra.database.models.profile import Profile
from agent.infra.database.models.room import Room
from agent.infra.database.models.round import Round
from agent.infra.database.models.runtime import Runtime
from agent.infra.database.models.session import Session
from agent.infra.database.models.connector import ConnectorConnection

__all__ = [
    "Agent",
    "AutomationCronJob",
    "AutomationCronRun",
    "AutomationDeliveryRoute",
    "AutomationHeartbeatState",
    "AutomationSystemEvent",
    "AuthSession",
    "ConnectorConnection",
    "ProviderConfig",
    "Profile",
    "Runtime",
    "Contact",
    "Room",
    "Member",
    "Conversation",
    "Session",
    "Message",
    "Round",
    "load_models",
]


def load_models() -> tuple[type, ...]:
    """显式加载所有 ORM 模型，确保 Base.metadata 完整。"""
    return (
        Agent,
        AutomationCronJob,
        AutomationCronRun,
        AutomationDeliveryRoute,
        AutomationHeartbeatState,
        AutomationSystemEvent,
        AuthSession,
        ConnectorConnection,
        ProviderConfig,
        Profile,
        Runtime,
        Contact,
        Room,
        Member,
        Conversation,
        Session,
        Message,
        Round,
    )
