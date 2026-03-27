"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import {
  addRoomMember,
  getRoom,
  getRoomView,
  postRoomAction,
  postRoomMessage,
  runRoomPhase,
  runRoomUntilFinished,
  startRoom,
  tickRoom,
} from "@/lib/room-api";
import { RoomActionEnvelope, RoomAggregate, RoomMessageEnvelope, RoomRuntimeView } from "@/types";

interface UseOpenRoomControllerOptions {
  room_id?: string | null;
}

export function useOpenRoomController({ room_id }: UseOpenRoomControllerOptions) {
  const [room, set_room] = useState<RoomAggregate | null>(null);
  const [view, set_view] = useState<RoomRuntimeView | null>(null);
  const [is_open_room, set_is_open_room] = useState(false);
  const [is_loading, set_is_loading] = useState(false);
  const [is_checked, set_is_checked] = useState(false);
  const [error, set_error] = useState<string | null>(null);

  const load_room = useCallback(async () => {
    if (!room_id) {
      set_room(null);
      set_view(null);
      set_is_open_room(false);
      set_error(null);
      set_is_checked(true);
      return;
    }

    set_is_loading(true);
    try {
      const next_room = await getRoom(room_id);
      set_room(next_room);

      if (next_room.room.mode !== "open") {
        set_view(null);
        set_is_open_room(false);
        set_error(null);
        return;
      }

      const next_view = await getRoomView(room_id);
      set_view(next_view);
      set_is_open_room(true);
      set_error(null);
    } catch (load_error) {
      set_room(null);
      set_view(null);
      set_is_open_room(false);
      set_error(load_error instanceof Error ? load_error.message : "加载 room 失败");
    } finally {
      set_is_loading(false);
      set_is_checked(true);
    }
  }, [room_id]);

  useEffect(() => {
    void load_room();
  }, [load_room]);

  const run_command = useCallback(async (
    executor: () => Promise<RoomRuntimeView>,
    fallback_message: string,
  ) => {
    set_is_loading(true);
    try {
      const next_view = await executor();
      set_view(next_view);
      set_error(null);
      return next_view;
    } catch (run_error) {
      set_error(run_error instanceof Error ? run_error.message : fallback_message);
      throw run_error;
    } finally {
      set_is_loading(false);
    }
  }, []);

  const handle_start = useCallback(async () => {
    if (!room_id) {
      return null;
    }
    return await run_command(() => startRoom(room_id), "启动 room 失败");
  }, [room_id, run_command]);

  const handle_tick = useCallback(async () => {
    if (!room_id) {
      return null;
    }
    return await run_command(() => tickRoom(room_id), "推进 room 失败");
  }, [room_id, run_command]);

  const handle_run_phase = useCallback(async () => {
    if (!room_id) {
      return null;
    }
    return await run_command(() => runRoomPhase(room_id), "执行 phase 失败");
  }, [room_id, run_command]);

  const handle_run_until_finished = useCallback(async () => {
    if (!room_id) {
      return null;
    }
    return await run_command(() => runRoomUntilFinished(room_id), "运行 room 失败");
  }, [room_id, run_command]);

  const handle_post_message = useCallback(async (params: RoomMessageEnvelope) => {
    if (!room_id) {
      return null;
    }
    return await run_command(() => postRoomMessage(room_id, params), "发送消息失败");
  }, [room_id, run_command]);

  const handle_post_action = useCallback(async (params: RoomActionEnvelope) => {
    if (!room_id) {
      return null;
    }
    return await run_command(() => postRoomAction(room_id, params), "提交动作失败");
  }, [room_id, run_command]);

  const handle_add_member = useCallback(async (existing_agent_id: string) => {
    if (!room_id) {
      return null;
    }
    return await run_command(
      () => addRoomMember(room_id, { existing_agent_id, source: "existing", workspace_binding: true }),
      "添加成员失败",
    );
  }, [room_id, run_command]);

  const participants = useMemo(
    () => view?.members ?? room?.members ?? [],
    [room?.members, view?.members],
  );

  return {
    open_room: room,
    open_room_view: view,
    participants,
    is_open_room,
    is_open_room_loading: is_loading,
    is_open_room_checked: is_checked,
    open_room_error: error,
    handle_open_room_refresh: load_room,
    handle_open_room_start: handle_start,
    handle_open_room_tick: handle_tick,
    handle_open_room_run_phase: handle_run_phase,
    handle_open_room_run_until_finished: handle_run_until_finished,
    handle_open_room_message: handle_post_message,
    handle_open_room_action: handle_post_action,
    handle_open_room_add_member: handle_add_member,
  };
}
