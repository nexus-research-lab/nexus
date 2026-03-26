"use client";

import { useEffect, useMemo, useState } from "react";
import { AlertTriangle } from "lucide-react";

import {
  ProtocolActionRequestRecord,
  ProtocolChannelAggregate,
  ProtocolRunControlOperation,
  ProtocolRunDetail,
  ProtocolRunListItem,
  ProtocolSnapshotRecord,
  RoomAggregate,
  RoomMemberRecord,
  WebSocketState,
} from "@/types";

import { ProtocolRoomControlPanel } from "./protocol-room-control-panel";
import { ProtocolRoomEmptyState } from "./protocol-room-empty-state";
import { ProtocolRoomFeed } from "./protocol-room-feed";
import { ProtocolRoomSidebar } from "./protocol-room-sidebar";

interface ProtocolRoomShellProps {
  room: RoomAggregate;
  runs: ProtocolRunListItem[];
  detail: ProtocolRunDetail | null;
  room_agent_members: RoomMemberRecord[];
  pending_requests: ProtocolActionRequestRecord[];
  selected_channel: ProtocolChannelAggregate | null;
  selected_channel_id: string | null;
  selected_channel_events: ProtocolSnapshotRecord[];
  viewer_agent_id: string | null;
  is_loading: boolean;
  ws_state: WebSocketState;
  error: string | null;
  on_create_run: (params?: { definition_slug?: string; title?: string }) => Promise<unknown>;
  on_select_run: (run_id: string) => void;
  on_select_channel: (channel_id: string) => void;
  on_set_viewer: (agent_id: string | null) => void;
  on_submit_action: (
    request_id: string,
    payload: Record<string, any>,
    actor_agent_id?: string | null,
    options?: { as_override?: boolean },
  ) => Promise<unknown>;
  on_control: (
    operation: ProtocolRunControlOperation,
    payload?: Record<string, any>,
  ) => Promise<unknown>;
  on_refresh: () => Promise<unknown>;
}

export function ProtocolRoomShell({
  room,
  runs,
  detail,
  room_agent_members,
  pending_requests,
  selected_channel,
  selected_channel_id,
  selected_channel_events,
  viewer_agent_id,
  is_loading,
  ws_state,
  error,
  on_create_run,
  on_select_run,
  on_select_channel,
  on_set_viewer,
  on_submit_action,
  on_control,
  on_refresh,
}: ProtocolRoomShellProps) {
  const [request_payloads, set_request_payloads] = useState<Record<string, Record<string, string>>>({});
  const [request_actors, set_request_actors] = useState<Record<string, string>>({});
  const [inject_channel_id, set_inject_channel_id] = useState<string>("");
  const [inject_message, set_inject_message] = useState("");
  const [composer_actor_id, set_composer_actor_id] = useState<string>("");
  const [force_phase_name, set_force_phase_name] = useState("");
  const [busy_request_id, set_busy_request_id] = useState<string | null>(null);
  const [is_busy_control, set_is_busy_control] = useState(false);

  const alive_agent_ids = useMemo(
    () => new Set<string>(
      Array.isArray(detail?.run.state?.alive_agent_ids)
        ? detail.run.state.alive_agent_ids.filter((item): item is string => typeof item === "string")
        : [],
    ),
    [detail?.run.state?.alive_agent_ids],
  );
  const eliminated_agent_ids = useMemo(
    () => new Set<string>(
      Array.isArray(detail?.run.state?.eliminated_agent_ids)
        ? detail.run.state.eliminated_agent_ids.filter((item): item is string => typeof item === "string")
        : [],
    ),
    [detail?.run.state?.eliminated_agent_ids],
  );
  const roles_by_agent_id = detail?.run.state?.roles ?? {};

  const resolved_request_ids = useMemo(
    () => new Set(
      detail?.action_submissions
        ?.filter((submission) => submission.status === "submitted" || submission.status === "overridden")
        .map((submission) => submission.request_id) ?? [],
    ),
    [detail?.action_submissions],
  );

  const pending_current_phase_requests = useMemo(
    () => pending_requests.filter((request) => request.phase_name === detail?.run.current_phase),
    [detail?.run.current_phase, pending_requests],
  );

  const member_request_status = useMemo(() => {
    const status_map = new Map<string, "pending" | "submitted" | "idle">();
    room_agent_members.forEach((member) => {
      if (member.member_agent_id) {
        status_map.set(member.member_agent_id, "idle");
      }
    });

    pending_current_phase_requests.forEach((request) => {
      request.allowed_actor_agent_ids.forEach((agent_id) => {
        status_map.set(
          agent_id,
          resolved_request_ids.has(request.id) ? "submitted" : "pending",
        );
      });
    });
    return status_map;
  }, [pending_current_phase_requests, resolved_request_ids, room_agent_members]);

  const remaining_phases = useMemo(() => {
    if (!detail) return [];
    const current_index = detail.definition.phases.indexOf(detail.run.current_phase);
    return detail.definition.phases.slice(Math.max(current_index + 1, 0));
  }, [detail]);

  const visible_channels = useMemo(
    () => detail?.channels ?? [],
    [detail?.channels],
  );

  const visible_channel_member_agent_ids = useMemo(
    () => selected_channel?.members
      ?.filter((member: any) => member.member_type === "agent" && member.member_agent_id)
      .map((member: any) => member.member_agent_id as string) ?? [],
    [selected_channel],
  );

  const selected_channel_requests = useMemo(
    () => pending_requests.filter((request) => request.channel_id === selected_channel_id),
    [pending_requests, selected_channel_id],
  );

  const pending_requests_for_viewer = useMemo(
    () => viewer_agent_id
      ? pending_requests.filter((request) => request.allowed_actor_agent_ids.includes(viewer_agent_id))
      : [],
    [pending_requests, viewer_agent_id],
  );

  useEffect(() => {
    if (!detail?.channels?.length) {
      set_inject_channel_id("");
      return;
    }
    if (detail.channels.some((channel) => channel.channel.id === inject_channel_id)) {
      return;
    }
    const fallback_channel = detail.channels.find((channel) => channel.channel.slug === "public-main") ?? detail.channels[0];
    set_inject_channel_id(fallback_channel?.channel.id ?? "");
  }, [detail?.channels, inject_channel_id]);

  useEffect(() => {
    if (!detail) {
      set_force_phase_name("");
      return;
    }
    const current_index = detail.definition.phases.indexOf(detail.run.current_phase);
    const next_phase = detail.definition.phases[current_index + 1] ?? "";
    set_force_phase_name(next_phase);
  }, [detail]);

  useEffect(() => {
    const next_actor = viewer_agent_id && visible_channel_member_agent_ids.includes(viewer_agent_id)
      ? viewer_agent_id
      : visible_channel_member_agent_ids[0] ?? "";
    set_composer_actor_id(next_actor);
  }, [viewer_agent_id, visible_channel_member_agent_ids, selected_channel_id]);

  const handle_create_run = async () => {
    await on_create_run({
      definition_slug: "werewolf_demo",
      title: room.room.name ? `${room.room.name} · 协议协作` : "协议协作",
    });
  };

  const handle_submit_request = async (
    request: ProtocolActionRequestRecord,
    as_override: boolean,
  ) => {
    const actor_agent_id = request_actors[request.id] || request.allowed_actor_agent_ids[0] || null;
    const payload = request_payloads[request.id] ?? {};
    set_busy_request_id(request.id);
    try {
      await on_submit_action(request.id, payload, actor_agent_id, { as_override });
    } finally {
      set_busy_request_id(null);
    }
  };

  const handle_control = async (
    operation: ProtocolRunControlOperation,
    payload: Record<string, any> = {},
  ) => {
    set_is_busy_control(true);
    try {
      await on_control(operation, payload);
    } finally {
      set_is_busy_control(false);
    }
  };

  const handle_send_room_message = async () => {
    const trimmed_message = inject_message.trim();
    if (!inject_channel_id || !trimmed_message) {
      return;
    }

    await handle_control("inject_message", {
      channel_id: inject_channel_id,
      content: trimmed_message,
      actor_agent_id: composer_actor_id || undefined,
      headline: composer_actor_id ? `${composer_actor_id} 的消息` : "观察者消息",
      message_kind: "message",
    });
    set_inject_message("");
  };

  if (!detail) {
    return (
      <ProtocolRoomEmptyState
        error={error}
        on_create_run={handle_create_run}
        on_refresh={on_refresh}
        room={room}
        room_agent_members={room_agent_members}
        ws_state={ws_state}
      />
    );
  }

  return (
    <div className="relative flex min-h-0 flex-1 flex-col px-4 py-4 sm:px-6 sm:py-6">
      <div className="pointer-events-none absolute inset-x-[12%] top-[8%] h-36 rounded-full bg-[radial-gradient(circle,rgba(120,170,255,0.2),transparent_72%)] blur-3xl" />
      <section className="panel-surface relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[34px] px-4 py-4 sm:px-6 sm:py-6">
        <header className="relative z-10 flex flex-col gap-4 border-b border-white/55 pb-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-700/52">
              <span className="neo-pill rounded-full px-3 py-1">多人协作房间</span>
              <span>{room.room.room_type === "room" ? "多人协作" : "单聊"}</span>
            </div>
            <h1 className="mt-3 text-[30px] font-black tracking-[-0.05em] text-slate-950/92">
              {room.room.name || room.room.id}
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-700/62">
              你正在和多个 agent 共享同一个房间。中间是房间流，左边是成员与频道，右边是辅助控制与当前阶段信息。
            </p>
          </div>
        </header>

        {error ? (
          <div className="neo-card-flat relative z-10 mt-4 flex items-start gap-3 rounded-[24px] border border-rose-400/25 px-4 py-3 text-sm text-rose-900/84">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-500" />
            <span>{error}</span>
          </div>
        ) : null}

        <div className="relative z-10 grid min-h-0 flex-1 gap-4 py-5 xl:grid-cols-[300px_minmax(0,1fr)_340px]">
          <ProtocolRoomSidebar
            alive_agent_ids={alive_agent_ids}
            detail={detail}
            eliminated_agent_ids={eliminated_agent_ids}
            member_request_status={member_request_status}
            on_select_channel={on_select_channel}
            on_select_run={on_select_run}
            on_set_viewer={on_set_viewer}
            roles_by_agent_id={roles_by_agent_id}
            room_agent_members={room_agent_members}
            runs={runs}
            selected_channel_id={selected_channel_id}
            viewer_agent_id={viewer_agent_id}
            visible_channels={visible_channels}
          />

          <ProtocolRoomFeed
            busy_request_id={busy_request_id}
            composer_actor_id={composer_actor_id}
            inject_channel_id={inject_channel_id}
            inject_message={inject_message}
            is_busy_control={is_busy_control}
            on_change_composer_actor={set_composer_actor_id}
            on_change_inject_message={set_inject_message}
            on_change_request_actor={(request_id, actor_id) => {
              set_request_actors((prev) => ({ ...prev, [request_id]: actor_id }));
            }}
            on_change_request_payload={(request_id, field_name, value) => {
              set_request_payloads((prev) => ({
                ...prev,
                [request_id]: {
                  ...(prev[request_id] ?? {}),
                  [field_name]: value,
                },
              }));
            }}
            on_send_room_message={handle_send_room_message}
            on_submit_request={handle_submit_request}
            request_actors={request_actors}
            request_payloads={request_payloads}
            selected_channel={selected_channel}
            selected_channel_events={selected_channel_events}
            selected_channel_requests={selected_channel_requests}
            visible_channel_member_agent_ids={visible_channel_member_agent_ids}
          />

          <ProtocolRoomControlPanel
            detail={detail}
            force_phase_name={force_phase_name}
            inject_channel_id={inject_channel_id}
            inject_message={inject_message}
            is_busy_control={is_busy_control}
            is_loading={is_loading}
            on_change_force_phase={set_force_phase_name}
            on_control={handle_control}
            on_send_room_message={handle_send_room_message}
            pending_current_phase_requests_count={pending_current_phase_requests.length}
            pending_requests_for_viewer_count={pending_requests_for_viewer.length}
            remaining_phases={remaining_phases}
            viewer_agent_id={viewer_agent_id}
          />
        </div>
      </section>
    </div>
  );
}
