from __future__ import annotations

import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
if str(ROOT) not in sys.path:
    # 让直接运行 pytest 时也能导入仓库根目录下的 agent 包。
    sys.path.insert(0, str(ROOT))


def test_parse_heartbeat_tasks_reads_tasks_block():
    from agent.service.automation.heartbeat.heartbeat_prompt import parse_heartbeat_tasks

    tasks = parse_heartbeat_tasks(
        "tasks:\n- name: inbox\n  interval: 30m\n  prompt: \"check inbox\"\n"
    )

    assert len(tasks) == 1
    assert tasks[0].name == "inbox"
    assert tasks[0].interval == "30m"
    assert tasks[0].prompt == "check inbox"


def test_parse_heartbeat_tasks_ignores_non_tasks_sections():
    from agent.service.automation.heartbeat.heartbeat_prompt import parse_heartbeat_tasks

    tasks = parse_heartbeat_tasks(
        "title: heartbeat\n"
        "notes: keep this short\n"
        "tasks:\n"
        "- name: sync\n"
        "  interval: 15m\n"
        "  prompt: run sync\n"
        "\n"
        "summary: done\n"
    )

    assert len(tasks) == 1
    assert tasks[0].name == "sync"


def test_parse_heartbeat_tasks_supports_indented_item_fields():
    from agent.service.automation.heartbeat.heartbeat_prompt import parse_heartbeat_tasks

    tasks = parse_heartbeat_tasks(
        "tasks:\n"
        "-\n"
        "  name: backlog\n"
        "  interval: 1h\n"
        "  prompt: review backlog\n"
    )

    assert len(tasks) == 1
    assert tasks[0].name == "backlog"
    assert tasks[0].interval == "1h"
    assert tasks[0].prompt == "review backlog"


def test_parse_heartbeat_tasks_supports_multiline_prompt_block_scalar():
    from agent.service.automation.heartbeat.heartbeat_prompt import parse_heartbeat_tasks

    tasks = parse_heartbeat_tasks(
        "tasks:\n"
        "- name: report\n"
        "  interval: 1h\n"
        "  prompt: |\n"
        "    gather metrics\n"
        "    and summarize\n"
    )

    assert len(tasks) == 1
    assert tasks[0].prompt == "gather metrics\nand summarize"


def test_strip_heartbeat_ok_suppresses_ack_only_reply():
    from agent.service.automation.heartbeat.heartbeat_prompt import filter_heartbeat_response

    result = filter_heartbeat_response("HEARTBEAT_OK", ack_max_chars=300)
    assert result.should_deliver is False
    assert result.text == ""


def test_strip_heartbeat_ok_keeps_long_alert_text():
    from agent.service.automation.heartbeat.heartbeat_prompt import filter_heartbeat_response

    result = filter_heartbeat_response(
        "HEARTBEAT_OK\nalert: disk space is low",
        ack_max_chars=8,
    )

    assert result.should_deliver is True
    assert result.text == "alert: disk space is low"


def test_strip_heartbeat_ok_respects_ack_max_chars_threshold():
    from agent.service.automation.heartbeat.heartbeat_prompt import filter_heartbeat_response

    short_result = filter_heartbeat_response("HEARTBEAT_OK\nwarn", ack_max_chars=4)
    long_result = filter_heartbeat_response("HEARTBEAT_OK\nwarn", ack_max_chars=3)

    assert short_result.should_deliver is False
    assert short_result.text == ""
    assert long_result.should_deliver is True
    assert long_result.text == "warn"
