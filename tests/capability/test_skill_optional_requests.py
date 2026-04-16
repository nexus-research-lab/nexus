from __future__ import annotations

from agent.service.capability.skills.skill_cli_runner import SkillCliRunner
from agent.service.capability.skills.skill_external_search_service import SkillExternalSearchService
from agent.service.capability.skills.skill_well_known_loader import SkillWellKnownLoader


class _FakeCli(SkillCliRunner):
    def __init__(self) -> None:
        self.calls: list[tuple[str, str]] = []

    def run_skills_cli_find(self, query: str) -> str:
        self.calls.append(("find", query))
        return "1 demo-skill github.com/demo/repo 42"

    def fetch_skill_markdown(self, detail_url: str) -> str:
        self.calls.append(("preview", detail_url))
        return "# demo"


def test_external_skill_search_falls_back_to_cli_when_requests_missing(monkeypatch):
    cli = _FakeCli()
    service = SkillExternalSearchService(cli)
    monkeypatch.setattr(
        SkillExternalSearchService,
        "_require_requests",
        staticmethod(lambda: (_ for _ in ()).throw(ValueError("requests 未安装，回退到 skills CLI 搜索"))),
    )

    items = service.search("demo")

    assert len(items) == 1
    assert items[0].name == "demo-skill"
    assert cli.calls == [("find", "demo")]


def test_well_known_loader_only_fails_when_network_is_actually_used(monkeypatch):
    loader = SkillWellKnownLoader(timeout_seconds=5)
    monkeypatch.setattr(
        SkillWellKnownLoader,
        "_require_requests",
        staticmethod(lambda: (_ for _ in ()).throw(ValueError("requests 未安装，无法加载远端 skill 索引"))),
    )

    assert loader.is_well_known_spec("skills.sh")

    try:
        loader._get_stream("https://skills.sh/.well-known/skills/index.json")
    except ValueError as exc:
        assert "requests 未安装" in str(exc)
    else:
        raise AssertionError("expected _get_stream to fail when requests is missing")
