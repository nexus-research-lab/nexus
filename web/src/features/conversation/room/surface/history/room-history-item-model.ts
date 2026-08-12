/**
 * INPUT: 单条历史会话、标题编辑状态与可选的批量选择状态。
 * OUTPUT: 条目模式、标题元信息、动作和选择控件的封闭展示模型。
 * POS: Room 历史单项从领域条目到纯视图的唯一投影入口。
 */

import { formatRelativeTime } from "@/lib/format/relative-time";
import type { Locale } from "@/shared/i18n/messages";

import type { RoomHistoryEntry } from "./room-history-model";

export type RoomHistoryItemAction = "delete" | "rename";
export type RoomHistoryItemMode = "editing" | "reading" | "selecting";
export type RoomHistoryItemState = "active" | "idle";

export interface RoomHistoryItemSelectionPresentation {
  checked: boolean;
  disabled: boolean;
}

export interface RoomHistoryItemPresentation {
  actionLabels: Record<RoomHistoryItemAction, string>;
  actions: RoomHistoryItemAction[];
  actionsPersistent: boolean;
  activityLabel: string;
  externalSessionLabel: string | null;
  editorLabels: {
    cancel: string;
    confirm: string;
    input: string;
  };
  mode: RoomHistoryItemMode;
  selection: RoomHistoryItemSelectionPresentation | null;
  state: RoomHistoryItemState;
  title: string;
}

interface RoomHistoryItemCopy {
  actionLabels: Record<RoomHistoryItemAction, string>;
  editorLabels: RoomHistoryItemPresentation["editorLabels"];
  locale: Locale;
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
  isSelecting: boolean,
): RoomHistoryItemAction[] {
  if (isEditing || isSelecting) {
    return [];
  }
  return ACTION_DEFINITIONS
    .filter((definition) => definition.enabled(entry))
    .map((definition) => definition.kind);
}

function itemMode(
  isEditing: boolean,
  isSelecting: boolean,
): RoomHistoryItemMode {
  if (isSelecting) {
    return "selecting";
  }
  return isEditing ? "editing" : "reading";
}

function itemState(isActive: boolean): RoomHistoryItemState {
  return isActive ? "active" : "idle";
}

export function buildRoomHistoryItemPresentation(
  entry: RoomHistoryEntry,
  {
    isEditing,
    isSelected,
    isSelecting,
  }: {
    isEditing: boolean;
    isSelected: boolean;
    isSelecting: boolean;
  },
  copy: RoomHistoryItemCopy,
): RoomHistoryItemPresentation {
  const actions = itemActions(entry, isEditing, isSelecting);
  return {
    actionLabels: copy.actionLabels,
    actions,
    actionsPersistent: actions.length > 0 && (
      entry.isActive || entry.externalSessionLabel !== null
    ),
    activityLabel: formatRelativeTime(
      entry.conversation.last_activity_at,
      copy.locale,
    ),
    editorLabels: copy.editorLabels,
    externalSessionLabel: entry.externalSessionLabel,
    mode: itemMode(isEditing, isSelecting),
    selection: isSelecting
      ? {
          checked: isSelected,
          disabled: !entry.canBulkDelete,
        }
      : null,
    state: itemState(entry.isActive),
    title: entry.conversation.title?.trim() || copy.untitled,
  };
}
