# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：websocket_server.py
# @Date   ：2025/11/28 15:27
# @Author ：leemysw

# 2025/11/28 15:27   Create
# =====================================================

from fastapi import APIRouter, WebSocket, WebSocketDisconnect

from agent.service.channels.ws.handler import WebSocketHandler
from agent.service.auth import auth_service
from agent.utils.logger import logger

router = APIRouter()


@router.websocket("/chat/ws")
async def chat(websocket: WebSocket):
    """
    WebSocket端点，处理前端连接

    Args:
        websocket: FastAPI WebSocket实例
    """
    logger.info("🌐新的WebSocket连接请求")

    try:
        if auth_service.is_auth_required():
            identity = await auth_service.resolve_websocket_identity(websocket)
            if not identity:
                await websocket.accept()
                await websocket.close(code=4401, reason="Unauthorized")
                logger.warning("❌WebSocket 未通过鉴权: %s", websocket.client)
                return
            websocket.state.auth_user = identity

        # 为每个连接创建独立的WebSocketHandler实例
        handler = WebSocketHandler()
        await handler.handle_websocket_connection(websocket)
    except WebSocketDisconnect as disconnect_error:
        logger.info(
            f"🔌WebSocket连接断开: code={disconnect_error.code}, reason={disconnect_error.reason}"
        )
    except Exception as e:
        logger.error(f"❌WebSocket端点处理失败: {e}")
        # 确保连接被关闭
        try:
            await websocket.close(code=1011, reason=f"Server error: {str(e)}")
        except Exception as e:
            logger.error(f"❌WebSocket关闭失败: {e}")
            pass  # 连接可能已经关闭


# 导出路由器
__all__ = ["router"]
