/**
 * INPUT: 当前路由、翻译函数、未读数、固定偏好与全局会话目录摘要。
 * OUTPUT: 一级导航、固定会话入口和底部操作文案的纯展示模型。
 * POS: 主侧栏控制器与纯视图之间的唯一派生层。
 */
import { MessageCircle, Puzzle, Users2 } from "lucide-react";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { PinnedConversationPreference } from "@/store/room-navigation";
import type { LauncherConversationSummary } from "@/types/app/launcher";

import type {
  SidebarPinnedConversationItem,
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarUtilityLabels,
} from "./view/sidebar-wide-panel-types";

const PRIMARY_TAB_DEFINITIONS = [
  {
    anchor: SIDEBAR_TOUR_ANCHORS.chat_tab,
    icon: MessageCircle,
    key: "chat",
    labelKey: "sidebar.tab_chat",
  },
  {
    anchor: SIDEBAR_TOUR_ANCHORS.contacts_tab,
    icon: Users2,
    key: "contacts",
    labelKey: "sidebar.tab_contacts",
  },
  {
    anchor: SIDEBAR_TOUR_ANCHORS.capabilities_tab,
    icon: Puzzle,
    key: "capabilities",
    labelKey: "sidebar.tab_capabilities",
  },
] as const satisfies readonly {
  anchor: string;
  icon: SidebarPrimaryTabItem["icon"];
  key: SidebarPrimaryTab;
  labelKey: TranslationKey;
}[];

export function deriveSidebarPrimaryTab(pathname: string): SidebarPrimaryTab {
  if (pathname.startsWith(AppRouteBuilders.contacts())) {
    return "contacts";
  }
  if (pathname.startsWith("/capability")) {
    return "capabilities";
  }
  return "chat";
}

export function buildSidebarPrimaryTabs(
  t: I18nContextValue["t"],
  activeTab: SidebarPrimaryTab | null,
  chatBadgeCount: number,
): SidebarPrimaryTabItem[] {
  return PRIMARY_TAB_DEFINITIONS.map((definition) => ({
    anchor: definition.anchor,
    badgeCount: definition.key === "chat" && activeTab !== "chat"
      ? chatBadgeCount
      : 0,
    icon: definition.icon,
    key: definition.key,
    label: t(definition.labelKey),
  }));
}

export function buildSidebarPinnedConversations({
  conversations,
  pathname,
  pinnedConversations,
  untitledLabel,
}: {
  conversations: LauncherConversationSummary[];
  pathname: string;
  pinnedConversations: PinnedConversationPreference[];
  untitledLabel: string;
}): SidebarPinnedConversationItem[] {
  const conversationBySessionKey = new Map(
    conversations.map((conversation) => [conversation.session_key, conversation]),
  );
  const conversationByIdentity = new Map(
    conversations
      .filter((conversation) => conversation.room_id && conversation.conversation_id)
      .map((conversation) => [
        getPinnedConversationIdentity(
          conversation.room_id ?? "",
          conversation.conversation_id ?? "",
        ),
        conversation,
      ]),
  );

  return pinnedConversations.map((conversation) => {
    const directoryConversation = (
      conversation.session_key
        ? conversationBySessionKey.get(conversation.session_key)
        : undefined
    ) ?? conversationByIdentity.get(getPinnedConversationIdentity(
      conversation.room_id,
      conversation.conversation_id,
    ));
    const route = AppRouteBuilders.conversation(
      conversation.room_id,
      conversation.conversation_id,
    );
    return {
      active: pathname === route,
      conversationId: conversation.conversation_id,
      key: getPinnedConversationIdentity(
        conversation.room_id,
        conversation.conversation_id,
      ),
      roomId: conversation.room_id,
      route,
      title: directoryConversation?.title.trim()
        || conversation.title
        || untitledLabel,
    };
  });
}

function getPinnedConversationIdentity(roomId: string, conversationId: string): string {
  return `${roomId}\u0000${conversationId}`;
}

export function buildSidebarUtilityLabels(
  t: I18nContextValue["t"],
): SidebarUtilityLabels {
  return {
    collapse: t("sidebar.collapse_panel"),
    expand: t("sidebar.expand_panel"),
    guide: t("common.guide_center"),
    logout: t("sidebar.logout"),
    settings: t("sidebar.settings"),
  };
}
