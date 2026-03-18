# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_error
# @Date   ：2024/2/28 0:23
# @Author ：leemysw

# 2024/2/28 0:23   Create
# =====================================================

from .base_resp import *


class ServerException(Exception):
    def __init__(self, errors):
        self.errors = errors
        self.resp = ServerError


class TokenAuthException(ServerException):
    def __init__(self, errors: str = "User Authentication Failed"):
        self.errors = errors
        self.resp = Unauthorized


class TokenExpiredException(ServerException):
    def __init__(self, errors: str = "Token has expired"):
        self.errors = errors
        self.resp = Unauthorized


class AuthenticationException(ServerException):
    def __init__(self, errors: str = "Permission denied"):
        self.errors = errors
        self.resp = Unauthorized
