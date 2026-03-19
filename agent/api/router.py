# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：router
# @Date   ：2024/1/22 23:22
# @Author ：leemysw

# 2024/1/22 23:22   Create
# =====================================================

from fastapi import APIRouter, Depends

from agent.api.agent.api_agent import router as agent_router
from agent.api.chat_ws.websocket_server import router as websocket_router
from agent.api.persistence.api_persistence import router as persistence_router
from agent.api.room.api_room import router as room_router
from agent.api.session.api_session import router as session_router
from agent.config.config import settings
from agent.infra.server.common.base_depends import extract_request_id

api_router = APIRouter(dependencies=[Depends(extract_request_id)], prefix=settings.API_PREFIX)

# Include the websocket router
if settings.WEBSOCKET_ENABLED:
    api_router.include_router(websocket_router, prefix="/v1")

# Include the agent router
api_router.include_router(agent_router, prefix="/v1")

# Include the session router
api_router.include_router(session_router, prefix="/v1")

# Include the room router
api_router.include_router(room_router, prefix="/v1")

# Include the persistence router
api_router.include_router(persistence_router, prefix="/v1")
