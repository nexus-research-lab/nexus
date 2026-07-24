"use client";

import { useMediaQuery } from "@/hooks/ui/use-media-query";

import type {
  RoomHeaderTab,
  RoomSurfaceTabKey,
} from "./room-header-tabs";

const ROOM_HEADER_TAB_COLLAPSE_QUERIES: Partial<
  Record<RoomSurfaceTabKey, string>
> = {
  about: "(max-width: 1239px)",
  workspace: "(max-width: 1119px)",
  subagents: "(max-width: 1039px)",
};

export function useRoomHeaderOverflowTabs(
  tabs: RoomHeaderTab[],
): RoomHeaderTab[] {
  const collapseAbout = useMediaQuery(
    ROOM_HEADER_TAB_COLLAPSE_QUERIES.about ?? "",
  );
  const collapseWorkspace = useMediaQuery(
    ROOM_HEADER_TAB_COLLAPSE_QUERIES.workspace ?? "",
  );
  const collapseSubagents = useMediaQuery(
    ROOM_HEADER_TAB_COLLAPSE_QUERIES.subagents ?? "",
  );
  const collapsedKeys = new Set<RoomSurfaceTabKey>([
    ...(collapseAbout ? ["about" as const] : []),
    ...(collapseWorkspace ? ["workspace" as const] : []),
    ...(collapseSubagents ? ["subagents" as const] : []),
  ]);

  return tabs.filter((tab) => collapsedKeys.has(tab.key));
}
