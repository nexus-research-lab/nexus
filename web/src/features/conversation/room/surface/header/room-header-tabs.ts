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

/** 主智能体主窗口只提供工作区与简介；工作图与子智能体不进入它的顶栏。 */
const MAIN_AGENT_TAB_KEYS: ReadonlySet<RoomSurfaceTabKey> = new Set(["workspace", "about"]);

/** 只有工作图与子智能体会因当前会话没有内容而隐藏；缺省视为可用。 */
export type RoomHeaderTabAvailability = Partial<Record<"workgraph" | "subagents", boolean>>;

export function buildRoomHeaderTabs(
  t: I18nContextValue["t"],
  options: {
    activeTab?: RoomSurfaceTabKey;
    availability?: RoomHeaderTabAvailability;
    mainAgent?: boolean;
  } = {},
): RoomHeaderTab[] {
  return ROOM_HEADER_TAB_DEFINITIONS
    .filter((tab) => !options.mainAgent || MAIN_AGENT_TAB_KEYS.has(tab.key))
    .filter((tab) => {
      if (tab.key !== "workgraph" && tab.key !== "subagents") return true;
      // 已打开的面板保留入口，否则用户无法从 Header 关闭它。
      return options.availability?.[tab.key] !== false || options.activeTab === tab.key;
    })
    .map(({ labelKey, ...tab }) => ({
      ...tab,
      label: t(labelKey),
    }));
}
