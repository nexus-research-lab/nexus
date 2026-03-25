"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { getAgentWsUrl } from "@/config/options";
import { useWebSocket } from "@/lib/websocket";
import {
  controlProtocolRun,
  createRoomProtocolRun,
  getProtocolRun,
  getRoom,
  listRoomProtocolRuns,
  submitProtocolAction,
} from "@/lib/room-api";
import {
  CreateProtocolRunParams,
  EventMessage,
  ProtocolRunControlOperation,
  ProtocolRunDetail,
  ProtocolRunListItem,
  RoomAggregate,
} from "@/types";

interface UseProtocolRoomControllerOptions {
  room_id?: string | null;
}

export function useProtocolRoomController({ room_id }: UseProtocolRoomControllerOptions) {
  const [room, set_room] = useState<RoomAggregate | null>(null);
  const [runs, set_runs] = useState<ProtocolRunListItem[]>([]);
  const [detail, set_detail] = useState<ProtocolRunDetail | null>(null);
  const [active_run_id, set_active_run_id] = useState<string | null>(null);
  const [viewer_agent_id, set_viewer_agent_id] = useState<string | null>(null);
  const [selected_channel_id, set_selected_channel_id] = useState<string | null>(null);
  const [is_protocol_room, set_is_protocol_room] = useState(false);
  const [is_room_loading, set_is_room_loading] = useState(false);
  const [is_run_loading, set_is_run_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const [is_checked, set_is_checked] = useState(false);

  const load_run_detail = useCallback(async (
    run_id: string,
    next_viewer_agent_id?: string | null,
  ) => {
    set_is_run_loading(true);
    try {
      const next_detail = await getProtocolRun(run_id, next_viewer_agent_id ?? viewer_agent_id);
      set_detail(next_detail);
      set_error(null);
      return next_detail;
    } catch (load_error) {
      set_error(load_error instanceof Error ? load_error.message : "加载 protocol run 失败");
      throw load_error;
    } finally {
      set_is_run_loading(false);
    }
  }, [viewer_agent_id]);

  const handle_room_socket_message = useCallback((backend_message: unknown) => {
    const event = backend_message as EventMessage;
    if (!event || event.event_type !== "room_state") {
      return;
    }
    const incoming_run_id = event.protocol_run_id || event.data?.run_id || null;
    if (!incoming_run_id || incoming_run_id !== active_run_id) {
      return;
    }
    void load_run_detail(incoming_run_id, viewer_agent_id);
  }, [active_run_id, load_run_detail, viewer_agent_id]);

  const { state: ws_state, send: ws_send } = useWebSocket({
    url: getAgentWsUrl(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_room_socket_message,
  });

  const load_room = useCallback(async () => {
    if (!room_id) {
      set_room(null);
      set_runs([]);
      set_detail(null);
      set_active_run_id(null);
      set_is_protocol_room(false);
      set_is_checked(true);
      return;
    }

    set_is_room_loading(true);
    try {
      const next_room = await getRoom(room_id);
      const next_runs = await listRoomProtocolRuns(room_id);
      const next_active_run_id = next_runs[0]?.run.id ?? null;

      set_room(next_room);
      set_runs(next_runs);
      set_active_run_id(next_active_run_id);
      set_is_protocol_room(true);
      set_error(null);

      if (!next_active_run_id) {
        set_detail(null);
      }
    } catch {
      set_room(null);
      set_runs([]);
      set_detail(null);
      set_active_run_id(null);
      set_is_protocol_room(false);
      set_error(null);
    } finally {
      set_is_room_loading(false);
      set_is_checked(true);
    }
  }, [room_id]);

  useEffect(() => {
    void load_room();
  }, [load_room]);

  useEffect(() => {
    if (!is_protocol_room || !active_run_id) {
      return;
    }
    void load_run_detail(active_run_id, viewer_agent_id);
  }, [active_run_id, is_protocol_room, load_run_detail, viewer_agent_id]);

  useEffect(() => {
    if (
      !is_protocol_room
      || !active_run_id
      || !detail
      || detail.run.status !== "running"
      || ws_state === "connected"
    ) {
      return;
    }

    const timer = window.setInterval(() => {
      void load_run_detail(active_run_id, viewer_agent_id);
    }, 4000);

    return () => {
      window.clearInterval(timer);
    };
  }, [active_run_id, detail, is_protocol_room, load_run_detail, viewer_agent_id, ws_state]);

  useEffect(() => {
    if (!active_run_id || ws_state !== "connected") {
      return;
    }

    ws_send({
      type: "subscribe_protocol_run",
      protocol_run_id: active_run_id,
    });

    return () => {
      ws_send({
        type: "unsubscribe_protocol_run",
        protocol_run_id: active_run_id,
      });
    };
  }, [active_run_id, ws_send, ws_state]);

  useEffect(() => {
    if (!detail?.channels?.length) {
      set_selected_channel_id(null);
      return;
    }

    const still_exists = detail.channels.some((channel) => channel.channel.id === selected_channel_id);
    if (still_exists) {
      return;
    }

    const first_visible_channel = detail.channels.find((channel) => channel.channel.metadata?.is_visible);
    set_selected_channel_id(first_visible_channel?.channel.id ?? detail.channels[0]?.channel.id ?? null);
  }, [detail?.channels, selected_channel_id]);

  const handle_create_run = useCallback(async (params: CreateProtocolRunParams = {}) => {
    if (!room_id) {
      return null;
    }
    set_is_run_loading(true);
    try {
      const next_detail = await createRoomProtocolRun(room_id, {
        definition_slug: params.definition_slug ?? "werewolf_demo",
        title: params.title,
        run_config: params.run_config ?? {},
      });
      set_detail(next_detail);
      set_active_run_id(next_detail.run.id);
      set_runs((prev) => [
        { run: next_detail.run, definition: next_detail.definition },
        ...prev.filter((item) => item.run.id !== next_detail.run.id),
      ]);
      set_error(null);
      return next_detail;
    } catch (create_error) {
      set_error(create_error instanceof Error ? create_error.message : "创建 protocol run 失败");
      throw create_error;
    } finally {
      set_is_run_loading(false);
    }
  }, [room_id]);

  const handle_select_run = useCallback((run_id: string) => {
    set_active_run_id(run_id);
  }, []);

  const handle_select_channel = useCallback((channel_id: string) => {
    set_selected_channel_id(channel_id);
  }, []);

  const handle_set_viewer = useCallback((agent_id: string | null) => {
    set_viewer_agent_id(agent_id);
  }, []);

  const handle_submit_action = useCallback(async (
    request_id: string,
    payload: Record<string, any>,
    actor_agent_id?: string | null,
    options?: { as_override?: boolean },
  ) => {
    if (!active_run_id) {
      return null;
    }
    set_is_run_loading(true);
    try {
      const next_detail = options?.as_override
        ? await controlProtocolRun(active_run_id, {
          operation: "override_action",
          payload: {
            request_id,
            actor_agent_id,
            action_payload: payload,
          },
        })
        : await submitProtocolAction(active_run_id, {
          request_id,
          payload,
          actor_agent_id: actor_agent_id ?? undefined,
        });

      set_detail(next_detail);
      set_runs((prev) => prev.map((item) => (
        item.run.id === next_detail.run.id
          ? { run: next_detail.run, definition: next_detail.definition }
          : item
      )));
      set_error(null);
      return next_detail;
    } catch (submit_error) {
      set_error(submit_error instanceof Error ? submit_error.message : "提交 protocol action 失败");
      throw submit_error;
    } finally {
      set_is_run_loading(false);
    }
  }, [active_run_id]);

  const handle_control = useCallback(async (
    operation: ProtocolRunControlOperation,
    payload: Record<string, any> = {},
  ) => {
    if (!active_run_id) {
      return null;
    }
    set_is_run_loading(true);
    try {
      const next_detail = await controlProtocolRun(active_run_id, {
        operation,
        payload,
      });
      set_detail(next_detail);
      set_runs((prev) => prev.map((item) => (
        item.run.id === next_detail.run.id
          ? { run: next_detail.run, definition: next_detail.definition }
          : item
      )));
      set_error(null);
      return next_detail;
    } catch (control_error) {
      set_error(control_error instanceof Error ? control_error.message : "执行 protocol control 失败");
      throw control_error;
    } finally {
      set_is_run_loading(false);
    }
  }, [active_run_id]);

  const handle_refresh = useCallback(async () => {
    await load_room();
    if (active_run_id) {
      await load_run_detail(active_run_id, viewer_agent_id);
    }
  }, [active_run_id, load_room, load_run_detail, viewer_agent_id]);

  const room_agent_members = useMemo(
    () => room?.members.filter((member) => member.member_type === "agent" && member.member_agent_id) ?? [],
    [room],
  );

  const selected_channel = useMemo(
    () => detail?.channels.find((channel) => channel.channel.id === selected_channel_id) ?? null,
    [detail?.channels, selected_channel_id],
  );

  const pending_requests = useMemo(
    () => detail?.action_requests.filter((request) => request.status === "pending") ?? [],
    [detail?.action_requests],
  );

  const visible_channel_ids = useMemo(
    () => new Set(
      detail?.channels
        .filter((channel) => channel.channel.metadata?.is_visible)
        .map((channel) => channel.channel.id) ?? [],
    ),
    [detail?.channels],
  );

  const selected_channel_events = useMemo(
    () => detail?.snapshots.filter((snapshot) => (
      snapshot.channel_id === selected_channel_id
      && (
        visible_channel_ids.has(snapshot.channel_id ?? "")
        || Boolean(snapshot.metadata?.redacted)
      )
    )) ?? [],
    [detail?.snapshots, selected_channel_id, visible_channel_ids],
  );

  return {
    room,
    runs,
    detail,
    active_run_id,
    viewer_agent_id,
    selected_channel_id,
    selected_channel,
    pending_requests,
    room_agent_members,
    selected_channel_events,
    ws_state,
    is_protocol_room,
    is_room_loading,
    is_run_loading,
    is_checked,
    error,
    handle_create_run,
    handle_select_run,
    handle_select_channel,
    handle_set_viewer,
    handle_submit_action,
    handle_control,
    handle_refresh,
  };
}
