# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_error_codes.py
# @Date   ：2026/04/14 21:06
# @Author ：leemysw
# 2026/04/14 21:06   Create
# =====================================================

"""权限链路的结构化错误码。"""

from __future__ import annotations


PERMISSION_REQUEST_TIMEOUT_MESSAGE = "Permission request timeout"
PERMISSION_CHANNEL_UNAVAILABLE_MESSAGE = "Permission channel unavailable"

ASK_USER_QUESTION_TIMEOUT_ERROR_CODE = "permission_request_timeout"
ASK_USER_QUESTION_CHANNEL_UNAVAILABLE_ERROR_CODE = "permission_channel_unavailable"


def infer_permission_error_code(tool_name: str, message: object) -> str | None:
    """根据工具类型与错误文本推导结构化错误码。"""
    normalized_message = str(message or "").strip()
    if tool_name != "AskUserQuestion" or not normalized_message:
        return None
    if normalized_message == PERMISSION_REQUEST_TIMEOUT_MESSAGE:
        return ASK_USER_QUESTION_TIMEOUT_ERROR_CODE
    if normalized_message == PERMISSION_CHANNEL_UNAVAILABLE_MESSAGE:
        return ASK_USER_QUESTION_CHANNEL_UNAVAILABLE_ERROR_CODE
    return None
