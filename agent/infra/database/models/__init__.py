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
from agent.infra.database.models.contact import Contact
from agent.infra.database.models.conversation import Conversation
from agent.infra.database.models.member import Member
from agent.infra.database.models.message import Message
from agent.infra.database.models.profile import Profile
from agent.infra.database.models.protocol import (
    ActionRequest,
    ActionSubmission,
    Channel,
    ChannelMember,
    ProtocolDefinition,
    ProtocolRun,
    RunStateSnapshot,
)
from agent.infra.database.models.room import Room
from agent.infra.database.models.round import Round
from agent.infra.database.models.runtime import Runtime
from agent.infra.database.models.session import Session

__all__ = [
    "Agent",
    "Profile",
    "Runtime",
    "Contact",
    "Room",
    "Member",
    "ProtocolDefinition",
    "ProtocolRun",
    "Channel",
    "ChannelMember",
    "ActionRequest",
    "ActionSubmission",
    "RunStateSnapshot",
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
        Profile,
        Runtime,
        Contact,
        Room,
        Member,
        ProtocolDefinition,
        ProtocolRun,
        Channel,
        ChannelMember,
        ActionRequest,
        ActionSubmission,
        RunStateSnapshot,
        Conversation,
        Session,
        Message,
        Round,
    )
