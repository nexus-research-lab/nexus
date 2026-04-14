# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：router
# @Date   ：2024/1/22 23:22
# @Author ：leemysw
#
# 2024/1/22 23:22   Create
# =====================================================

from fastapi import APIRouter, Depends

from agent.api.automation.api_heartbeat import router as automation_heartbeat_router
from agent.api.agent.api_agent import router as agent_router
from agent.api.agent.api_agent_workspace import router as agent_workspace_router
from agent.api.auth.api_auth import router as auth_router
from agent.api.capability.api_connector import router as connector_router
from agent.api.capability.api_scheduled_task import router as scheduled_task_router
from agent.api.capability.api_skill import router as capability_skill_router
from agent.api.chat.websocket_server import router as websocket_router
from agent.api.common.api_runtime import router as runtime_router
from agent.api.common.api_provider_config import router as provider_config_router
from agent.api.repository.api_persistence import router as persistence_router
from agent.api.room.api_room import router as room_router
from agent.api.session.api_session import router as session_router
from agent.api.launcher.api_launcher import router as launcher_router
from agent.config.config import settings
from agent.infra.server.common.base_depends import extract_request_id, require_http_auth

api_router = APIRouter(dependencies=[Depends(extract_request_id)], prefix=settings.API_PREFIX)

# Include websocket router
if settings.WEBSOCKET_ENABLED:
    api_router.include_router(websocket_router, prefix="/v1")

# Include auth router
api_router.include_router(auth_router, prefix="/v1")

# Include to agent router
api_router.include_router(agent_router, prefix="/v1", dependencies=[Depends(require_http_auth)])
api_router.include_router(agent_workspace_router, prefix="/v1", dependencies=[Depends(require_http_auth)])
api_router.include_router(connector_router, prefix="/v1", dependencies=[Depends(require_http_auth)])
api_router.include_router(scheduled_task_router, prefix="/v1", dependencies=[Depends(require_http_auth)])
api_router.include_router(capability_skill_router, prefix="/v1", dependencies=[Depends(require_http_auth)])

# Include to session router
api_router.include_router(session_router, prefix="/v1", dependencies=[Depends(require_http_auth)])

# Include to room router
api_router.include_router(room_router, prefix="/v1", dependencies=[Depends(require_http_auth)])

# Include to persistence router
api_router.include_router(persistence_router, prefix="/v1", dependencies=[Depends(require_http_auth)])

# Include to launcher router
api_router.include_router(launcher_router, prefix="/v1", dependencies=[Depends(require_http_auth)])

# Include runtime route
api_router.include_router(runtime_router, prefix="/v1")
api_router.include_router(provider_config_router, prefix="/v1", dependencies=[Depends(require_http_auth)])

# Include automation route
api_router.include_router(automation_heartbeat_router, prefix="/v1", dependencies=[Depends(require_http_auth)])
