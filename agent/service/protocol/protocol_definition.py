# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：protocol_definition.py
# @Date   ：2026/3/25 21:10
# @Author ：OpenAI
# =====================================================

"""Protocol Definition 抽象与示例协议。"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional, Protocol

from agent.schema.model_chat_persistence import MemberRecord
from agent.schema.model_protocol import (
    ActionRequestRecord,
    ActionSubmissionRecord,
    ChannelAggregate,
    ProtocolDefinitionRecord,
    ProtocolRunRecord,
)


@dataclass
class ChannelBlueprint:
    """频道蓝图。"""

    slug: str
    name: str
    channel_type: str
    visibility: str
    topic: str = ""
    position: int = 0
    member_agent_ids: list[str] = field(default_factory=list)
    include_user: bool = False
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class ActionRequestBlueprint:
    """动作请求蓝图。"""

    action_type: str
    phase_name: str
    channel_slug: Optional[str] = None
    turn_key: Optional[str] = None
    requested_by_agent_id: Optional[str] = None
    allowed_actor_agent_ids: list[str] = field(default_factory=list)
    audience_agent_ids: list[str] = field(default_factory=list)
    input_schema: dict[str, Any] = field(default_factory=dict)
    target_scope: dict[str, Any] = field(default_factory=dict)
    prompt_text: Optional[str] = None
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class SnapshotBlueprint:
    """事件快照蓝图。"""

    event_type: str
    phase_name: str
    channel_slug: Optional[str] = None
    actor_agent_id: Optional[str] = None
    visibility: str = "public"
    audience_agent_ids: list[str] = field(default_factory=list)
    headline: Optional[str] = None
    body: Optional[str] = None
    state: dict[str, Any] = field(default_factory=dict)
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class PhasePlan:
    """进入/归约阶段后的计划结果。"""

    snapshots: list[SnapshotBlueprint] = field(default_factory=list)
    action_requests: list[ActionRequestBlueprint] = field(default_factory=list)
    state_patch: dict[str, Any] = field(default_factory=dict)
    next_phase: Optional[str] = None
    current_turn_key: Optional[str] = None
    status_override: Optional[str] = None


class ProtocolDefinition(Protocol):
    """协议定义接口。"""

    slug: str
    name: str
    description: str
    version: int
    coordinator_mode: str
    phases: list[str]
    channel_policy: list[dict[str, Any]]
    turn_policy: dict[str, Any]
    action_schemas: dict[str, Any]
    visibility_resolver: str
    completion_rule: dict[str, Any]

    def to_record(self) -> ProtocolDefinitionRecord: ...

    def build_initial_state(self, agent_ids: list[str]) -> dict[str, Any]: ...

    def build_channels(
        self,
        members: list[MemberRecord],
        state: dict[str, Any],
    ) -> list[ChannelBlueprint]: ...

    def on_phase_enter(
        self,
        run: ProtocolRunRecord,
        members: list[MemberRecord],
        channels: dict[str, ChannelAggregate],
    ) -> PhasePlan: ...

    def reduce_phase(
        self,
        run: ProtocolRunRecord,
        members: list[MemberRecord],
        channels: dict[str, ChannelAggregate],
        requests: list[ActionRequestRecord],
        submissions: list[ActionSubmissionRecord],
    ) -> PhasePlan: ...


class WerewolfDemoProtocolDefinition:
    """Protocol Room 示例规则：狼人杀式多频道协作。"""

    slug = "werewolf_demo"
    name = "Werewolf Demo"
    description = "A demo protocol showing public, scoped, direct, and system collaboration channels."
    version = 1
    coordinator_mode = "main_agent"
    phases = [
        "setup",
        "night",
        "day_announcement",
        "day_speeches",
        "voting",
        "game_over",
    ]
    visibility_resolver = "membership_or_observer_redaction"
    turn_policy = {
        "night": "parallel_secret_actions",
        "day_speeches": "single_speaker_turns",
        "voting": "parallel_public_vote",
    }
    action_schemas = {
        "kill_target": {
            "fields": [
                {
                    "name": "target_agent_id",
                    "type": "agent_id",
                    "label": "Night target",
                    "required": True,
                }
            ]
        },
        "inspect_target": {
            "fields": [
                {
                    "name": "target_agent_id",
                    "type": "agent_id",
                    "label": "Inspect target",
                    "required": True,
                }
            ]
        },
        "save_target": {
            "fields": [
                {
                    "name": "target_agent_id",
                    "type": "agent_id",
                    "label": "Protect target",
                    "required": True,
                }
            ]
        },
        "speak": {
            "fields": [
                {
                    "name": "content",
                    "type": "text",
                    "label": "Speech",
                    "required": True,
                    "multiline": True,
                }
            ]
        },
        "vote_target": {
            "fields": [
                {
                    "name": "target_agent_id",
                    "type": "agent_id",
                    "label": "Vote target",
                    "required": True,
                }
            ]
        },
        "signal_ready": {"fields": []},
    }
    completion_rule = {"villagers_win_if_no_wolves": True, "wolves_win_if_majority": True}
    channel_policy = [
        {"slug": "public-main", "channel_type": "public", "visibility": "public"},
        {"slug": "system-broadcast", "channel_type": "system", "visibility": "system"},
        {"slug": "wolves-den", "channel_type": "scoped", "visibility": "scoped"},
        {"slug_pattern": "direct-{agent_id}", "channel_type": "direct", "visibility": "direct"},
    ]

    def to_record(self) -> ProtocolDefinitionRecord:
        return ProtocolDefinitionRecord(
            id=f"{self.slug}_v{self.version}",
            slug=self.slug,
            name=self.name,
            description=self.description,
            version=self.version,
            coordinator_mode=self.coordinator_mode,
            phases=self.phases,
            channel_policy=self.channel_policy,
            turn_policy=self.turn_policy,
            action_schemas=self.action_schemas,
            visibility_resolver=self.visibility_resolver,
            completion_rule=self.completion_rule,
        )

    def build_initial_state(self, agent_ids: list[str]) -> dict[str, Any]:
        roles: dict[str, str] = {}
        assigned_roles: list[str] = []
        if len(agent_ids) >= 2:
            assigned_roles.append("wolf")
        if len(agent_ids) >= 3:
            assigned_roles.append("seer")
        if len(agent_ids) >= 4:
            assigned_roles.append("healer")
        while len(assigned_roles) < len(agent_ids):
            assigned_roles.append("villager")

        for agent_id, role in zip(agent_ids, assigned_roles):
            roles[agent_id] = role

        return {
            "day": 1,
            "roles": roles,
            "alive_agent_ids": list(agent_ids),
            "eliminated_agent_ids": [],
            "spoken_agent_ids": [],
            "last_night_result": {},
            "last_vote_result": {},
            "winner": None,
            "vote_history": [],
        }

    def build_channels(
        self,
        members: list[MemberRecord],
        state: dict[str, Any],
    ) -> list[ChannelBlueprint]:
        agent_ids = self._agent_ids(members)
        wolves = [agent_id for agent_id, role in state.get("roles", {}).items() if role == "wolf"]
        channels = [
            ChannelBlueprint(
                slug="public-main",
                name="Public Stage",
                channel_type="public",
                visibility="public",
                topic="Shared public discussion timeline",
                member_agent_ids=agent_ids,
                include_user=True,
                position=0,
            ),
            ChannelBlueprint(
                slug="system-broadcast",
                name="Coordinator Broadcast",
                channel_type="system",
                visibility="system",
                topic="Coordinator events and rulings",
                member_agent_ids=agent_ids,
                include_user=True,
                position=1,
            ),
        ]

        if wolves:
            channels.append(
                ChannelBlueprint(
                    slug="wolves-den",
                    name="Wolves Den",
                    channel_type="scoped",
                    visibility="scoped",
                    topic="Secret wolf coordination channel",
                    member_agent_ids=wolves,
                    position=2,
                    metadata={"role_group": "wolf"},
                )
            )

        for offset, agent_id in enumerate(agent_ids, start=3):
            role = state.get("roles", {}).get(agent_id, "member")
            channels.append(
                ChannelBlueprint(
                    slug=f"direct-{agent_id}",
                    name=f"Direct · {agent_id}",
                    channel_type="direct",
                    visibility="direct",
                    topic=f"Private direct channel for {agent_id}",
                    member_agent_ids=[agent_id],
                    position=offset,
                    metadata={"owner_agent_id": agent_id, "role": role},
                )
            )

        return channels

    def on_phase_enter(
        self,
        run: ProtocolRunRecord,
        members: list[MemberRecord],
        channels: dict[str, ChannelAggregate],
    ) -> PhasePlan:
        state = run.state
        alive_agent_ids = self._alive_agent_ids(state)
        roles = state.get("roles", {})
        day = int(state.get("day") or 1)

        if run.current_phase == "night":
            snapshots = [
                SnapshotBlueprint(
                    event_type="phase_started",
                    phase_name="night",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline=f"Night {day} has begun",
                    body="Private coordination is open. Public discussion is paused until dawn.",
                    metadata={"message_kind": "phase_event"},
                )
            ]
            requests: list[ActionRequestBlueprint] = []
            wolf_agents = [agent_id for agent_id in alive_agent_ids if roles.get(agent_id) == "wolf"]
            wolf_targets = [agent_id for agent_id in alive_agent_ids if roles.get(agent_id) != "wolf"]
            if wolf_agents and wolf_targets:
                requests.append(
                    ActionRequestBlueprint(
                        action_type="kill_target",
                        phase_name="night",
                        channel_slug="wolves-den",
                        allowed_actor_agent_ids=wolf_agents,
                        audience_agent_ids=wolf_agents,
                        input_schema=self._schema_with_candidates("kill_target", wolf_targets),
                        target_scope={"candidate_agent_ids": wolf_targets},
                        prompt_text="Agree on a night target for the wolves.",
                    )
                )

            seer_agents = [agent_id for agent_id in alive_agent_ids if roles.get(agent_id) == "seer"]
            if seer_agents:
                seer_targets = [agent_id for agent_id in alive_agent_ids if agent_id != seer_agents[0]]
                if seer_targets:
                    requests.append(
                        ActionRequestBlueprint(
                            action_type="inspect_target",
                            phase_name="night",
                            channel_slug=f"direct-{seer_agents[0]}",
                            allowed_actor_agent_ids=seer_agents,
                            audience_agent_ids=seer_agents,
                            input_schema=self._schema_with_candidates("inspect_target", seer_targets),
                            target_scope={"candidate_agent_ids": seer_targets},
                            prompt_text="Choose one member to inspect before dawn.",
                        )
                    )

            healer_agents = [agent_id for agent_id in alive_agent_ids if roles.get(agent_id) == "healer"]
            if healer_agents:
                requests.append(
                    ActionRequestBlueprint(
                        action_type="save_target",
                        phase_name="night",
                        channel_slug=f"direct-{healer_agents[0]}",
                        allowed_actor_agent_ids=healer_agents,
                        audience_agent_ids=healer_agents,
                        input_schema=self._schema_with_candidates("save_target", alive_agent_ids),
                        target_scope={"candidate_agent_ids": alive_agent_ids},
                        prompt_text="Choose one member to protect tonight.",
                    )
                )

            if not requests:
                snapshots.append(
                    SnapshotBlueprint(
                        event_type="phase_resolved",
                        phase_name="night",
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Night resolved immediately",
                        body="No active night roles remained, so the protocol is moving to dawn.",
                        metadata={"message_kind": "phase_event"},
                    )
                )
                return PhasePlan(snapshots=snapshots, next_phase="day_announcement")

            return PhasePlan(snapshots=snapshots, action_requests=requests)

        if run.current_phase == "day_announcement":
            last_result = state.get("last_night_result", {})
            deaths = last_result.get("deaths", [])
            snapshots = [
                SnapshotBlueprint(
                    event_type="phase_started",
                    phase_name="day_announcement",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline=f"Dawn of Day {day}",
                    body="The coordinator is announcing the public night resolution.",
                    metadata={"message_kind": "phase_event"},
                )
            ]
            if deaths:
                for agent_id in deaths:
                    snapshots.append(
                        SnapshotBlueprint(
                            event_type="verdict",
                            phase_name="day_announcement",
                            channel_slug="public-main",
                            visibility="public",
                            headline=f"{agent_id} did not survive the night",
                            body="The room resumes under visible public pressure.",
                            metadata={"message_kind": "verdict"},
                        )
                    )
            else:
                snapshots.append(
                    SnapshotBlueprint(
                        event_type="verdict",
                        phase_name="day_announcement",
                        channel_slug="public-main",
                        visibility="public",
                        headline="No overnight elimination",
                        body="Dawn breaks without a visible casualty.",
                        metadata={"message_kind": "verdict"},
                    )
                )
            snapshots.append(
                SnapshotBlueprint(
                    event_type="phase_resolved",
                    phase_name="day_announcement",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline="Announcement complete",
                    body="The protocol is moving into sequential public speeches.",
                    metadata={"message_kind": "phase_event"},
                )
            )
            return PhasePlan(
                snapshots=snapshots,
                state_patch={"spoken_agent_ids": []},
                next_phase="day_speeches",
            )

        if run.current_phase == "day_speeches":
            spoken_agent_ids = list(state.get("spoken_agent_ids") or [])
            remaining_speakers = [agent_id for agent_id in alive_agent_ids if agent_id not in spoken_agent_ids]
            snapshots: list[SnapshotBlueprint] = []
            if not spoken_agent_ids:
                snapshots.append(
                    SnapshotBlueprint(
                        event_type="phase_started",
                        phase_name="day_speeches",
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Public speech round opened",
                        body="Alive members now speak one at a time on the public stage.",
                        metadata={"message_kind": "phase_event"},
                    )
                )

            if not remaining_speakers:
                snapshots.append(
                    SnapshotBlueprint(
                        event_type="phase_resolved",
                        phase_name="day_speeches",
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Speech round complete",
                        body="The room is moving into public voting.",
                        metadata={"message_kind": "phase_event"},
                    )
                )
                return PhasePlan(snapshots=snapshots, next_phase="voting")

            speaker_agent_id = remaining_speakers[0]
            snapshots.append(
                SnapshotBlueprint(
                    event_type="turn_opened",
                    phase_name="day_speeches",
                    channel_slug="public-main",
                    visibility="public",
                    actor_agent_id=speaker_agent_id,
                    headline=f"{speaker_agent_id} has the floor",
                    body="A single public speaking turn is now open.",
                    metadata={"message_kind": "turn_event"},
                )
            )
            request = ActionRequestBlueprint(
                action_type="speak",
                phase_name="day_speeches",
                channel_slug="public-main",
                turn_key=speaker_agent_id,
                allowed_actor_agent_ids=[speaker_agent_id],
                audience_agent_ids=alive_agent_ids,
                input_schema=self.action_schemas["speak"],
                prompt_text=f"{speaker_agent_id} should make a concise public statement.",
                metadata={"speaker_agent_id": speaker_agent_id},
            )
            return PhasePlan(
                snapshots=snapshots,
                action_requests=[request],
                current_turn_key=speaker_agent_id,
            )

        if run.current_phase == "voting":
            snapshots = [
                SnapshotBlueprint(
                    event_type="phase_started",
                    phase_name="voting",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline="Voting opened",
                    body="All alive members must submit a public vote target.",
                    metadata={"message_kind": "phase_event"},
                )
            ]
            requests = [
                ActionRequestBlueprint(
                    action_type="vote_target",
                    phase_name="voting",
                    channel_slug="public-main",
                    turn_key=agent_id,
                    allowed_actor_agent_ids=[agent_id],
                    audience_agent_ids=alive_agent_ids,
                    input_schema=self._schema_with_candidates(
                        "vote_target",
                        [candidate for candidate in alive_agent_ids if candidate != agent_id],
                    ),
                    target_scope={"candidate_agent_ids": [candidate for candidate in alive_agent_ids if candidate != agent_id]},
                    prompt_text=f"{agent_id} must choose a public vote target.",
                    metadata={"voter_agent_id": agent_id},
                )
                for agent_id in alive_agent_ids
                if len(alive_agent_ids) > 1
            ]
            if not requests:
                snapshots.append(
                    SnapshotBlueprint(
                        event_type="phase_resolved",
                        phase_name="voting",
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Voting skipped",
                        body="Only one living member remains, so the protocol is moving to completion.",
                        metadata={"message_kind": "phase_event"},
                    )
                )
                return PhasePlan(snapshots=snapshots, next_phase="game_over")
            return PhasePlan(snapshots=snapshots, action_requests=requests)

        if run.current_phase == "game_over":
            winner = state.get("winner") or "no one"
            return PhasePlan(
                snapshots=[
                    SnapshotBlueprint(
                        event_type="phase_started",
                        phase_name="game_over",
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Protocol complete",
                        body="The coordinator is finalizing the run and publishing the result.",
                        metadata={"message_kind": "phase_event"},
                    ),
                    SnapshotBlueprint(
                        event_type="run_completed",
                        phase_name="game_over",
                        channel_slug="public-main",
                        visibility="public",
                        headline=f"Winner: {winner}",
                        body="The demonstration protocol has reached its terminal condition.",
                        metadata={"message_kind": "verdict", "winner": winner},
                    ),
                ],
                status_override="completed",
            )

        return PhasePlan()

    def reduce_phase(
        self,
        run: ProtocolRunRecord,
        members: list[MemberRecord],
        channels: dict[str, ChannelAggregate],
        requests: list[ActionRequestRecord],
        submissions: list[ActionSubmissionRecord],
    ) -> PhasePlan:
        if run.current_phase == "night":
            return self._resolve_night(run, requests, submissions)
        if run.current_phase == "day_speeches":
            return self._resolve_day_speeches(run, requests, submissions)
        if run.current_phase == "voting":
            return self._resolve_voting(run, requests, submissions)
        return PhasePlan()

    def _resolve_night(
        self,
        run: ProtocolRunRecord,
        requests: list[ActionRequestRecord],
        submissions: list[ActionSubmissionRecord],
    ) -> PhasePlan:
        state = run.state
        roles = state.get("roles", {})
        alive_agent_ids = self._alive_agent_ids(state)
        latest_by_request = self._latest_submission_by_request(submissions)
        requests_by_type = {request.action_type: request for request in requests}
        kill_request = requests_by_type.get("kill_target")
        inspect_request = requests_by_type.get("inspect_target")
        save_request = requests_by_type.get("save_target")

        kill_submission = latest_by_request.get(kill_request.id) if kill_request else None
        inspect_submission = latest_by_request.get(inspect_request.id) if inspect_request else None
        save_submission = latest_by_request.get(save_request.id) if save_request else None

        kill_target = str(kill_submission.payload.get("target_agent_id") or "").strip() if kill_submission else ""
        inspect_target = str(inspect_submission.payload.get("target_agent_id") or "").strip() if inspect_submission else ""
        save_target = str(save_submission.payload.get("target_agent_id") or "").strip() if save_submission else ""

        deaths: list[str] = []
        if kill_target and kill_target in alive_agent_ids and kill_target != save_target:
            deaths.append(kill_target)

        next_alive_agent_ids = [agent_id for agent_id in alive_agent_ids if agent_id not in deaths]
        eliminated_agent_ids = list(dict.fromkeys([*state.get("eliminated_agent_ids", []), *deaths]))
        winner = self._resolve_winner(next_alive_agent_ids, roles)

        snapshots = [
            SnapshotBlueprint(
                event_type="phase_resolved",
                phase_name="night",
                channel_slug="system-broadcast",
                visibility="system",
                headline="Night actions resolved",
                body="Private night coordination has been reduced into the next public state.",
                metadata={"message_kind": "phase_event"},
            )
        ]

        if inspect_submission and inspect_submission.actor_agent_id and inspect_target:
            inspected_role = roles.get(inspect_target, "unknown")
            snapshots.append(
                SnapshotBlueprint(
                    event_type="channel_message",
                    phase_name="night",
                    channel_slug=f"direct-{inspect_submission.actor_agent_id}",
                    actor_agent_id=inspect_submission.actor_agent_id,
                    visibility="direct",
                    audience_agent_ids=[inspect_submission.actor_agent_id],
                    headline="Inspection result",
                    body=f"{inspect_target} is aligned as {inspected_role}.",
                    metadata={"message_kind": "result"},
                )
            )

        if save_submission and save_submission.actor_agent_id:
            snapshots.append(
                SnapshotBlueprint(
                    event_type="channel_message",
                    phase_name="night",
                    channel_slug=f"direct-{save_submission.actor_agent_id}",
                    actor_agent_id=save_submission.actor_agent_id,
                    visibility="direct",
                    audience_agent_ids=[save_submission.actor_agent_id],
                    headline="Protection locked",
                    body=f"Protection was aimed at {save_target or 'no one'}.",
                    metadata={"message_kind": "result"},
                )
            )

        return PhasePlan(
            snapshots=snapshots,
            state_patch={
                "alive_agent_ids": next_alive_agent_ids,
                "eliminated_agent_ids": eliminated_agent_ids,
                "last_night_result": {
                    "deaths": deaths,
                    "kill_target": kill_target or None,
                    "saved_target": save_target or None,
                    "inspected_target": inspect_target or None,
                    "inspected_role": roles.get(inspect_target) if inspect_target else None,
                },
                "winner": winner,
            },
            next_phase="game_over" if winner else "day_announcement",
            current_turn_key=None,
        )

    def _resolve_day_speeches(
        self,
        run: ProtocolRunRecord,
        requests: list[ActionRequestRecord],
        submissions: list[ActionSubmissionRecord],
    ) -> PhasePlan:
        state = run.state
        alive_agent_ids = self._alive_agent_ids(state)
        latest_by_request = self._latest_submission_by_request(submissions)
        spoken_agent_ids = list(state.get("spoken_agent_ids") or [])
        current_request = requests[-1] if requests else None
        current_submission = latest_by_request.get(current_request.id) if current_request else None
        speaker_agent_id = current_submission.actor_agent_id if current_submission else run.current_turn_key

        if speaker_agent_id:
            spoken_agent_ids.append(speaker_agent_id)
            spoken_agent_ids = list(dict.fromkeys(spoken_agent_ids))

        snapshots: list[SnapshotBlueprint] = []
        speech_content = str(current_submission.payload.get("content") or "").strip() if current_submission else ""
        if speaker_agent_id:
            snapshots.append(
                SnapshotBlueprint(
                    event_type="channel_message",
                    phase_name="day_speeches",
                    channel_slug="public-main",
                    actor_agent_id=speaker_agent_id,
                    visibility="public",
                    headline=f"{speaker_agent_id} spoke to the room",
                    body=speech_content or f"{speaker_agent_id} yielded the floor without speaking.",
                    metadata={"message_kind": "speech"},
                )
            )

        remaining_speakers = [agent_id for agent_id in alive_agent_ids if agent_id not in spoken_agent_ids]
        if not remaining_speakers:
            snapshots.append(
                SnapshotBlueprint(
                    event_type="phase_resolved",
                    phase_name="day_speeches",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline="Public speeches complete",
                    body="All living members have spoken. Voting will begin next.",
                    metadata={"message_kind": "phase_event"},
                )
            )
            return PhasePlan(
                snapshots=snapshots,
                state_patch={"spoken_agent_ids": spoken_agent_ids},
                next_phase="voting",
                current_turn_key=None,
            )

        next_speaker = remaining_speakers[0]
        snapshots.append(
            SnapshotBlueprint(
                event_type="turn_opened",
                phase_name="day_speeches",
                channel_slug="public-main",
                actor_agent_id=next_speaker,
                visibility="public",
                headline=f"{next_speaker} has the floor",
                body="The next public speaker is now active.",
                metadata={"message_kind": "turn_event"},
            )
        )
        return PhasePlan(
            snapshots=snapshots,
            action_requests=[
                ActionRequestBlueprint(
                    action_type="speak",
                    phase_name="day_speeches",
                    channel_slug="public-main",
                    turn_key=next_speaker,
                    allowed_actor_agent_ids=[next_speaker],
                    audience_agent_ids=alive_agent_ids,
                    input_schema=self.action_schemas["speak"],
                    prompt_text=f"{next_speaker} should make a concise public statement.",
                    metadata={"speaker_agent_id": next_speaker},
                )
            ],
            state_patch={"spoken_agent_ids": spoken_agent_ids},
            current_turn_key=next_speaker,
        )

    def _resolve_voting(
        self,
        run: ProtocolRunRecord,
        requests: list[ActionRequestRecord],
        submissions: list[ActionSubmissionRecord],
    ) -> PhasePlan:
        state = run.state
        roles = state.get("roles", {})
        alive_agent_ids = self._alive_agent_ids(state)
        latest_by_request = self._latest_submission_by_request(submissions)

        vote_tally: dict[str, int] = {}
        vote_map: dict[str, str] = {}
        for request in requests:
            submission = latest_by_request.get(request.id)
            if submission is None or not submission.actor_agent_id:
                continue
            target_agent_id = str(submission.payload.get("target_agent_id") or "").strip()
            if not target_agent_id:
                continue
            vote_map[submission.actor_agent_id] = target_agent_id
            vote_tally[target_agent_id] = vote_tally.get(target_agent_id, 0) + 1

        top_target: Optional[str] = None
        if vote_tally:
            top_count = max(vote_tally.values())
            leaders = sorted([agent_id for agent_id, count in vote_tally.items() if count == top_count])
            if len(leaders) == 1:
                top_target = leaders[0]

        next_alive_agent_ids = list(alive_agent_ids)
        eliminated_agent_ids = list(state.get("eliminated_agent_ids") or [])
        if top_target and top_target in next_alive_agent_ids:
            next_alive_agent_ids = [agent_id for agent_id in next_alive_agent_ids if agent_id != top_target]
            eliminated_agent_ids = list(dict.fromkeys([*eliminated_agent_ids, top_target]))

        winner = self._resolve_winner(next_alive_agent_ids, roles)
        body = (
            f"{top_target} was eliminated by public vote."
            if top_target
            else "The vote ended in a tie, so no one was eliminated."
        )
        snapshots = [
            SnapshotBlueprint(
                event_type="phase_resolved",
                phase_name="voting",
                channel_slug="public-main",
                visibility="public",
                headline="Voting resolved",
                body=body,
                metadata={"message_kind": "verdict", "vote_tally": vote_tally},
            )
        ]

        return PhasePlan(
            snapshots=snapshots,
            state_patch={
                "alive_agent_ids": next_alive_agent_ids,
                "eliminated_agent_ids": eliminated_agent_ids,
                "last_vote_result": {"target": top_target, "vote_tally": vote_tally, "votes": vote_map},
                "vote_history": [*state.get("vote_history", []), {"day": state.get("day", 1), "votes": vote_map}],
                "spoken_agent_ids": [],
                "winner": winner,
                "day": int(state.get("day") or 1) + (0 if winner else 1),
            },
            next_phase="game_over" if winner else "night",
            current_turn_key=None,
        )

    def _schema_with_candidates(
        self,
        action_type: str,
        candidate_agent_ids: list[str],
    ) -> dict[str, Any]:
        schema = dict(self.action_schemas[action_type])
        fields = []
        for field_def in schema.get("fields", []):
            current_field = dict(field_def)
            if current_field.get("type") == "agent_id":
                current_field["options"] = list(candidate_agent_ids)
            fields.append(current_field)
        schema["fields"] = fields
        return schema

    def _agent_ids(self, members: list[MemberRecord]) -> list[str]:
        return [
            member.member_agent_id
            for member in members
            if member.member_type == "agent" and member.member_agent_id
        ]

    def _alive_agent_ids(self, state: dict[str, Any]) -> list[str]:
        return list(state.get("alive_agent_ids") or [])

    def _resolve_winner(self, alive_agent_ids: list[str], roles: dict[str, str]) -> Optional[str]:
        alive_wolves = [agent_id for agent_id in alive_agent_ids if roles.get(agent_id) == "wolf"]
        alive_non_wolves = [agent_id for agent_id in alive_agent_ids if roles.get(agent_id) != "wolf"]
        if not alive_wolves:
            return "villagers"
        if len(alive_wolves) >= len(alive_non_wolves):
            return "wolves"
        return None

    def _latest_submission_by_request(
        self,
        submissions: list[ActionSubmissionRecord],
    ) -> dict[str, ActionSubmissionRecord]:
        latest: dict[str, ActionSubmissionRecord] = {}
        for submission in submissions:
            latest[submission.request_id] = submission
        return latest


class ProtocolDefinitionRegistry:
    """协议定义注册表。"""

    def __init__(self) -> None:
        self._definitions: dict[str, ProtocolDefinition] = {}

    def register(self, definition: ProtocolDefinition) -> None:
        self._definitions[definition.slug] = definition

    def get(self, slug: str) -> Optional[ProtocolDefinition]:
        return self._definitions.get(slug)

    def list(self) -> list[ProtocolDefinition]:
        return list(self._definitions.values())


protocol_definition_registry = ProtocolDefinitionRegistry()
protocol_definition_registry.register(WerewolfDemoProtocolDefinition())
