import { Bot, FolderTree, Info, Workflow, type LucideIcon } from "lucide-react";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

export type RoomSurfaceTabKey =
  | "chat"
  | "workgraph"
  | "workspace"
  | "about"
  | "subagents";

export interface RoomHeaderTab {
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
    icon: Workflow,
    key: "workgraph",
    labelKey: "room.workgraph",
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
  { workgraphAvailable = true }: { workgraphAvailable?: boolean } = {},
): RoomHeaderTab[] {
  return ROOM_HEADER_TAB_DEFINITIONS
    .filter((tab) => tab.key !== "workgraph" || workgraphAvailable)
    .map(({ labelKey, ...tab }) => ({
      ...tab,
      label: t(labelKey),
    }));
}
