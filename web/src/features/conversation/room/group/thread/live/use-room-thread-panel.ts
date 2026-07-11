import { useMemo } from "react";

import { useGroupThread } from "../group-thread-state";
import { useRoomThreadLiveStore } from "./room-thread-live-store";
import {
  buildRoomThreadPanelModel,
  type RoomThreadPanelModel,
} from "./room-thread-panel-model";

export function useRoomThreadPanel(): RoomThreadPanelModel | null {
  const { activeThread } = useGroupThread();
  const source = useRoomThreadLiveStore((state) => state.source);
  return useMemo(
    () => buildRoomThreadPanelModel(source, activeThread),
    [activeThread, source],
  );
}
