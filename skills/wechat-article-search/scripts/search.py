#!/usr/bin/env python3
"""通过搜狗微信搜索发现微信公众号文章。"""

from __future__ import annotations

import argparse
import json
import random
import re
import sys
import time
from dataclasses import asdict, dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Callable, Sequence
from urllib.parse import urljoin, urlparse

try:
    import requests
    from bs4 import BeautifulSoup
    from bs4.element import Tag
except ModuleNotFoundError:
    requests = None
    BeautifulSoup = None
    Tag = Any


DEFAULT_RESULT_COUNT = 10
MAX_RESULT_COUNT = 50
PAGE_SIZE = 10
REQUEST_DELAY_SECONDS = 0.8
REQUEST_TIMEOUT_SECONDS = 20
SEARCH_ORIGIN = "https://weixin.sogou.com"
SEARCH_ENDPOINT = f"{SEARCH_ORIGIN}/weixin"
COOKIE_BOOTSTRAP_ENDPOINT = "https://v.sogou.com/v?ie=utf8&query=&p=40030600"
BASE_SOGOU_COOKIE = (
    "ABTEST=7|1716888919|v1; IPLOC=CN5101; ariaDefaultTheme=default; "
    "ariaFixed=true; ariaReadtype=1; ariaStatus=false"
)
CHINA_TIMEZONE = timezone(timedelta(hours=8))
USER_AGENTS = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2_1) "
    "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64; rv:123.0) "
    "Gecko/20100101 Firefox/123.0",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) "
    "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 "
    "Mobile/15E148 Safari/604.1",
    "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
)


class SearchError(Exception):
    """表示可向调用方稳定暴露的搜索失败。"""

    def __init__(
        self,
        code: str,
        message: str,
        details: dict[str, Any] | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.details = details or {}


@dataclass(frozen=True)
class Article:
    """表示一篇搜索结果文章。"""

    title: str
    url: str
    summary: str
    datetime: str
    date_text: str
    date_description: str
    source: str
    url_resolved: bool | None = None

    def to_dict(self) -> dict[str, Any]:
        """投影为稳定 JSON 结构。"""
        result = asdict(self)
        if self.url_resolved is None:
            result.pop("url_resolved")
        return result


@dataclass(frozen=True)
class CLIOptions:
    """保存命令行解析后的规范输入。"""

    query: str
    num: int
    output: str
    resolve_url: bool
    sort: str
    input_html: str


class SearchPageParser:
    """把搜狗微信搜索页解析成文章列表。"""

    def parse(self, html: str, max_results: int) -> list[Article]:
        """解析一页搜索结果。"""
        self._ensure_dependencies()
        self._assert_search_page(html)
        soup = BeautifulSoup(html, "html.parser")
        news_list = soup.select_one("ul.news-list")
        if news_list is None:
            return []

        items = news_list.find_all("li", recursive=False)
        articles: list[Article] = []
        for item in items:
            article = self._parse_article(item)
            if article is not None:
                articles.append(article)
            if len(articles) >= max_results:
                break
        return articles

    @staticmethod
    def _assert_search_page(html: str) -> None:
        """区分空结果、访问限制和非 HTML 响应。"""
        normalized = (html or "").lower()
        if any(
            marker in normalized
            for marker in ("antispider", "请输入验证码", "访问过于频繁")
        ):
            raise SearchError(
                "antispider",
                "搜狗微信返回了验证码或访问限制，请停止重试并稍后再试。",
            )
        if "<html" not in normalized and "<!doctype" not in normalized:
            raise SearchError("page_changed", "搜索来源没有返回可识别的 HTML 页面。")

    def _parse_article(self, item: Tag) -> Article | None:
        """解析单篇文章，缺失字段保持为空。"""
        title_link = item.select_one("h3 a")
        if title_link is None:
            return None
        title = clean_text(title_link)
        if not title:
            return None

        summary = clean_text(item.select_one("p.txt-info"))
        source_node = item.select_one(".s-p .all-time-y2")
        if source_node is None:
            source_node = item.select_one(".s-p a.account")
        published = self._parse_published_time(item)
        return Article(
            title=title,
            url=normalize_article_url(str(title_link.get("href", ""))),
            summary=summary,
            datetime=published["datetime"],
            date_text=published["date_text"],
            date_description=published["description"],
            source=clean_text(source_node),
        )

    @staticmethod
    def _parse_published_time(item: Tag) -> dict[str, str]:
        """从脚本时间戳或页面可见文字解析发布时间。"""
        script_text = " ".join(
            str(node.string or node.get_text())
            for node in item.select(".s-p .s2 script")
        )
        timestamp_match = re.search(r"(\d{10})", script_text)
        if timestamp_match:
            date = datetime.fromtimestamp(
                int(timestamp_match.group(1)),
                tz=CHINA_TIMEZONE,
            )
            formatted = format_china_date(date)
            return {
                "datetime": formatted["datetime"],
                "date_text": formatted["date_text"],
                "description": formatted["date_text"],
            }

        time_node = item.select_one(".s-p .s2")
        time_text = visible_text_without_scripts(time_node)
        parsed = parse_visible_date(time_text)
        return {
            "datetime": parsed["datetime"],
            "date_text": parsed["date_text"],
            "description": time_text or parsed["date_text"],
        }

    @staticmethod
    def _ensure_dependencies() -> None:
        """在依赖缺失时返回可执行的安装提示。"""
        ensure_dependencies()


class SogouWechatClient:
    """封装搜狗会话、低频访问和直达链接解析。"""

    def __init__(
        self,
        session: Any | None = None,
        parser: SearchPageParser | None = None,
        sleep_fn: Callable[[float], None] = time.sleep,
        choose_user_agent: Callable[[Sequence[str]], str] = random.choice,
    ) -> None:
        ensure_dependencies()
        self._session = session or requests.Session()
        self._parser = parser or SearchPageParser()
        self._sleep = sleep_fn
        self._choose_user_agent = choose_user_agent
        self._cookie = BASE_SOGOU_COOKIE

    def search(self, query: str, max_results: int) -> list[Article]:
        """按关键词搜索文章。"""
        self._initialize_session()
        page_count = (max_results + PAGE_SIZE - 1) // PAGE_SIZE
        articles: list[Article] = []

        for page in range(1, page_count + 1):
            html = self._request_text(
                SEARCH_ENDPOINT,
                params={
                    "query": query,
                    "s_from": "input",
                    "_sug_": "n",
                    "type": "2",
                    "page": str(page),
                    "ie": "utf8",
                },
            )
            remaining = max_results - len(articles)
            parsed = self._parser.parse(html, remaining)
            articles.extend(parsed)
            if not parsed or len(articles) >= max_results:
                break
            self._sleep(REQUEST_DELAY_SECONDS)
        return articles[:max_results]

    def resolve_article_urls(self, articles: list[Article]) -> list[Article]:
        """逐条解析微信直达链接。"""
        resolved: list[Article] = []
        for index, article in enumerate(articles):
            direct_url = self._resolve_article_url(article.url)
            succeeded = is_wechat_article_url(direct_url)
            resolved.append(
                Article(
                    title=article.title,
                    url=direct_url if succeeded else article.url,
                    summary=article.summary,
                    datetime=article.datetime,
                    date_text=article.date_text,
                    date_description=article.date_description,
                    source=article.source,
                    url_resolved=succeeded,
                )
            )
            if index < len(articles) - 1:
                self._sleep(REQUEST_DELAY_SECONDS)
        return resolved

    def _initialize_session(self) -> None:
        """预热搜狗会话，并拼接基础 Cookie 与 SNUID。"""
        self._cookie = BASE_SOGOU_COOKIE
        try:
            response = self._request(
                COOKIE_BOOTSTRAP_ENDPOINT,
                allow_status=True,
            )
            snuid = response.cookies.get("SNUID", "")
            if not snuid:
                snuid = self._session.cookies.get("SNUID", "")
            self._cookie = build_sogou_cookie(snuid)
        except Exception:
            # 预热失败时保留基础 Cookie，搜索本身仍可继续。
            self._cookie = BASE_SOGOU_COOKIE

    def _resolve_article_url(self, article_url: str) -> str:
        """只跟随来源显式返回的跳转。"""
        if not article_url or not is_sogou_url(article_url):
            return article_url
        try:
            response = self._request(
                article_url,
                allow_status=True,
                allow_redirects=False,
            )
            location = response.headers.get("Location", "")
            if is_wechat_article_url(location):
                return location
            if response.status_code == 200:
                redirect_url = extract_redirect_url(response.text)
                if is_wechat_article_url(redirect_url):
                    return redirect_url
        except Exception:
            return article_url
        return article_url

    def _request_text(
        self,
        url: str,
        params: dict[str, str] | None = None,
    ) -> str:
        """请求一页文本，并保持来源声明的字符编码。"""
        response = self._request(url, params=params)
        if not response.encoding:
            response.encoding = response.apparent_encoding or "utf-8"
        return response.text

    def _request(
        self,
        url: str,
        *,
        params: dict[str, str] | None = None,
        allow_status: bool = False,
        allow_redirects: bool = True,
    ) -> Any:
        """请求来源；只对网络错误和服务端临时错误重试一次。"""
        for attempt in range(2):
            try:
                response = self._session.get(
                    url,
                    params=params,
                    headers=self._request_headers(),
                    timeout=REQUEST_TIMEOUT_SECONDS,
                    allow_redirects=allow_redirects,
                )
            except Exception as error:
                if attempt == 0:
                    self._sleep(REQUEST_DELAY_SECONDS)
                    continue
                raise SearchError(
                    "network_error",
                    f"请求搜狗微信失败：{error}",
                ) from error

            if allow_status or response.ok:
                return response
            if response.status_code == 429:
                raise SearchError(
                    "rate_limited",
                    "搜狗微信返回了限流响应，请停止重试并稍后再试。",
                )
            if response.status_code >= 500 and attempt == 0:
                self._sleep(REQUEST_DELAY_SECONDS)
                continue
            raise SearchError(
                "http_error",
                f"搜狗微信返回 HTTP {response.status_code}。",
                {"status": response.status_code},
            )
        raise SearchError("network_error", "请求搜狗微信失败。")

    def _request_headers(self) -> dict[str, str]:
        """构造随机浏览器请求头和搜狗 Cookie。"""
        return {
            "Accept": "text/html,application/xhtml+xml",
            "Accept-Language": "zh-CN,zh;q=0.9",
            "Referer": f"{SEARCH_ORIGIN}/",
            "User-Agent": self._choose_user_agent(USER_AGENTS),
            "Cookie": self._cookie,
        }


class RaisingArgumentParser(argparse.ArgumentParser):
    """把 argparse 错误转换为统一 JSON 错误。"""

    def error(self, message: str) -> None:
        raise SearchError("invalid_arguments", message)


class CLIParser:
    """解析微信公众号文章搜索命令行。"""

    def parse(self, arguments: Sequence[str]) -> CLIOptions:
        """返回规范化命令行输入。"""
        parser = RaisingArgumentParser(
            prog="search.py",
            description="搜索微信公众号文章",
        )
        parser.add_argument("query", nargs="+", help="搜索关键词")
        parser.add_argument(
            "-n",
            "--num",
            type=bounded_result_count,
            default=DEFAULT_RESULT_COUNT,
            help="返回数量，默认 10，最大 50",
        )
        parser.add_argument("-o", "--output", default="", help="输出 JSON 文件")
        parser.add_argument(
            "-r",
            "--resolve-url",
            action="store_true",
            help="尝试解析微信直达链接",
        )
        parser.add_argument(
            "--sort",
            choices=("relevance", "latest"),
            default="relevance",
            help="结果排序",
        )
        parser.add_argument(
            "--input-html",
            default="",
            help="从本地 HTML 解析，供离线验证",
        )
        namespace = parser.parse_args(list(arguments))
        return CLIOptions(
            query=" ".join(namespace.query).strip(),
            num=namespace.num,
            output=namespace.output,
            resolve_url=namespace.resolve_url,
            sort=namespace.sort,
            input_html=namespace.input_html,
        )


class WechatArticleSearchApplication:
    """组装命令行输入、搜索流程和 JSON 输出。"""

    def __init__(self, client: SogouWechatClient | None = None) -> None:
        self._client = client

    def run(self, arguments: Sequence[str]) -> dict[str, Any]:
        """执行一次搜索并输出 JSON。"""
        options = CLIParser().parse(arguments)
        if options.input_html:
            html = Path(options.input_html).read_text(encoding="utf-8")
            articles = SearchPageParser().parse(html, options.num)
        else:
            client = self._client or SogouWechatClient()
            articles = client.search(options.query, options.num)
            if options.resolve_url and articles:
                articles = client.resolve_article_urls(articles)

        if options.sort == "latest":
            articles = sort_articles_by_latest(articles)
        result = {
            "query": options.query,
            "sort": options.sort,
            "total": len(articles),
            "fetched_at": utc_timestamp(),
            "articles": [article.to_dict() for article in articles],
        }
        output = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
        if options.output:
            output_path = Path(options.output).expanduser().resolve()
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_text(output, encoding="utf-8")
        sys.stdout.write(output)
        return result


def ensure_dependencies() -> None:
    """检查运行依赖，并给出 Skill 内的安装命令。"""
    missing: list[str] = []
    if requests is None:
        missing.append("requests")
    if BeautifulSoup is None:
        missing.append("beautifulsoup4")
    if not missing:
        return
    requirements = Path(__file__).resolve().parent.parent / "requirements.txt"
    raise SearchError(
        "dependency_missing",
        "缺少 Python 依赖："
        + "、".join(missing)
        + f"。请执行：python3 -m pip install -r {requirements}",
    )


def bounded_result_count(value: str) -> int:
    """校验搜索结果数量。"""
    try:
        count = int(value)
    except ValueError as error:
        raise argparse.ArgumentTypeError("--num 必须是整数") from error
    if count < 1 or count > MAX_RESULT_COUNT:
        raise argparse.ArgumentTypeError(
            f"--num 必须是 1 到 {MAX_RESULT_COUNT} 的整数"
        )
    return count


def clean_text(node: Tag | None) -> str:
    """读取节点纯文本并压缩空白。"""
    if node is None:
        return ""
    return " ".join(node.get_text(" ", strip=True).split())


def visible_text_without_scripts(node: Tag | None) -> str:
    """读取时间节点中不属于 script 的可见文本。"""
    if node is None:
        return ""
    values = (
        str(text).strip()
        for text in node.find_all(string=True)
        if getattr(text.parent, "name", "") != "script"
    )
    return " ".join(value for value in values if value)


def normalize_article_url(raw_url: str) -> str:
    """把搜索结果链接统一为绝对 HTTPS URL。"""
    value = (raw_url or "").strip()
    if value.startswith("//"):
        return f"https:{value}"
    return urljoin(SEARCH_ORIGIN, value) if value else ""


def format_china_date(date: datetime) -> dict[str, str]:
    """把时间格式化为中国时区稳定字符串。"""
    china_date = date.astimezone(CHINA_TIMEZONE)
    return {
        "datetime": china_date.strftime("%Y-%m-%d %H:%M:%S"),
        "date_text": china_date.strftime("%Y年%m月%d日"),
    }


def parse_visible_date(
    time_text: str,
    now: datetime | None = None,
) -> dict[str, str]:
    """解析页面直接显示的绝对或相对日期。"""
    text = (time_text or "").strip()
    absolute = re.search(r"(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})日?", text)
    if absolute:
        date = datetime(
            int(absolute.group(1)),
            int(absolute.group(2)),
            int(absolute.group(3)),
            tzinfo=CHINA_TIMEZONE,
        )
        return format_china_date(date)

    relative = re.search(r"(\d+)\s*(天|小时|分钟)前", text)
    if relative:
        current = now or datetime.now(tz=CHINA_TIMEZONE)
        units = {
            "天": timedelta(days=1),
            "小时": timedelta(hours=1),
            "分钟": timedelta(minutes=1),
        }
        date = current - int(relative.group(1)) * units[relative.group(2)]
        return format_china_date(date)
    return {"datetime": "", "date_text": ""}


def extract_redirect_url(html: str) -> str:
    """从搜狗中间页提取显式重定向地址。"""
    source = html or ""
    meta = re.search(
        r"""<meta[^>]*http-equiv=["']refresh["'][^>]*"""
        r"""content=["'][^"']*url\s*=\s*([^"']+)["'][^>]*>""",
        source,
        flags=re.IGNORECASE,
    )
    if meta:
        return meta.group(1).strip()

    direct = re.search(
        r"""(?:window\.)?location(?:\.href)?\s*=\s*["']([^"']+)["']""",
        source,
        flags=re.IGNORECASE,
    )
    if direct is None:
        direct = re.search(
            r"""window\.location\.replace\(\s*["']([^"']+)["']\s*\)""",
            source,
            flags=re.IGNORECASE,
        )
    if direct:
        return direct.group(1)

    parts = re.findall(
        r"""\burl\s*\+=\s*(?:'([^']*)'|"([^"]*)")""",
        source,
        flags=re.IGNORECASE,
    )
    assembled = "".join(left or right for left, right in parts)
    return assembled if is_wechat_article_url(assembled) else ""


def sort_articles_by_latest(articles: list[Article]) -> list[Article]:
    """把未知时间稳定放在末尾，其余按发布时间降序。"""
    indexed = list(enumerate(articles))
    indexed.sort(
        key=lambda item: (
            parse_datetime(item[1].datetime) is not None,
            parse_datetime(item[1].datetime) or datetime.min.replace(tzinfo=timezone.utc),
            -item[0],
        ),
        reverse=True,
    )
    return [article for _, article in indexed]


def parse_datetime(value: str) -> datetime | None:
    """把中国时间字符串解析为带时区时间。"""
    try:
        return datetime.strptime(value, "%Y-%m-%d %H:%M:%S").replace(
            tzinfo=CHINA_TIMEZONE
        )
    except ValueError:
        return None


def build_sogou_cookie(snuid: str) -> str:
    """拼接搜狗基础 Cookie 与预热响应中的 SNUID。"""
    normalized = (snuid or "").strip()
    if normalized:
        return f"{BASE_SOGOU_COOKIE}; SNUID={normalized}"
    return BASE_SOGOU_COOKIE


def is_sogou_url(value: str) -> bool:
    """判断 URL 是否属于搜狗微信来源。"""
    try:
        return urlparse(value).hostname.lower() == "weixin.sogou.com"
    except (AttributeError, ValueError):
        return False


def is_wechat_article_url(value: str) -> bool:
    """判断 URL 是否为微信公众平台文章地址。"""
    try:
        parsed = urlparse(value)
        return (
            parsed.scheme == "https"
            and parsed.hostname is not None
            and parsed.hostname.lower() == "mp.weixin.qq.com"
        )
    except ValueError:
        return False


def utc_timestamp() -> str:
    """返回毫秒精度 UTC 时间戳。"""
    return (
        datetime.now(tz=timezone.utc)
        .isoformat(timespec="milliseconds")
        .replace("+00:00", "Z")
    )


def report_error(error: Exception) -> None:
    """把错误输出为 Agent 可判断的稳定 JSON。"""
    if isinstance(error, SearchError):
        normalized = error
    else:
        normalized = SearchError("unexpected_error", str(error) or "未知错误")
    payload = {
        "code": normalized.code,
        "message": str(normalized),
        **normalized.details,
    }
    sys.stderr.write(json.dumps(payload, ensure_ascii=False) + "\n")


def main(arguments: Sequence[str] | None = None) -> int:
    """执行 CLI 并返回进程退出码。"""
    try:
        WechatArticleSearchApplication().run(
            list(arguments) if arguments is not None else sys.argv[1:]
        )
        return 0
    except SearchError as error:
        report_error(error)
        return 1
    except (OSError, ValueError) as error:
        report_error(error)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
