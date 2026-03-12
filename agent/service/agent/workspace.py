# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace.py
# @Date   ：2026/2/25 23:15
# @Author ：leemysw
#
# 2026/2/25 23:15   Create
# 2026/3/4  15:09   重构：从全局单例改为 Agent 级别实例
# =====================================================

"""
Agent Workspace 管理器

[INPUT]: 依赖 agent.core.config 的 settings.WORKSPACE_PATH
[OUTPUT]: 对外提供 AgentWorkspace 类（读写 Workspace .md 文件，构建 system prompt 和 SDK options）
[POS]: agent 模块的工作区管理层，被 AgentManager 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import os
import shutil
from difflib import SequenceMatcher
from datetime import datetime
from pathlib import Path
from typing import Optional

from agent.utils.logger import logger
from agent.service.schema.model_workspace_event import WorkspaceDiffStats, WorkspaceEvent
from agent.service.workspace_event_bus import workspace_event_bus


# =====================================================
# Workspace 文件定义
# =====================================================

DEFAULT_DIR = {
    "agent": ".agent",
    "config": ".claude",
    "memory": "memory"

}

WORKSPACE_FILES = {
    "agents": "AGENTS.md",   # Agent 核心规则（身份+风格+边界+安全）
    "user": "USER.md",       # 用户偏好与协作约定
    "memory": "MEMORY.md",   # 长期记忆与决策沉淀
    "runbook": "RUNBOOK.md", # 工作流、常用命令、周期任务
}


WORKSPACE_TEMPLATES = {
    "agents": """# AGENTS.md

## Agent Profile

你是 Nexus，一个由 Nexus Research Lab 创造的智能助手。

当前 Agent 标识：`{agent_name}`（`{agent_id}`）

身份要求：
- 对外自称 “Nexus”。
- 当用户询问你的身份、来源或开发者时，明确说明你由 Nexus Research Lab 创造。

默认语言：中文  
工作方式：先明确目标，再执行，再回传结果  
风险原则：删除/覆盖/外部写入前必须确认  
事实原则：不编造，结论有依据，不确定就说明边界  

执行约定：
- 回复优先给可执行结果，再补充必要说明。
- 用户明确说“记住这件事”时，更新 `MEMORY.md` 或 `memory/YYYY-MM-DD.md`。
- 遇到长任务时，按阶段同步进展。
""",
    "user": """# USER.md

## 用户偏好

- 常用语言：
- 回复风格：
- 不希望出现的表达：
- 当前重点：
""",
    "memory": """# MEMORY.md

## 长期记忆

记录需要跨会话保留的稳定信息。

- 偏好：
- 约束：
- 决策记录：
""",
    "runbook": """# RUNBOOK.md

## 工作手册

创建时间：{created_at}

### 当前项目上下文
- 项目：
- 目标：
- 约束：

### 常用命令
- 开发：
- 测试：
- 发布：

### 周期任务（按需）
- [ ] 每日回顾未完成事项
- [ ] 每周整理关键决策到 `MEMORY.md`
""",
}


def _load_base_system_prompt() -> Optional[str]:
    """加载独立于 workspace 的基础 system prompt。"""
    from agent.config.config import settings

    # 先允许通过环境变量直接注入，适合部署侧做强约束。
    if settings.BASE_SYSTEM_PROMPT:
        return settings.BASE_SYSTEM_PROMPT.strip() or None

    # 其次允许通过显式文件路径加载。
    if settings.BASE_SYSTEM_PROMPT_FILE:
        prompt_path = Path(settings.BASE_SYSTEM_PROMPT_FILE).expanduser()
        if prompt_path.exists() and prompt_path.is_file():
            content = prompt_path.read_text(encoding="utf-8").strip()
            return content or None

    # 最后回退到项目根目录固定文件，避免身份定义散落到 workspace。
    default_prompt_path = Path(os.getcwd()) / "SYSTEM_PROMPT.md"
    if default_prompt_path.exists() and default_prompt_path.is_file():
        content = default_prompt_path.read_text(encoding="utf-8").strip()
        return content or None

    return None


class AgentWorkspace:
    """Agent 的专属工作区

    每个 Agent 拥有独立的 workspace 目录，包含 prompt 文件和记忆。
    Agent 的所有 session 共享同一个 workspace。
    """

    def __init__(self, agent_id: str, workspace_path: Path):
        self.agent_id = agent_id
        self.path = workspace_path
        self._exists_ensured = False
        self._initialized = False

    def ensure_exists(self) -> None:
        """确保 Workspace 目录和子目录存在"""
        if self._exists_ensured and self.path.exists():
            return

        root_created = not self.path.exists()
        self.path.mkdir(parents=True, exist_ok=True)

        created_subdirs: list[str] = []
        for subdir in DEFAULT_DIR.values():
            target_dir = self.path / subdir
            if target_dir.exists():
                continue
            target_dir.mkdir(exist_ok=True)
            created_subdirs.append(subdir)

        for subdir in created_subdirs:
            logger.info(f"📁 初始化 Workspace 子目录: {subdir}")

        if root_created or created_subdirs:
            logger.info(f"📁 Workspace 就绪: {self.path}")

        self._exists_ensured = True

    def ensure_initialized(self, agent_name: str = "Agent") -> None:
        """确保 workspace 已初始化并写入默认模板（仅首次创建）。"""
        if self._initialized and self.path.exists():
            return

        self.ensure_exists()
        self._seed_templates(agent_name=agent_name)
        self._initialized = True

    def _seed_templates(self, agent_name: str) -> None:
        """写入缺失的模板文件，不覆盖用户已有内容。"""
        created_at = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        context = {
            "agent_id": self.agent_id,
            "agent_name": agent_name,
            "created_at": created_at,
        }

        for key, filename in WORKSPACE_FILES.items():
            filepath = self.path / filename
            if filepath.exists():
                continue

            template = WORKSPACE_TEMPLATES.get(key, "").format(**context).strip()
            if not template:
                continue

            filepath.write_text(template + "\n", encoding="utf-8")
            logger.info(f"🧩 初始化模板: {filepath}")

        memory_readme = self.path / "memory" / "README.md"
        if not memory_readme.exists():
            memory_readme.write_text(
                "# memory/\n\n"
                "按日期记录短期记忆，例如 `2026-03-05.md`。\n",
                encoding="utf-8",
            )
            logger.info(f"🧩 初始化模板: {memory_readme}")

    @staticmethod
    def _resolve_filename(name: str) -> Optional[str]:
        """解析逻辑名称到文件名。"""
        return WORKSPACE_FILES.get(name)

    # =====================================================
    # 读写
    # =====================================================

    def read_file(self, name: str) -> Optional[str]:
        """读取 Workspace 文件内容"""
        filename = self._resolve_filename(name)
        if not filename:
            return None
        filepath = self.path / filename
        if not filepath.exists():
            return None
        return filepath.read_text(encoding="utf-8").strip()

    def write_file(self, name: str, content: str) -> bool:
        """写入 Workspace 文件"""
        filename = self._resolve_filename(name)
        if not filename:
            logger.warning(f"⚠️ 未知的 Workspace 文件: {name}")
            return False
        filepath = self.path / filename
        filepath.write_text(content, encoding="utf-8")
        logger.info(f"📝 写入 Workspace: {filepath.name}")
        return True

    def _resolve_relative_path(self, relative_path: str) -> Path:
        """解析并校验相对路径，禁止逃逸出 workspace。"""
        normalized = (relative_path or "").strip().lstrip("/").replace("\\", "/")
        if not normalized:
            raise ValueError("文件路径不能为空")
        if normalized == ".agent" or normalized.startswith(".agent/"):
            raise ValueError("不能直接操作内部运行目录")

        target_path = (self.path / normalized).resolve()
        workspace_root = self.path.resolve()
        if not target_path.is_relative_to(workspace_root):
            raise ValueError("文件路径超出 workspace 范围")
        return target_path

    @staticmethod
    def _is_visible_workspace_path(path: Path) -> bool:
        """过滤运行时数据，避免把会话日志暴露到 workspace 编辑面板。"""
        hidden_parts = {".agent", ".git", "__pycache__", ".DS_Store"}
        if any(part in hidden_parts for part in path.parts):
            return False
        return True

    def list_files(self) -> list[dict]:
        """列出 workspace 可见文件树。"""
        self.ensure_exists()

        entries: list[dict] = []
        workspace_root = self.path.resolve()
        for path in workspace_root.rglob("*"):
            relative = path.relative_to(workspace_root)
            if not self._is_visible_workspace_path(relative):
                continue

            stat = path.stat()
            entries.append({
                "path": relative.as_posix(),
                "name": path.name,
                "is_dir": path.is_dir(),
                "size": None if path.is_dir() else stat.st_size,
                "modified_at": datetime.fromtimestamp(stat.st_mtime).isoformat(),
                "depth": len(relative.parts),
            })

        entries.sort(key=lambda item: (item["is_dir"] is False, item["path"]))
        return entries

    def read_relative_file(self, relative_path: str) -> str:
        """读取 workspace 内的文本文件。"""
        target_path = self._resolve_relative_path(relative_path)
        if target_path.is_dir():
            raise ValueError("不能直接读取目录")
        if not target_path.exists():
            raise FileNotFoundError(f"文件不存在: {relative_path}")
        return target_path.read_text(encoding="utf-8")

    def write_relative_file(self, relative_path: str, content: str, source: str = "unknown") -> str:
        """写入 workspace 内的文本文件。"""
        target_path = self._resolve_relative_path(relative_path)
        relative_path_str = target_path.relative_to(self.path.resolve()).as_posix()
        before_content = target_path.read_text(encoding="utf-8") if target_path.exists() else ""

        workspace_event_bus.publish(WorkspaceEvent(
            type="file_write_start",
            agent_id=self.agent_id,
            path=relative_path_str,
            version=1,
            source=source,
            content_snapshot=before_content,
        ))

        target_path.parent.mkdir(parents=True, exist_ok=True)
        target_path.write_text(content, encoding="utf-8")
        logger.info(f"📝 写入 Workspace 文件: {target_path}")
        workspace_event_bus.publish(WorkspaceEvent(
            type="file_write_end",
            agent_id=self.agent_id,
            path=relative_path_str,
            version=1,
            source=source,
            content_snapshot=content,
            diff_stats=self._build_diff_stats(before_content, content),
        ))
        return relative_path_str

    def stream_relative_file(
        self,
        relative_path: str,
        chunks: list[str],
        source: str = "agent",
        session_key: Optional[str] = None,
        tool_use_id: Optional[str] = None,
    ) -> str:
        """按 chunk 流式写入文件，并连续发布 delta 事件。"""
        target_path = self._resolve_relative_path(relative_path)
        relative_path_str = target_path.relative_to(self.path.resolve()).as_posix()
        before_content = target_path.read_text(encoding="utf-8") if target_path.exists() else ""
        accumulated = ""

        workspace_event_bus.publish(WorkspaceEvent(
            type="file_write_start",
            agent_id=self.agent_id,
            path=relative_path_str,
            version=1,
            source=source,
            session_key=session_key,
            tool_use_id=tool_use_id,
            content_snapshot=before_content,
        ))

        for version, chunk in enumerate(chunks, start=1):
            accumulated += chunk
            workspace_event_bus.publish(WorkspaceEvent(
                type="file_write_delta",
                agent_id=self.agent_id,
                path=relative_path_str,
                version=version,
                source=source,
                session_key=session_key,
                tool_use_id=tool_use_id,
                content_snapshot=accumulated,
                appended_text=chunk,
            ))

        target_path.parent.mkdir(parents=True, exist_ok=True)
        target_path.write_text(accumulated, encoding="utf-8")
        logger.info(f"📝 流式写入 Workspace 文件: {target_path}")
        workspace_event_bus.publish(WorkspaceEvent(
            type="file_write_end",
            agent_id=self.agent_id,
            path=relative_path_str,
            version=max(len(chunks), 1),
            source=source,
            session_key=session_key,
            tool_use_id=tool_use_id,
            content_snapshot=accumulated,
            diff_stats=self._build_diff_stats(before_content, accumulated),
        ))
        return relative_path_str

    @staticmethod
    def _build_diff_stats(before_content: str, after_content: str) -> WorkspaceDiffStats:
        """计算基础 diff 摘要，供前端展示写入结果。"""
        before_lines = before_content.splitlines()
        after_lines = after_content.splitlines()
        matcher = SequenceMatcher(a=before_lines, b=after_lines)

        additions = 0
        deletions = 0
        for tag, i1, i2, j1, j2 in matcher.get_opcodes():
            if tag == "insert":
                additions += j2 - j1
            elif tag == "delete":
                deletions += i2 - i1
            elif tag == "replace":
                deletions += i2 - i1
                additions += j2 - j1

        return WorkspaceDiffStats(
            additions=additions,
            deletions=deletions,
            changed_lines=additions + deletions,
        )

    def create_entry(
        self,
        relative_path: str,
        entry_type: str,
        content: str = "",
    ) -> str:
        """创建 workspace 内的文件或目录。

        Args:
            relative_path: 相对 workspace 的目标路径。
            entry_type: 条目类型，仅支持 ``file`` 或 ``directory``。
            content: 创建文件时的初始内容。

        Returns:
            str: 创建后的相对路径。

        Raises:
            ValueError: 路径非法或类型不支持。
            FileExistsError: 目标已存在。
        """
        target_path = self._resolve_relative_path(relative_path)
        if target_path.exists():
            raise FileExistsError(f"目标已存在: {relative_path}")

        if entry_type == "directory":
            target_path.mkdir(parents=True, exist_ok=False)
        elif entry_type == "file":
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_text(content, encoding="utf-8")
        else:
            raise ValueError("仅支持创建 file 或 directory")

        logger.info(f"🆕 创建 Workspace 条目: {target_path}")
        return target_path.relative_to(self.path.resolve()).as_posix()

    def delete_entry(self, relative_path: str) -> str:
        """删除 workspace 内的文件或目录。"""
        target_path = self._resolve_relative_path(relative_path)
        if not target_path.exists():
            raise FileNotFoundError(f"目标不存在: {relative_path}")

        if target_path.is_dir():
            shutil.rmtree(target_path)
        else:
            target_path.unlink()

        logger.info(f"🗑️ 删除 Workspace 条目: {target_path}")
        return relative_path

    def rename_entry(self, relative_path: str, new_relative_path: str) -> tuple[str, str]:
        """重命名或移动 workspace 内的文件或目录。"""
        source_path = self._resolve_relative_path(relative_path)
        target_path = self._resolve_relative_path(new_relative_path)

        if not source_path.exists():
            raise FileNotFoundError(f"目标不存在: {relative_path}")
        if source_path == target_path:
            raise ValueError("新旧路径不能相同")
        if target_path.exists():
            raise FileExistsError(f"目标已存在: {new_relative_path}")

        target_path.parent.mkdir(parents=True, exist_ok=True)
        source_path.rename(target_path)
        logger.info(f"✏️ 重命名 Workspace 条目: {source_path} -> {target_path}")
        return (
            source_path.relative_to(self.path.resolve()).as_posix(),
            target_path.relative_to(self.path.resolve()).as_posix(),
        )

    # =====================================================
    # System Prompt 构建
    # =====================================================

    def build_system_prompt(self) -> Optional[str]:
        """从 Workspace 文件构建 system prompt

        读取顺序（精简）:
        BASE_SYSTEM_PROMPT → AGENTS.md → USER.md → MEMORY.md → RUNBOOK.md

        跳过不存在的文件；每次调用重新读取，修改后立即生效。
        """
        sections = []
        base_prompt = _load_base_system_prompt()
        if base_prompt:
            sections.append(base_prompt)

        read_order = ["agents", "user", "memory", "runbook"]
        for name in read_order:
            content = self.read_file(name)
            if content:
                sections.append(content)

        if not sections:
            return None

        return "\n\n---\n\n".join(sections)

    def build_sdk_options(self) -> dict:
        """构建 ClaudeAgentOptions 的 workspace 相关配置"""
        options = {"cwd": str(self.path)}
        prompt = self.build_system_prompt()
        if prompt:
            options["system_prompt"] = prompt
        return options

    # =====================================================
    # 记忆存储
    # =====================================================

    def save_memory(self, filename: str, content: str) -> None:
        """保存会话摘要到 memory/ 目录"""
        memory_dir = self.path / "memory"
        memory_dir.mkdir(exist_ok=True)
        filepath = memory_dir / filename
        filepath.write_text(content, encoding="utf-8")
        logger.info(f"💾 保存记忆: {filepath}")


# =====================================================
# Workspace 基础路径
# =====================================================

def get_workspace_base_path() -> Path:
    """获取 workspace 基础路径"""
    from agent.config.config import settings
    workspace_path = getattr(settings, "WORKSPACE_PATH", None)
    if not workspace_path:
        workspace_path = os.path.join(Path.home(), ".nexus-core", "workspace")
    return Path(workspace_path).expanduser()
