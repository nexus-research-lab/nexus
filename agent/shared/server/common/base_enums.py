# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_enums
# @Date   ：2024/1/22 23:25
# @Author ：leemysw

# 2024/1/22 23:25   Create
# =====================================================
import enum


class Method(str, enum.Enum):
    GET = 'GET'
    PUT = 'PUT'
    POST = 'POST'
    PATCH = 'PATCH'
    UPDATE = 'UPDATE'
    DELETE = 'DELETE'
    OPTIONS = 'OPTIONS'
