from __future__ import annotations

import importlib
import sys


def _reload_config_module(monkeypatch, debug_value: str):
    monkeypatch.setenv("ENV_FILE", "/dev/null")
    monkeypatch.setenv("DEBUG", debug_value)
    monkeypatch.delitem(sys.modules, "agent.config.config", raising=False)
    return importlib.import_module("agent.config.config")


def test_settings_accepts_release_as_false(monkeypatch):
    module = _reload_config_module(monkeypatch, "release")
    assert module.settings.DEBUG is False


def test_settings_accepts_debug_as_true(monkeypatch):
    module = _reload_config_module(monkeypatch, "debug")
    assert module.settings.DEBUG is True


def test_settings_default_database_url_uses_nexus_db(monkeypatch):
    module = _reload_config_module(monkeypatch, "false")
    assert module.settings.DATABASE_URL == "sqlite+aiosqlite:///~/.nexus/data/nexus.db"
