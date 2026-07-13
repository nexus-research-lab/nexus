import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import type { RoomEventPayload } from "@/types/agent/agent-conversation";

const REFRESH_EVENT_TYPES = new Set([
  "room_resync_required",
  "session_resync_required",
]);

interface UseRoomPageEventsOptions {
  roomId?: string | null;
  roomType: string;
  refreshRoomState: () => Promise<void>;
}

export function useRoomPageEvents({
  roomId,
  roomType,
  refreshRoomState,
}: UseRoomPageEventsOptions) {
  const navigate = useNavigate();

  return useCallback((eventType: string, data: RoomEventPayload) => {
    if (eventType === "room_deleted" && data.room_id === roomId) {
      navigate(
        roomType === "dm"
          ? AppRouteBuilders.contacts()
          : AppRouteBuilders.home(),
        {replace: true},
      );
      return;
    }

    if (REFRESH_EVENT_TYPES.has(eventType)) {
      void refreshRoomState().catch(() => undefined);
    }
  }, [navigate, refreshRoomState, roomId, roomType]);
}
