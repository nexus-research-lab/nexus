"use client";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-provider";

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

export function build_dm_conversation_tour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: DM_CONVERSATION_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("room.tour_dm_intro_title"),
        description: t("room.tour_dm_intro_description"),
        image: "/nexus/listening.png",
        placement: "center",
      },
      {
        id: "session-switcher",
        title: t("room.tour_session_title"),
        description: t("room.tour_session_description"),
        target: CONVERSATION_TOUR_ANCHORS.session_switcher,
        image: "/nexus/reading.png",
        placement: "bottom",
      },
      {
        id: "chat-tab",
        title: t("room.tour_tab_chat_title"),
        description: t("room.tour_tab_chat_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_chat,
        image: "/nexus/listening.png",
        placement: "bottom",
      },
      {
        id: "history-tab",
        title: t("room.tour_tab_history_title"),
        description: t("room.tour_tab_history_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_history,
        image: "/nexus/reading.png",
        placement: "bottom",
      },
      {
        id: "workspace-tab",
        title: t("room.tour_tab_workspace_title"),
        description: t("room.tour_tab_workspace_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_workspace,
        image: "/nexus/working.png",
        placement: "bottom",
      },
      {
        id: "about-tab",
        title: t("room.tour_tab_about_title"),
        description: t("room.tour_tab_about_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_about,
        image: "/nexus/pointing.png",
        placement: "bottom",
      },
      {
        id: "feed",
        title: t("room.tour_dm_feed_title"),
        description: t("room.tour_dm_feed_description"),
        target: CONVERSATION_TOUR_ANCHORS.feed,
        image: "/nexus/reviewing.png",
        placement: "right",
      },
      {
        id: "composer",
        title: t("room.tour_dm_composer_title"),
        description: t("room.tour_dm_composer_description"),
        target: CONVERSATION_TOUR_ANCHORS.composer,
        image: "/nexus/writing.png",
        placement: "top",
      },
    ],
  };
}

export function build_room_conversation_tour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: ROOM_CONVERSATION_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("room.tour_group_intro_title"),
        description: t("room.tour_group_intro_description"),
        image: "/nexus/in-room.png",
        placement: "center",
      },
      {
        id: "session-switcher",
        title: t("room.tour_session_title"),
        description: t("room.tour_session_description"),
        target: CONVERSATION_TOUR_ANCHORS.session_switcher,
        image: "/nexus/discussing.png",
        placement: "bottom",
      },
      {
        id: "member-manage",
        title: t("room.tour_member_manage_title"),
        description: t("room.tour_member_manage_description"),
        target: CONVERSATION_TOUR_ANCHORS.member_manage,
        image: "/nexus/discussing.png",
        placement: "bottom",
      },
      {
        id: "chat-tab",
        title: t("room.tour_tab_chat_title"),
        description: t("room.tour_tab_chat_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_chat,
        image: "/nexus/listening.png",
        placement: "bottom",
      },
      {
        id: "history-tab",
        title: t("room.tour_tab_history_title"),
        description: t("room.tour_tab_history_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_history,
        image: "/nexus/reading.png",
        placement: "bottom",
      },
      {
        id: "workspace-tab",
        title: t("room.tour_tab_workspace_title"),
        description: t("room.tour_tab_workspace_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_workspace,
        image: "/nexus/working.png",
        placement: "bottom",
      },
      {
        id: "about-tab",
        title: t("room.tour_tab_about_title"),
        description: t("room.tour_tab_about_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_about,
        image: "/nexus/pointing.png",
        placement: "bottom",
      },
      {
        id: "feed",
        title: t("room.tour_group_feed_title"),
        description: t("room.tour_group_feed_description"),
        target: CONVERSATION_TOUR_ANCHORS.feed,
        image: "/nexus/working.png",
        placement: "right",
      },
      {
        id: "composer",
        title: t("room.tour_group_composer_title"),
        description: t("room.tour_group_composer_description"),
        target: CONVERSATION_TOUR_ANCHORS.composer,
        image: "/nexus/writing.png",
        placement: "top",
      },
    ],
  };
}

export function build_room_empty_conversation_tour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: ROOM_EMPTY_CONVERSATION_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("room.tour_group_empty_intro_title"),
        description: t("room.tour_group_empty_intro_description"),
        image: "/nexus/in-room.png",
        placement: "center",
      },
      {
        id: "session-switcher",
        title: t("room.tour_session_title"),
        description: t("room.tour_session_description"),
        target: CONVERSATION_TOUR_ANCHORS.session_switcher,
        image: "/nexus/discussing.png",
        placement: "bottom",
      },
      {
        id: "member-manage",
        title: t("room.tour_member_manage_title"),
        description: t("room.tour_member_manage_description"),
        target: CONVERSATION_TOUR_ANCHORS.member_manage,
        image: "/nexus/discussing.png",
        placement: "bottom",
      },
      {
        id: "chat-tab",
        title: t("room.tour_tab_chat_title"),
        description: t("room.tour_tab_chat_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_chat,
        image: "/nexus/listening.png",
        placement: "bottom",
      },
      {
        id: "history-tab",
        title: t("room.tour_tab_history_title"),
        description: t("room.tour_tab_history_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_history,
        image: "/nexus/reading.png",
        placement: "bottom",
      },
      {
        id: "workspace-tab",
        title: t("room.tour_tab_workspace_title"),
        description: t("room.tour_tab_workspace_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_workspace,
        image: "/nexus/working.png",
        placement: "bottom",
      },
      {
        id: "about-tab",
        title: t("room.tour_tab_about_title"),
        description: t("room.tour_tab_about_description"),
        target: CONVERSATION_TOUR_ANCHORS.tab_about,
        image: "/nexus/pointing.png",
        placement: "bottom",
      },
      {
        id: "create",
        title: t("room.tour_group_empty_create_title"),
        description: t("room.tour_group_empty_create_description"),
        target: CONVERSATION_TOUR_ANCHORS.empty_create,
        image: "/nexus/pointing.png",
        placement: "right",
      },
    ],
  };
}
