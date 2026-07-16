import { Bot, FolderTree, History, Info, type LucideIcon } from "lucide-react";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

export type RoomSurfaceTabKey = "chat" | "history" | "workspace" | "about" | "subagents";

interface RoomHeaderTab {
  anchor?: string;
  icon: LucideIcon;
  key: RoomSurfaceTabKey;
  label: string;
}

interface RoomHeaderTabDefinition extends Omit<RoomHeaderTab, "label"> {
  labelKey: TranslationKey;
}

const ROOM_HEADER_TAB_DEFINITIONS: readonly RoomHeaderTabDefinition[] = [
  {
    anchor: CONVERSATION_TOUR_ANCHORS.tab_history,
    icon: History,
    key: "history",
    labelKey: "room.history",
  },
  {
    icon: Bot,
    key: "subagents",
    labelKey: "subagents.label",
  },
  {
    anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace,
    icon: FolderTree,
    key: "workspace",
    labelKey: "room.workspace",
  },
  {
    anchor: CONVERSATION_TOUR_ANCHORS.tab_about,
    icon: Info,
    key: "about",
    labelKey: "room.about",
  },
];

export function buildRoomHeaderTabs(
  t: I18nContextValue["t"],
): RoomHeaderTab[] {
  return ROOM_HEADER_TAB_DEFINITIONS.map(({ labelKey, ...tab }) => ({
    ...tab,
    label: t(labelKey),
  }));
}
