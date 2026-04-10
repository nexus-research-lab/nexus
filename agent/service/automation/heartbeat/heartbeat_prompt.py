from __future__ import annotations

from dataclasses import dataclass


ACK_TOKEN = "HEARTBEAT_OK"


@dataclass(slots=True)
class HeartbeatTask:
    name: str
    interval: str
    prompt: str


@dataclass(slots=True)
class HeartbeatFilterResult:
    should_deliver: bool
    text: str


def parse_heartbeat_tasks(text: str) -> list[HeartbeatTask]:
    """Parse the ``tasks:`` block from HEARTBEAT.md content."""

    # 只提取 tasks 段落，避免把其他章节误当成定时任务。
    lines = text.splitlines()
    tasks: list[HeartbeatTask] = []
    current: dict[str, str] = {}
    in_tasks = False
    tasks_indent = 0
    pending_block_key: str | None = None
    pending_block_indent = 0
    block_lines: list[str] = []
    block_content_indent = 0

    def flush_current() -> None:
        if current:
            task = _build_task(current)
            if task is not None:
                tasks.append(task)
            current.clear()

    def finish_block() -> None:
        nonlocal pending_block_key, pending_block_indent, block_lines, block_content_indent
        if pending_block_key is None:
            return
        current[pending_block_key] = "\n".join(block_lines).rstrip()
        pending_block_key = None
        pending_block_indent = 0
        block_lines = []
        block_content_indent = 0

    i = 0
    while i < len(lines):
        line = lines[i].rstrip()
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if not in_tasks:
            if stripped == "tasks:":
                in_tasks = True
                tasks_indent = indent
            i += 1
            continue

        if pending_block_key is not None:
            if not stripped:
                block_lines.append("")
                i += 1
                continue
            if indent <= pending_block_indent:
                finish_block()
                continue
            if block_content_indent == 0:
                block_content_indent = indent
            if indent < block_content_indent:
                finish_block()
                continue
            block_lines.append(line[block_content_indent:].rstrip())
            i += 1
            continue

        # 任务块一旦回到更外层，就视为结束。
        if stripped and indent <= tasks_indent and not stripped.startswith("-"):
            break
        if not stripped:
            i += 1
            continue

        if stripped.startswith("-"):
            flush_current()
            item = stripped[1:].lstrip()
            if item:
                key, value = _parse_key_value(item)
                if key:
                    if value == "|":
                        pending_block_key = key
                        pending_block_indent = indent
                    else:
                        current[key] = value
            i += 1
            continue

        key, value = _parse_key_value(stripped)
        if key:
            if value == "|":
                pending_block_key = key
                pending_block_indent = indent
            else:
                current[key] = value
        i += 1

    if pending_block_key is not None:
        finish_block()
    flush_current()
    return tasks


def filter_heartbeat_response(text: str, ack_max_chars: int = 300) -> HeartbeatFilterResult:
    """Suppress heartbeat-only acknowledgements while preserving real alerts."""

    normalized = text.strip()
    # 纯 HEARTBEAT_OK 直接吞掉；带边缘 token 的内容按剩余文本长度判断。
    if normalized == ACK_TOKEN:
        return HeartbeatFilterResult(should_deliver=False, text="")
    stripped = normalized
    prefix_removed = False
    suffix_removed = False

    if stripped.startswith(ACK_TOKEN):
        stripped = stripped[len(ACK_TOKEN) :].lstrip()
        prefix_removed = True
    if stripped.endswith(ACK_TOKEN):
        stripped = stripped[: -len(ACK_TOKEN)].rstrip()
        suffix_removed = True

    if not prefix_removed and not suffix_removed:
        return HeartbeatFilterResult(should_deliver=True, text=text)
    if len(stripped) <= ack_max_chars:
        return HeartbeatFilterResult(should_deliver=False, text="")
    return HeartbeatFilterResult(should_deliver=True, text=stripped)


def _parse_key_value(line: str) -> tuple[str, str]:
    if ":" not in line:
        return "", ""
    key, value = line.split(":", 1)
    return key.strip(), _clean_value(value.strip())


def _clean_value(value: str) -> str:
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        return value[1:-1]
    return value


def _build_task(fields: dict[str, str]) -> HeartbeatTask | None:
    name = fields.get("name", "").strip()
    interval = fields.get("interval", "").strip()
    prompt = fields.get("prompt", "").strip()
    if not name and not interval and not prompt:
        return None
    return HeartbeatTask(name=name, interval=interval, prompt=prompt)
