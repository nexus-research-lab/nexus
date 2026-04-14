# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：command.py
# @Date   ：2026/04/01 16:03
# @Author ：leemysw
# 2026/04/01 16:03   Create
# =====================================================

"""主智能体 Typer CLI 命令注册。"""
from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Annotated, Any

import typer

from agent.schema.model_main_agent_cli import (
    AddRoomMemberCommand,
    CreateAgentCommand,
    CreateRoomCommand,
    CreateWorkspaceEntryCommand,
    DeleteAgentCommand,
    DeleteRoomCommand,
    DeleteWorkspaceEntryCommand,
    GetAgentCommand,
    GetAgentSessionsCommand,
    GetAgentSkillsCommand,
    GetRoomCommand,
    GetRoomContextsCommand,
    InstallSkillCommand,
    ListAgentsCommand,
    ListRoomsCommand,
    ListWorkspaceFilesCommand,
    ReadWorkspaceFileCommand,
    RemoveRoomMemberCommand,
    RenameWorkspaceEntryCommand,
    UninstallSkillCommand,
    UpdateRoomCommand,
    UpdateWorkspaceFileCommand,
    ValidateAgentNameCommand,
)
from agent.service.agent.main_agent_profile import MainAgentProfile

ServiceCall = Callable[["MainAgentOrchestrationService"], Awaitable[Any]]
CommandRunner = Callable[[ServiceCall], None]
AgentIdsParser = Callable[[str], list[str]]
OutputConfigurator = Callable[..., None]


def build_typer_app(
    run_service_call: CommandRunner,
    parse_agent_ids: AgentIdsParser,
    configure_output: OutputConfigurator,
) -> typer.Typer:
    """构建 Typer 应用。"""
    app = typer.Typer(
        name=f"{MainAgentProfile.display_name()}_orchestration_cli",
        help=f"供{MainAgentProfile.display_label()}调用的协作编排 CLI",
        no_args_is_help=True,
        add_completion=False,
        rich_markup_mode=None,
        pretty_exceptions_enable=False,
    )

    @app.callback()
    def main(
            verbose: Annotated[bool, typer.Option("--verbose", help="输出过程日志")] = False,
            pretty: Annotated[bool, typer.Option("--pretty", help="格式化 JSON 输出")] = False,
    ) -> None:
        """配置 CLI 输出模式。"""
        configure_output(verbose=verbose, pretty=pretty)

    @app.command("list_agents", help="列出成员 agent")
    def list_agents(
            include_main: Annotated[bool, typer.Option(
                "--include-main", "--include_main", help=f"是否包含{MainAgentProfile.display_label()}"
            )] = False,
    ) -> None:
        command = ListAgentsCommand(include_main=include_main)
        run_service_call(lambda service: service.list_agents(include_main=command.include_main))

    @app.command("validate_agent_name", help="校验 agent 名称")
    def validate_agent_name(
            name: Annotated[str, typer.Option("--name", help="待校验名称")],
    ) -> None:
        command = ValidateAgentNameCommand(name=name)
        run_service_call(lambda service: service.validate_agent_name(command.name))

    @app.command("create_agent", help="创建成员 agent")
    def create_agent(
            name: Annotated[str, typer.Option("--name", help="成员名称")],
            provider: Annotated[str | None, typer.Option("--provider", help="provider 名")] = None,
    ) -> None:
        command = CreateAgentCommand(name=name, provider=provider)
        run_service_call(lambda service: service.create_agent(name=command.name, provider=command.provider))

    @app.command("get_agent", help="读取成员详情")
    def get_agent(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = GetAgentCommand(agent_id=agent_id)
        run_service_call(lambda service: service.get_agent(command.agent_id))

    @app.command("delete_agent", help="删除成员 agent")
    def delete_agent(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = DeleteAgentCommand(agent_id=agent_id)
        run_service_call(lambda service: service.delete_agent(command.agent_id))

    @app.command("get_agent_sessions", help="读取成员会话")
    def get_agent_sessions(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = GetAgentSessionsCommand(agent_id=agent_id)
        run_service_call(lambda service: service.get_agent_sessions(command.agent_id))

    @app.command("list_rooms", help="列出最近 room")
    def list_rooms(
            limit: Annotated[int, typer.Option("--limit", help="数量上限")] = 20,
    ) -> None:
        command = ListRoomsCommand(limit=limit)
        run_service_call(lambda service: service.list_rooms(limit=command.limit))

    @app.command("get_room", help="读取 room")
    def get_room(
            room_id: Annotated[str, typer.Option("--room-id", "--room_id", help="room_id")],
    ) -> None:
        command = GetRoomCommand(room_id=room_id)
        run_service_call(lambda service: service.get_room(command.room_id))

    @app.command("get_room_contexts", help="读取 room 上下文")
    def get_room_contexts(
            room_id: Annotated[str, typer.Option("--room-id", "--room_id", help="room_id")],
    ) -> None:
        command = GetRoomContextsCommand(room_id=room_id)
        run_service_call(lambda service: service.get_room_contexts(command.room_id))

    @app.command("create_room", help="创建 room")
    def create_room(
            agent_ids: Annotated[str, typer.Option("--agent-ids", "--agent_ids", help="逗号分隔的 agent_id 列表")],
            name: Annotated[str | None, typer.Option("--name", help="room 名称")] = None,
            title: Annotated[str | None, typer.Option("--title", help="首条对话标题")] = None,
            description: Annotated[str, typer.Option("--description", help="room 描述")] = "",
    ) -> None:
        command = CreateRoomCommand(
            agent_ids=parse_agent_ids(agent_ids),
            name=name,
            title=title,
            description=description,
        )
        run_service_call(
            lambda service: service.create_room(
                agent_ids=command.agent_ids,
                name=command.name,
                title=command.title,
                description=command.description,
            )
        )

    @app.command("update_room", help="更新 room")
    def update_room(
            room_id: Annotated[str, typer.Option("--room-id", "--room_id", help="room_id")],
            name: Annotated[str | None, typer.Option("--name", help="room 名称")] = None,
            title: Annotated[str | None, typer.Option("--title", help="主对话标题")] = None,
            description: Annotated[str | None, typer.Option("--description", help="room 描述")] = None,
    ) -> None:
        command = UpdateRoomCommand(
            room_id=room_id,
            name=name,
            title=title,
            description=description,
        )
        run_service_call(
            lambda service: service.update_room(
                room_id=command.room_id,
                name=command.name,
                title=command.title,
                description=command.description,
            )
        )

    @app.command("add_room_member", help="向 room 追加成员")
    def add_room_member(
            room_id: Annotated[str, typer.Option("--room-id", "--room_id", help="room_id")],
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = AddRoomMemberCommand(room_id=room_id, agent_id=agent_id)
        run_service_call(lambda service: service.add_room_member(command.room_id, command.agent_id))

    @app.command("remove_room_member", help="移除 room 成员")
    def remove_room_member(
            room_id: Annotated[str, typer.Option("--room-id", "--room_id", help="room_id")],
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = RemoveRoomMemberCommand(room_id=room_id, agent_id=agent_id)
        run_service_call(lambda service: service.remove_room_member(command.room_id, command.agent_id))

    @app.command("delete_room", help="删除 room")
    def delete_room(
            room_id: Annotated[str, typer.Option("--room-id", "--room_id", help="room_id")],
    ) -> None:
        command = DeleteRoomCommand(room_id=room_id)
        run_service_call(lambda service: service.delete_room(command.room_id))

    @app.command("list_workspace_files", help="列出工作区文件")
    def list_workspace_files(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = ListWorkspaceFilesCommand(agent_id=agent_id)
        run_service_call(lambda service: service.list_workspace_files(command.agent_id))

    @app.command("read_workspace_file", help="读取工作区文件")
    def read_workspace_file(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            path: Annotated[str, typer.Option("--path", help="工作区相对路径")],
    ) -> None:
        command = ReadWorkspaceFileCommand(agent_id=agent_id, path=path)
        run_service_call(lambda service: service.read_workspace_file(command.agent_id, command.path))

    @app.command("update_workspace_file", help="更新工作区文件")
    def update_workspace_file(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            path: Annotated[str, typer.Option("--path", help="工作区相对路径")],
            content: Annotated[str, typer.Option("--content", help="文件内容")],
    ) -> None:
        command = UpdateWorkspaceFileCommand(agent_id=agent_id, path=path, content=content)
        run_service_call(
            lambda service: service.update_workspace_file(
                agent_id=command.agent_id,
                path=command.path,
                content=command.content,
            )
        )

    @app.command("create_workspace_entry", help="创建工作区条目")
    def create_workspace_entry(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            path: Annotated[str, typer.Option("--path", help="工作区相对路径")],
            entry_type: Annotated[str, typer.Option("--entry-type", "--entry_type", help="file 或 dir")],
            content: Annotated[str, typer.Option("--content", help="文件内容")] = "",
    ) -> None:
        command = CreateWorkspaceEntryCommand(
            agent_id=agent_id,
            path=path,
            entry_type=entry_type,
            content=content,
        )
        run_service_call(
            lambda service: service.create_workspace_entry(
                agent_id=command.agent_id,
                path=command.path,
                entry_type=command.entry_type,
                content=command.content,
            )
        )

    @app.command("rename_workspace_entry", help="重命名工作区条目")
    def rename_workspace_entry(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            path: Annotated[str, typer.Option("--path", help="原路径")],
            new_path: Annotated[str, typer.Option("--new-path", "--new_path", help="新路径")],
    ) -> None:
        command = RenameWorkspaceEntryCommand(agent_id=agent_id, path=path, new_path=new_path)
        run_service_call(
            lambda service: service.rename_workspace_entry(
                agent_id=command.agent_id,
                path=command.path,
                new_path=command.new_path,
            )
        )

    @app.command("delete_workspace_entry", help="删除工作区条目")
    def delete_workspace_entry(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            path: Annotated[str, typer.Option("--path", help="工作区相对路径")],
    ) -> None:
        command = DeleteWorkspaceEntryCommand(agent_id=agent_id, path=path)
        run_service_call(lambda service: service.delete_workspace_entry(command.agent_id, command.path))

    @app.command("list_skills", help="列出可安装技能")
    def list_skills() -> None:
        run_service_call(lambda service: service.list_skills())

    @app.command("get_agent_skills", help="读取成员技能")
    def get_agent_skills(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
    ) -> None:
        command = GetAgentSkillsCommand(agent_id=agent_id)
        run_service_call(lambda service: service.get_agent_skills(command.agent_id))

    @app.command("install_skill", help="安装技能")
    def install_skill(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            skill_name: Annotated[str, typer.Option("--skill-name", "--skill_name", help="skill 名称")],
    ) -> None:
        command = InstallSkillCommand(agent_id=agent_id, skill_name=skill_name)
        run_service_call(lambda service: service.install_skill(command.agent_id, command.skill_name))

    @app.command("uninstall_skill", help="卸载技能")
    def uninstall_skill(
            agent_id: Annotated[str, typer.Option("--agent-id", "--agent_id", help="agent_id")],
            skill_name: Annotated[str, typer.Option("--skill-name", "--skill_name", help="skill 名称")],
    ) -> None:
        command = UninstallSkillCommand(agent_id=agent_id, skill_name=skill_name)
        run_service_call(
            lambda service: service.uninstall_skill(command.agent_id, command.skill_name)
        )

    return app
