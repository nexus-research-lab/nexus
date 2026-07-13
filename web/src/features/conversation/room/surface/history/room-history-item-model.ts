import { formatRelativeTime } from "@/lib/format/relative-time";

import type { RoomHistoryEntry } from "./room-history-model";

export type RoomHistoryItemAction = "delete" | "rename";
export type RoomHistoryItemMode = "editing" | "reading";
export type RoomHistoryItemState = "active" | "idle";

export interface RoomHistoryItemPresentation {
  actions: RoomHistoryItemAction[];
  activityLabel: string;
  currentLabel: string;
  externalSessionLabel: string | null;
  mode: RoomHistoryItemMode;
  state: RoomHistoryItemState;
  title: string;
}

interface RoomHistoryItemCopy {
  current: string;
  untitled: string;
}

const ACTION_DEFINITIONS: Array<{
  enabled: (entry: RoomHistoryEntry) => boolean;
  kind: RoomHistoryItemAction;
}> = [
  { enabled: (entry) => entry.canRename, kind: "rename" },
  { enabled: (entry) => entry.canDelete, kind: "delete" },
];

function itemActions(
  entry: RoomHistoryEntry,
  isEditing: boolean,
): RoomHistoryItemAction[] {
  if (isEditing) {
    return [];
  }
  return ACTION_DEFINITIONS
    .filter((definition) => definition.enabled(entry))
    .map((definition) => definition.kind);
}

function itemMode(isEditing: boolean): RoomHistoryItemMode {
  return isEditing ? "editing" : "reading";
}

function itemState(isActive: boolean): RoomHistoryItemState {
  return isActive ? "active" : "idle";
}

export function buildRoomHistoryItemPresentation(
  entry: RoomHistoryEntry,
  isEditing: boolean,
  copy: RoomHistoryItemCopy,
): RoomHistoryItemPresentation {
  return {
    actions: itemActions(entry, isEditing),
    activityLabel: formatRelativeTime(entry.conversation.last_activity_at),
    currentLabel: copy.current,
    externalSessionLabel: entry.externalSessionLabel,
    mode: itemMode(isEditing),
    state: itemState(entry.isActive),
    title: entry.conversation.title?.trim() || copy.untitled,
  };
}
