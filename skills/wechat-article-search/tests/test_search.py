"""微信公众号文章搜索脚本的离线回归测试。"""

from __future__ import annotations

import io
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from typing import Any

from scripts.search import (
    BASE_SOGOU_COOKIE,
    Article,
    CLIOptions,
    CLIParser,
    SearchError,
    SearchPageParser,
    SogouWechatClient,
    USER_AGENTS,
    WechatArticleSearchApplication,
    extract_redirect_url,
    sort_articles_by_latest,
)


FIXTURE_PATH = Path(__file__).parent / "fixtures" / "search-results.html"


class FakeResponse:
    """提供 requests.Response 的最小测试替身。"""

    def __init__(
        self,
        text: str,
        *,
        status_code: int = 200,
        headers: dict[str, str] | None = None,
        cookies: dict[str, str] | None = None,
    ) -> None:
        self.text = text
        self.status_code = status_code
        self.headers = headers or {}
        self.cookies = cookies or {}
        self.encoding = "utf-8"
        self.apparent_encoding = "utf-8"

    @property
    def ok(self) -> bool:
        """表示响应状态是否成功。"""
        return 200 <= self.status_code < 400


class FakeSession:
    """记录请求头并返回固定离线响应。"""

    def __init__(self, fixture: str) -> None:
        self.fixture = fixture
        self.calls: list[dict[str, Any]] = []
        self.cookies: dict[str, str] = {}

    def get(self, url: str, **kwargs: Any) -> FakeResponse:
        """返回会话预热或搜索 fixture。"""
        self.calls.append({"url": url, **kwargs})
        if url.startswith("https://v.sogou.com/"):
            return FakeResponse("", cookies={"SNUID": "fixture-snuid"})
        return FakeResponse(self.fixture)


class SearchPageParserTest(unittest.TestCase):
    """验证 BeautifulSoup 页面解析。"""

    def setUp(self) -> None:
        self.fixture = FIXTURE_PATH.read_text(encoding="utf-8")

    def test_parses_committed_fixture(self) -> None:
        articles = SearchPageParser().parse(self.fixture, 10)

        self.assertEqual(len(articles), 1)
        self.assertEqual(articles[0].title, "Codex & Agent 模型路由实战")
        self.assertEqual(articles[0].source, "智见 AI")
        self.assertEqual(
            articles[0].url,
            "https://weixin.sogou.com/link?url=demo",
        )
        self.assertEqual(articles[0].datetime, "2024-01-01 08:00:00")
        self.assertEqual(articles[0].date_text, "2024年01月01日")

    def test_rejects_antispider_page(self) -> None:
        with self.assertRaisesRegex(SearchError, "验证码"):
            SearchPageParser().parse(
                "<!doctype html><html><body>访问过于频繁，请输入验证码</body></html>",
                10,
            )


class RedirectParserTest(unittest.TestCase):
    """验证中间链接解析。"""

    def test_extracts_supported_redirect_forms(self) -> None:
        self.assertEqual(
            extract_redirect_url(
                '<meta http-equiv="refresh" '
                'content="0; url=https://mp.weixin.qq.com/s/a">'
            ),
            "https://mp.weixin.qq.com/s/a",
        )
        self.assertEqual(
            extract_redirect_url(
                'window.location.replace("https://mp.weixin.qq.com/s/b")'
            ),
            "https://mp.weixin.qq.com/s/b",
        )
        self.assertEqual(
            extract_redirect_url(
                "var url='';url += 'https://mp.weixin.';"
                "url += 'qq.com/s/c';window.location.replace(url)"
            ),
            "https://mp.weixin.qq.com/s/c",
        )


class CLIParserTest(unittest.TestCase):
    """验证命令行参数与输出。"""

    def test_parses_multiword_query_and_options(self) -> None:
        self.assertEqual(
            CLIParser().parse(
                [
                    "AI",
                    "Agent",
                    "--num",
                    "3",
                    "--output",
                    "out.json",
                    "--resolve-url",
                    "--sort",
                    "latest",
                ]
            ),
            CLIOptions(
                query="AI Agent",
                num=3,
                output="out.json",
                resolve_url=True,
                sort="latest",
                input_html="",
            ),
        )

    def test_rejects_invalid_count(self) -> None:
        with self.assertRaises(SearchError):
            CLIParser().parse(["Codex", "--num", "0"])

    def test_application_emits_json_from_local_fixture(self) -> None:
        output = io.StringIO()
        with redirect_stdout(output):
            result = WechatArticleSearchApplication().run(
                [
                    "Codex",
                    "--input-html",
                    str(FIXTURE_PATH),
                    "--num",
                    "1",
                ]
            )

        self.assertEqual(result["total"], 1)
        self.assertIn('"source": "智见 AI"', output.getvalue())


class ClientTest(unittest.TestCase):
    """验证随机 UA、Cookie 与排序行为。"""

    def test_uses_browser_ua_and_bootstrapped_cookie(self) -> None:
        session = FakeSession(FIXTURE_PATH.read_text(encoding="utf-8"))
        client = SogouWechatClient(
            session=session,
            sleep_fn=lambda _: None,
            choose_user_agent=lambda values: values[-1],
        )

        articles = client.search("Codex", 1)

        self.assertEqual(len(articles), 1)
        self.assertEqual(len(session.calls), 2)
        self.assertEqual(
            session.calls[0]["headers"]["User-Agent"],
            USER_AGENTS[-1],
        )
        self.assertEqual(
            session.calls[1]["headers"]["User-Agent"],
            USER_AGENTS[-1],
        )
        search_cookie = session.calls[1]["headers"]["Cookie"]
        self.assertIn(BASE_SOGOU_COOKIE, search_cookie)
        self.assertIn("SNUID=fixture-snuid", search_cookie)

    def test_sorts_unknown_time_last(self) -> None:
        articles = [
            Article("未知", "", "", "", "", "", ""),
            Article("较早", "", "", "2026-07-01 08:00:00", "", "", ""),
            Article("较新", "", "", "2026-07-02 08:00:00", "", "", ""),
        ]
        self.assertEqual(
            [article.title for article in sort_articles_by_latest(articles)],
            ["较新", "较早", "未知"],
        )


if __name__ == "__main__":
    unittest.main()
