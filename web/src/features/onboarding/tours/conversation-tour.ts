import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-contract";

export const CONVERSATION_TOUR_ANCHORS = {
  composer: "conversation-composer",
  empty_create: "conversation-empty-create",
  feed: "conversation-feed",
  header: "conversation-header",
  member_manage: "conversation-member-manage",
  session_switcher: "conversation-session-switcher",
  tab_about: "conversation-tab-about",
  tab_chat: "conversation-tab-chat",
  tab_history: "conversation-tab-history",
  tab_workspace: "conversation-tab-workspace",
} as const;

export const DM_CONVERSATION_TOUR_ID = "dm-conversation";
export const ROOM_CONVERSATION_TOUR_ID = "room-conversation";
export const ROOM_EMPTY_CONVERSATION_TOUR_ID = "room-empty-conversation";

export function buildDmConversationTour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: DM_CONVERSATION_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("room.tour_dm_intro_title"),
        description: t("room.tour_dm_intro_description"),
        image: "/nexus/stickers/guide-dm.png",
        placement: "center",
      },
      {
        id: "session-switcher",
        title: t("room.tour_session_title"),
        description: t("room.tour_session_description"),
        target: CONVERSATION_TOUR_ANCHORS.session_switcher,
        placement: "bottom",
      },
      {
        id: "history-tab",
        title: t("room.tour_tab_history_title"),
        description: t("room.tour_tab_history_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_history,
        placement: "bottom",
      },
      {
        id: "workspace-tab",
        title: t("room.tour_tab_workspace_title"),
        description: t("room.tour_tab_workspace_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_workspace,
        placement: "bottom",
      },
      {
        id: "about-tab",
        title: t("room.tour_tab_about_title"),
        description: t("room.tour_tab_about_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_about,
        placement: "bottom",
      },
      {
        id: "feed",
        title: t("room.tour_dm_feed_title"),
        description: t("room.tour_dm_feed_description"),
        target: CONVERSATION_TOUR_ANCHORS.feed,
        placement: "right",
      },
      {
        id: "composer",
        title: t("room.tour_dm_composer_title"),
        description: t("room.tour_dm_composer_description"),
        target: CONVERSATION_TOUR_ANCHORS.composer,
        placement: "top",
      },
    ],
  };
}

export function buildRoomConversationTour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: ROOM_CONVERSATION_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("room.tour_group_intro_title"),
        description: t("room.tour_group_intro_description"),
        image: "/nexus/stickers/guide-room.png",
        placement: "center",
      },
      {
        id: "session-switcher",
        title: t("room.tour_session_title"),
        description: t("room.tour_session_description"),
        target: CONVERSATION_TOUR_ANCHORS.session_switcher,
        placement: "bottom",
      },
      {
        id: "member-manage",
        title: t("room.tour_member_manage_title"),
        description: t("room.tour_member_manage_description"),
        target: CONVERSATION_TOUR_ANCHORS.member_manage,
        placement: "bottom",
      },
      {
        id: "history-tab",
        title: t("room.tour_tab_history_title"),
        description: t("room.tour_tab_history_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_history,
        placement: "bottom",
      },
      {
        id: "workspace-tab",
        title: t("room.tour_tab_workspace_title"),
        description: t("room.tour_tab_workspace_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_workspace,
        placement: "bottom",
      },
      {
        id: "about-tab",
        title: t("room.tour_tab_about_title"),
        description: t("room.tour_tab_about_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_about,
        placement: "bottom",
      },
      {
        id: "feed",
        title: t("room.tour_group_feed_title"),
        description: t("room.tour_group_feed_description"),
        target: CONVERSATION_TOUR_ANCHORS.feed,
        placement: "right",
      },
      {
        id: "composer",
        title: t("room.tour_group_composer_title"),
        description: t("room.tour_group_composer_description"),
        target: CONVERSATION_TOUR_ANCHORS.composer,
        placement: "top",
      },
    ],
  };
}

export function buildRoomEmptyConversationTour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: ROOM_EMPTY_CONVERSATION_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("room.tour_group_empty_intro_title"),
        description: t("room.tour_group_empty_intro_description"),
        image: "/nexus/stickers/guide-room.png",
        placement: "center",
      },
      {
        id: "session-switcher",
        title: t("room.tour_session_title"),
        description: t("room.tour_session_description"),
        target: CONVERSATION_TOUR_ANCHORS.session_switcher,
        placement: "bottom",
      },
      {
        id: "member-manage",
        title: t("room.tour_member_manage_title"),
        description: t("room.tour_member_manage_description"),
        target: CONVERSATION_TOUR_ANCHORS.member_manage,
        placement: "bottom",
      },
      {
        id: "history-tab",
        title: t("room.tour_tab_history_title"),
        description: t("room.tour_tab_history_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_history,
        placement: "bottom",
      },
      {
        id: "workspace-tab",
        title: t("room.tour_tab_workspace_title"),
        description: t("room.tour_tab_workspace_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_workspace,
        placement: "bottom",
      },
      {
        id: "about-tab",
        title: t("room.tour_tab_about_title"),
        description: t("room.tour_tab_about_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_about,
        placement: "bottom",
      },
      {
        id: "create",
        title: t("room.tour_group_empty_create_title"),
        description: t("room.tour_group_empty_create_description"),
        target: CONVERSATION_TOUR_ANCHORS.empty_create,
        placement: "right",
      },
    ],
  };
}
