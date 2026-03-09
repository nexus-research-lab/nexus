# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_websocket
# @Date   ：2025/12/7 02:37
# @Author ：leemysw

# 2025/12/7 02:37   Create
# =====================================================

from pydantic import BaseModel

class WSMessage(BaseModel):
    message_type: str
    agent_id: str
    message: str
