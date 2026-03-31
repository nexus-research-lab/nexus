# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：connector_catalog.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""内置连接器目录 —— 预定义第三方应用清单。"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional


@dataclass(frozen=True)
class CatalogEntry:
    """一条连接器目录记录。"""

    connector_id: str
    name: str
    title: str
    description: str
    icon: str  # emoji 或 icon key，前端映射
    category: str
    auth_type: str = "oauth2"
    status: str = "coming_soon"  # available | coming_soon
    auth_url: Optional[str] = None
    token_url: Optional[str] = None
    scopes: list[str] = field(default_factory=list)
    mcp_server_url: Optional[str] = None
    docs_url: Optional[str] = None
    features: list[str] = field(default_factory=list)


# ── 应用目录 ──────────────────────────────────────────

CONNECTOR_CATALOG: list[CatalogEntry] = [
    # ── 可用连接器（auth_type + status=available）────
    CatalogEntry(
        connector_id="gmail",
        name="gmail",
        title="Gmail 邮箱",
        description="完整管理邮件、标签与基础设置",
        icon="gmail",
        category="productivity",
        auth_type="oauth2",
        status="available",
        auth_url="https://accounts.google.com/o/oauth2/v2/auth",
        token_url="https://oauth2.googleapis.com/token",
        scopes=["https://www.googleapis.com/auth/gmail.modify"],
        docs_url="https://developers.google.com/gmail/api",
        features=["读取邮件", "发送邮件", "管理标签", "搜索邮件"],
    ),
    CatalogEntry(
        connector_id="x-twitter",
        name="x-twitter",
        title="X（Twitter）",
        description="读取推文、搜索对话并追踪趋势",
        icon="x-twitter",
        category="social",
        auth_type="oauth2",
        status="available",
        auth_url="https://twitter.com/i/oauth2/authorize",
        token_url="https://api.twitter.com/2/oauth2/token",
        scopes=["tweet.read", "tweet.write", "users.read"],
        docs_url="https://developer.twitter.com/en/docs",
        features=["读取推文", "发布推文", "搜索话题", "追踪趋势"],
    ),
    CatalogEntry(
        connector_id="linkedin",
        name="linkedin",
        title="LinkedIn",
        description="访问个人档案、公司数据和职位浏览",
        icon="linkedin",
        category="social",
        auth_type="oauth2",
        status="available",
        auth_url="https://www.linkedin.com/oauth/v2/authorization",
        token_url="https://www.linkedin.com/oauth/v2/accessToken",
        scopes=["r_liteprofile", "r_emailaddress"],
        docs_url="https://learn.microsoft.com/en-us/linkedin/",
        features=["个人档案", "公司搜索", "职位浏览", "人脉管理"],
    ),
    CatalogEntry(
        connector_id="shopify",
        name="shopify",
        title="Shopify",
        description="访问商品、订单与客户数据",
        icon="shopify",
        category="ecommerce",
        auth_type="oauth2",
        status="available",
        auth_url="https://{shop}.myshopify.com/admin/oauth/authorize",
        token_url="https://{shop}.myshopify.com/admin/oauth/access_token",
        scopes=["read_products", "read_orders", "read_customers"],
        docs_url="https://shopify.dev/docs/api",
        features=["商品管理", "订单查看", "客户数据", "库存同步"],
    ),
    CatalogEntry(
        connector_id="instagram",
        name="instagram",
        title="Instagram",
        description="访问账号、媒体和互动数据",
        icon="instagram",
        category="social",
        auth_type="oauth2",
        status="available",
        auth_url="https://api.instagram.com/oauth/authorize",
        token_url="https://api.instagram.com/oauth/access_token",
        scopes=["user_profile", "user_media"],
        docs_url="https://developers.facebook.com/docs/instagram-api",
        features=["媒体发布", "互动数据", "粉丝分析", "评论管理"],
    ),
    CatalogEntry(
        connector_id="github",
        name="github",
        title="GitHub",
        description="管理仓库、协作开发并跟踪问题",
        icon="github",
        category="development",
        auth_type="oauth2",
        status="available",
        auth_url="https://github.com/login/oauth/authorize",
        token_url="https://github.com/login/oauth/access_token",
        scopes=["repo", "read:user", "user:email"],
        docs_url="https://docs.github.com/en/rest",
        features=["仓库管理", "Issue 跟踪", "PR 审查", "代码搜索"],
    ),

    # ── 即将推出 ──────────────────────────────────────
    CatalogEntry(
        connector_id="google-calendar",
        name="google-calendar",
        title="Google 日历",
        description="读取日程并创建、修改事件",
        icon="google-calendar",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="google-drive",
        name="google-drive",
        title="Google 云盘",
        description="读取、检索并管理云盘文件",
        icon="google-drive",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="youtube",
        name="youtube",
        title="YouTube",
        description="读取、上传并管理频道与视频内容",
        icon="youtube",
        category="social",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="reddit",
        name="reddit",
        title="Reddit",
        description="搜索帖子、阅读讨论并监控社区",
        icon="reddit",
        category="social",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="tiktok",
        name="tiktok",
        title="TikTok",
        description="访问视频数据、热门内容和创作者洞察",
        icon="tiktok",
        category="social",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="odoo-erp",
        name="odoo-erp",
        title="Odoo ERP",
        description="访问 Odoo 模块（CRM、库存、财务等）",
        icon="odoo",
        category="business",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="square-pos",
        name="square-pos",
        title="Square POS",
        description="连接 Square POS，访问商户、目录与订单数据",
        icon="square",
        category="ecommerce",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="alibaba",
        name="alibaba",
        title="Alibaba.com",
        description="搜索供应商、商品与贸易数据",
        icon="alibaba",
        category="ecommerce",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="outlook",
        name="outlook",
        title="Outlook",
        description="高效管理邮件与日程",
        icon="outlook",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="airtable",
        name="airtable",
        title="Airtable",
        description="以协作方式组织与管理数据",
        icon="airtable",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="meta-ads",
        name="meta-ads",
        title="Meta Ads Manager",
        description="自动化广告洞察与优化，提升 ROI",
        icon="meta",
        category="marketing",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="ahrefs",
        name="ahrefs",
        title="Ahrefs",
        description="优化 SEO 策略并分析关键词",
        icon="ahrefs",
        category="marketing",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="similarweb",
        name="similarweb",
        title="Similarweb",
        description="分析网站流量与竞品情报",
        icon="similarweb",
        category="marketing",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="dropbox",
        name="dropbox",
        title="Dropbox",
        description="管理文件、文件夹和共享权限",
        icon="dropbox",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="slack",
        name="slack",
        title="Slack",
        description="读写 Slack 会话消息",
        icon="slack",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="notion",
        name="notion",
        title="Notion",
        description="通过 Integration Token 读写工作区内容",
        icon="notion",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="zapier",
        name="zapier",
        title="Zapier",
        description="连接数千应用并自动化工作流",
        icon="zapier",
        category="automation",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="monday",
        name="monday",
        title="monday.com",
        description="管理任务、看板和项目流程",
        icon="monday",
        category="productivity",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="make",
        name="make",
        title="Make",
        description="搭建自动化场景的 AI 可视化工具",
        icon="make",
        category="automation",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="linear",
        name="linear",
        title="Linear",
        description="管理 Issue、追踪项目进度",
        icon="linear",
        category="development",
        status="coming_soon",
    ),
    CatalogEntry(
        connector_id="atlassian",
        name="atlassian",
        title="Atlassian",
        description="集成 Jira、Confluence、Compass 等 AI 工具链",
        icon="atlassian",
        category="development",
        status="coming_soon",
    ),
]

# 目录类别映射
CONNECTOR_CATEGORIES: dict[str, str] = {
    "productivity": "效率工具",
    "social": "社交媒体",
    "ecommerce": "电商平台",
    "development": "开发工具",
    "business": "企业管理",
    "marketing": "营销分析",
    "automation": "自动化",
}


class ConnectorCatalog:
    """连接器内置目录管理。"""

    @staticmethod
    def list_all() -> list[CatalogEntry]:
        return CONNECTOR_CATALOG

    @staticmethod
    def get(connector_id: str) -> CatalogEntry | None:
        for entry in CONNECTOR_CATALOG:
            if entry.connector_id == connector_id:
                return entry
        return None

    @staticmethod
    def search(query: str) -> list[CatalogEntry]:
        """按名称/标题/描述模糊搜索。"""
        q = query.lower().strip()
        if not q:
            return CONNECTOR_CATALOG
        results: list[CatalogEntry] = []
        for entry in CONNECTOR_CATALOG:
            if (
                q in entry.name.lower()
                or q in entry.title.lower()
                or q in entry.description.lower()
                or q in entry.category.lower()
            ):
                results.append(entry)
        return results

    @staticmethod
    def list_categories() -> dict[str, str]:
        return CONNECTOR_CATEGORIES
