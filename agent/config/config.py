# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：config
# @Date   ：2024/2/23 10:15
# @Author ：leemysw

# 2024/2/23 10:15   Create
# =====================================================

import os
from typing import Optional

from pydantic_settings import BaseSettings
from pydantic_settings import SettingsConfigDict

from agent.infra.schemas.model_cython import CyFunctionDetector

ROOT_PATH = os.path.abspath(os.path.abspath(os.path.dirname(__file__)) + '/../')
if os.environ.get("ENV_FILE"):
    ENV_FILE = os.environ.get("ENV_FILE")
elif os.path.isfile(os.path.join(os.getcwd(), ".env")):
    ENV_FILE = os.path.join(os.getcwd(), ".env")
else:
    ENV_FILE = os.path.join(ROOT_PATH, "../.env")

# http://patorjk.com/software/taag/#p=display&f=Lil%20Devil&t=v%201.0.1%0A
logo = """

     (`-')  _            (`-')  _<-. (`-')_ (`-')              <-.(`-')  _     (`-')      
     (OO ).-/     .->    ( OO).-/   \( OO) )( OO).->            __( OO) (_)    ( OO).->   
     / ,---.   ,---(`-')(,------.,--./ ,--/ /    '._   (`-')   '-'. ,--.,-(`-')/    '._   
     | \ /`.\ '  .-(OO ) |  .---'|   \ |  | |'--...__) ( OO).->|  .'   /| ( OO)|'--...__) 
     '-'|_.' ||  | .-, \(|  '--. |  . '|  |)`--.  .--'(,------.|      /)|  |  )`--.  .--' 
    (|  .-.  ||  | '.(_/ |  .--' |  |\    |    |  |    `------'|  .   '(|  |_/    |  |    
     |  | |  ||  '-'  |  |  `---.|  | \   |    |  |            |  |\   \|  |'->   |  |    
     `--' `--' `-----'   `------'`--'  `--'    `--'            `--' '--'`--'      `--'    
"""


class Settings(BaseSettings):
    # 项目信息, 服务配置
    LOGO: str = logo
    WORKERS: int = os.getenv("WORKERS", 1)
    DEBUG: bool = True
    PROJECT_NAME: str = "agent"
    API_PREFIX: str = "/agent"
    ENABLE_SWAGGER_DOC: bool = True
    SERVER_TYPE: str = "uvicorn"

    HOST: str = "0.0.0.0"
    PORT: int = os.getenv("PORT", 8010)
    DOMAIN: str = f'http://localhost:{PORT}'

    # 日志配置
    LOG_LEVEL: str = "INFO"
    LOG_NAME: str = "agent"
    LOG_PATH: str = os.path.abspath(os.path.join(os.getcwd(), "logs"))
    LOGGER_FORMAT: str = f"\033[97m[ \033[90m%(asctime)s \033[97m]\033[35m %(levelname)-7s \033[97m| \033[36m%(filename)s %(lineno)4d \033[97m - \033[32m%(message)s\033[97m"

    # 跨域配置
    BACKEND_CORS_ORIGINS: list = ["*"]

    # 权限配置，如果需要开启权限验证，请在.env文件中设置 ACCESS_TOKEN 的值
    # 采用Access Token模式，需要在请求头中添加Authorization字段，值以Bearer开头，后面跟上Access Token
    # Token 生成参考：openssl rand -hex 32
    ACCESS_TOKEN: Optional[str] = None

    # 缓存配置
    CACHE_FILE_DIR: str = os.path.abspath(os.path.join(os.getcwd(), "cache"))
    DEFAULT_CACHE_TTL_DAYS: int = 7

    # 数据库配置
    DATABASE_URL: str = "sqlite+aiosqlite:///./cache/data/data.db"

    # Key
    ANTHROPIC_AUTH_TOKEN: str = ""
    ANTHROPIC_BASE_URL: str = ""
    ANTHROPIC_MODEL: str = ""

    # =====================================================
    # 消息通道配置
    # =====================================================
    WEBSOCKET_ENABLED: bool = True
    DEFAULT_AGENT_ID: str = "nexus"
    PERMISSION_REQUEST_TIMEOUT_SECONDS: int = 90

    DISCORD_ENABLED: bool = False
    DISCORD_BOT_TOKEN: str = ""
    DISCORD_ALLOWED_GUILDS: str = ""  # 逗号分隔的 Guild ID
    DISCORD_TRIGGER_WORD: str = ""

    TELEGRAM_ENABLED: bool = False
    TELEGRAM_BOT_TOKEN: str = ""
    TELEGRAM_ALLOWED_USERS: str = ""  # 逗号分隔的 User ID

    # =====================================================
    # Workspace 配置
    # =====================================================
    WORKSPACE_PATH: str = ""  # 为空时使用 ~/.nexus-core/workspace
    BASE_SYSTEM_PROMPT: str = ""
    MAIN_AGENT_MODEL: str = ""
    MAIN_AGENT_SYSTEM_PROMPT: str = ""

    model_config = SettingsConfigDict(
        env_file=os.path.abspath(ENV_FILE),
        env_file_encoding="utf-8",
        extra="allow",
        case_sensitive=True,
        ignored_types=(CyFunctionDetector,)
    )

    def update_dependent_settings(self):
        os.environ["ANTHROPIC_AUTH_TOKEN"] = self.ANTHROPIC_AUTH_TOKEN
        os.environ["ANTHROPIC_BASE_URL"] = self.ANTHROPIC_BASE_URL
        os.environ["ANTHROPIC_MODEL"] = self.ANTHROPIC_MODEL
        if os.environ.get("CACHE_FILE_DIR"):
            self.CACHE_FILE_DIR = os.path.abspath(os.environ.get("CACHE_FILE_DIR"))
        else:
            self.CACHE_FILE_DIR = os.path.abspath(self.CACHE_FILE_DIR)
        ...

    def status(self, logger):
        self.update_dependent_settings()
        logger.info("USE: " + self.__class__.__name__)
        for attr in dir(self):
            if attr in ["model_computed_fields", "model_fields"]:
                continue
            if not attr.startswith("__") and \
                    not callable(getattr(self, attr)) and \
                    attr not in ["LOGO", "SECRET_KEY", "_abc_impl"] and \
                    not attr.startswith("model_"):
                logger.info(f"{attr}: {getattr(self, attr)}")
        for attr in self.model_extra:
            logger.info(f"{attr}: {getattr(self, attr)}")

    def __str__(self):
        text = "\n".join(
            [
                attr + ": " + str(getattr(self, attr))
                for attr in dir(self)
                if not attr.startswith("__") and
                not callable(getattr(self, attr)) and
                attr not in ["LOGO", "SECRET_KEY"]
            ]
        )
        return "\n" + text

    def __repr__(self):
        return self.__str__()


settings = Settings()
