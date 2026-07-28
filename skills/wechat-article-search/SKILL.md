---
name: wechat-article-search
title: 微信公众号文章搜索
version: 1.0.0
category_key: research-analysis
category_name: 研究与分析
tags:
  - wechat
  - official-account
  - article-search
  - chinese-research
allowed-tools:
  - Bash
  - Read
  - Write
  - WebSearch
  - WebFetch
description: >-
  搜索微信公众号文章，并整理标题、摘要、发布时间、来源公众号和链接。用户提到微信公众号、
  公众号文章、微信文章、搜一批公众号资料、按关键词找公众号内容、查某公众号相关报道，
  或需要为中文研究收集微信公众平台文章时使用；即使用户只说“搜微信里的文章”也应触发。
  本 Skill 负责文章发现和结果整理，不用于批量抓取正文或长期监控。
recommendation: 适合按关键词发现微信公众号文章，并快速整理来源、时间与可访问链接。
metadata:
  security:
    allowed_domains:
      - weixin.sogou.com
      - mp.weixin.qq.com
---

# 微信公众号文章搜索

按关键词发现微信公众号文章，并把可核验的搜索结果整理给用户。默认使用内置 Python
脚本访问搜狗微信搜索；脚本依赖 `requests` 与 `beautifulsoup4`。当本机缺少 Python、
依赖未安装、来源限流或页面结构暂时不可解析时，再使用 `WebSearch` / `WebFetch` 做明确
标注的降级检索。

## 运行依赖

需要 Python 3.10+、`requests` 和 `beautifulsoup4`。执行前先检查：

```bash
python3 --version
python3 -c "import requests, bs4"
```

如果 Python 包缺失，先向用户说明需要安装依赖，并在用户同意后执行：

```bash
python3 -m pip install -r "${CLAUDE_SKILL_DIR}/requirements.txt"
```

不要静默安装依赖，也不要改动用户现有的 Python 环境。

## 请求策略

- `requests` 负责 HTTP 会话、Cookie 和超时，`BeautifulSoup` 负责解析搜索结果 DOM。
- 每次请求从有限的浏览器 User-Agent 池中选择一个，并在请求前访问搜狗视频入口预热会话。
- 搜索请求会带基础搜狗 Cookie，并叠加预热响应提供的 `SNUID`；这只是兼容搜狗搜索页的
  请求策略，不代表拥有微信文章访问权限。
- 仍然采用低频人工请求、固定延迟和一次重试；遇到验证码、反爬或限流就停止。

## 执行流程

1. 从用户请求提取关键词、数量和是否需要直达微信链接。数量未指定时用 10，最大 50；
   不要为了默认值额外追问。
2. 先运行脚本。`${CLAUDE_SKILL_DIR}` 在 nxs 与 Claude Code 中都会展开为当前 Skill
   的真实目录：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/search.py" "关键词" --num 10
```

3. 读取 stdout JSON，按[结果格式](#结果格式)回答。不要把脚本的“0 条”扩写成“网上没有”；
   它只表示本次来源没有返回可解析结果。
4. 只有用户明确要求 `mp.weixin.qq.com` 直达链接，或后续任务确实需要抓取正文时，才使用
   `--resolve-url`。链接解析会逐条增加请求，建议一次不超过 10 篇：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/search.py" "关键词" --num 5 --resolve-url
```

5. 用户要求保存时才传 `--output`，优先写入用户指定目录；未指定目录时先给结果，不要自行
   把文件散落在 workspace 根目录：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/search.py" "关键词" --num 20 --output "research/wechat-results.json"
```

### 排序

默认保持搜索相关性顺序。用户明确要“最新”“最近”时，使用 `--sort latest`；这只会对本次
检索到的结果按可解析发布时间降序排列，不能宣称覆盖全部公众号文章：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/search.py" "关键词" --num 20 --sort latest
```

## 降级检索

遇到以下情况时停止重复调用脚本，改走降级链路：

- `python3` 不存在，或 stderr 的错误码是 `dependency_missing`；
- stderr 的错误码是 `antispider`、`rate_limited` 或 `page_changed`；
- 连续一次正常重试后仍是网络错误。

使用 `WebSearch` 搜索：

```text
site:mp.weixin.qq.com/s "关键词"
```

对最多 5 个高相关结果用 `WebFetch` 核验标题、公众号和发布时间。只返回工具真实提供的
字段；无法核验的字段写“未核验”，不要从 URL、摘要或账号习惯推断。回答中说明“搜狗微信
检索不可用，以下来自公开网页索引”，避免把降级结果伪装成同一数据源。

如果 `WebSearch` 也未配置或失败，直接说明当前缺少可用搜索来源，并给出可执行建议：
稍后重试、缩短关键词、去掉特殊字符，或让用户提供候选链接。不要循环请求触发更严格限流。

## 结果格式

脚本输出一个 JSON 对象：

```json
{
  "query": "AI Agent",
  "sort": "relevance",
  "total": 1,
  "fetched_at": "2026-07-28T08:00:00.000Z",
  "articles": [
    {
      "title": "文章标题",
      "url": "https://weixin.sogou.com/link?...",
      "summary": "搜索结果摘要",
      "datetime": "2026-07-27 10:30:00",
      "date_text": "2026年07月27日",
      "date_description": "2026年07月27日",
      "source": "公众号名称"
    }
  ]
}
```

向用户展示时优先使用紧凑编号列表，每条包含：

```text
标题 — 公众号 · 发布时间
摘要
链接
```

- `url_resolved: true` 表示已解析为微信直达链接。
- `url_resolved: false` 时保留可访问的搜狗中间链接，不要声称它是直达链接。
- `datetime` 为空表示来源没有提供可解析时间；不要补造日期。
- 用户要求“整理参考资料”时，可在结果之后按主题聚类，但保持原始链接与来源可追溯。

## 使用边界

- 本 Skill 用于低频、人工发起的资料发现，不用于批量采集、持续爬取或规避验证码与明确封禁。
- 尊重站点条款和访问限制；出现验证码、反爬或限流就停止自动重试。
- 搜索结果只证明索引页当时返回了该条目，不证明文章观点、事实或时效性。需要引用文章
  内容时，再获取原文并独立核验。
