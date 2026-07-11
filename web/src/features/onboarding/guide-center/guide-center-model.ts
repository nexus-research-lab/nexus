import {
  Compass,
  type LucideIcon,
  MessageSquare,
  Rocket,
  Wrench,
} from "lucide-react";

import {
  DM_CONVERSATION_TOUR_ID,
  ROOM_CONVERSATION_TOUR_ID,
  ROOM_EMPTY_CONVERSATION_TOUR_ID,
} from "@/features/onboarding/tours/conversation-tour";
import { LAUNCHER_TOUR_ID } from "@/features/onboarding/tours/launcher-tour";
import { SIDEBAR_NAVIGATION_TOUR_ID } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { SKILLS_TOUR_ID } from "@/features/onboarding/tours/skills-tour";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { OnboardingGuideCenterItem } from "@/shared/ui/onboarding/onboarding-guide-center";
import type {
  LauncherBootstrapResponse,
  LauncherConversationSummary,
} from "@/types/app/launcher";

export type GuideCenterTourId =
  | typeof LAUNCHER_TOUR_ID
  | typeof SIDEBAR_NAVIGATION_TOUR_ID
  | typeof DM_CONVERSATION_TOUR_ID
  | typeof ROOM_CONVERSATION_TOUR_ID
  | typeof SKILLS_TOUR_ID;

export type GuideCenterTourActions = Record<GuideCenterTourId, () => void>;

interface GuideCenterDefinition {
  completedTourIds: readonly string[];
  descriptionKey: TranslationKey;
  icon: LucideIcon;
  id: GuideCenterTourId;
  titleKey: TranslationKey;
}

export interface RoomTourNavigationTarget {
  conversationId: string | null;
  roomId: string;
  tourId:
    | typeof ROOM_CONVERSATION_TOUR_ID
    | typeof ROOM_EMPTY_CONVERSATION_TOUR_ID;
}

const GUIDE_CENTER_DEFINITIONS: readonly GuideCenterDefinition[] = [
  {
    completedTourIds: [LAUNCHER_TOUR_ID],
    descriptionKey: "launcher.tour_intro_description",
    icon: Rocket,
    id: LAUNCHER_TOUR_ID,
    titleKey: "launcher.tour_intro_title",
  },
  {
    completedTourIds: [SIDEBAR_NAVIGATION_TOUR_ID],
    descriptionKey: "sidebar.tour_intro_description",
    icon: Compass,
    id: SIDEBAR_NAVIGATION_TOUR_ID,
    titleKey: "sidebar.tour_intro_title",
  },
  {
    completedTourIds: [DM_CONVERSATION_TOUR_ID],
    descriptionKey: "room.tour_dm_intro_description",
    icon: MessageSquare,
    id: DM_CONVERSATION_TOUR_ID,
    titleKey: "room.tour_dm_intro_title",
  },
  {
    completedTourIds: [
      ROOM_CONVERSATION_TOUR_ID,
      ROOM_EMPTY_CONVERSATION_TOUR_ID,
    ],
    descriptionKey: "room.tour_group_intro_description",
    icon: MessageSquare,
    id: ROOM_CONVERSATION_TOUR_ID,
    titleKey: "room.tour_group_intro_title",
  },
  {
    completedTourIds: [SKILLS_TOUR_ID],
    descriptionKey: "capability.skills_tour_intro_description",
    icon: Wrench,
    id: SKILLS_TOUR_ID,
    titleKey: "capability.skills_tour_intro_title",
  },
];

const REGISTERED_ROOM_TOUR_PRIORITY = [
  ROOM_CONVERSATION_TOUR_ID,
  ROOM_EMPTY_CONVERSATION_TOUR_ID,
] as const;

export function buildGuideCenterItems(
  t: I18nContextValue["t"],
  hasCompletedTour: (tourId: string) => boolean,
  actions: GuideCenterTourActions,
): OnboardingGuideCenterItem[] {
  return GUIDE_CENTER_DEFINITIONS.map((definition) => ({
    actionLabel: t("common.view_guide"),
    completed: definition.completedTourIds.some(hasCompletedTour),
    description: t(definition.descriptionKey),
    icon: definition.icon,
    id: definition.id,
    onAction: actions[definition.id],
    title: t(definition.titleKey),
  }));
}

export function resolveRegisteredRoomTourId(
  isTourRegistered: (tourId: string) => boolean,
): RoomTourNavigationTarget["tourId"] | null {
  return REGISTERED_ROOM_TOUR_PRIORITY.find(isTourRegistered) ?? null;
}

export function resolveRoomTourNavigationTarget(
  payload: LauncherBootstrapResponse,
): RoomTourNavigationTarget | null {
  const room = payload.rooms.find((candidate) => candidate.room_type === "room");
  if (!room) {
    return null;
  }
  const conversation = payload.conversations
    .filter((candidate) => candidate.room_id === room.id && candidate.conversation_id)
    .reduce<LauncherConversationSummary | null>(selectLatestConversation, null);
  return {
    conversationId: conversation?.conversation_id ?? null,
    roomId: room.id,
    tourId: conversation
      ? ROOM_CONVERSATION_TOUR_ID
      : ROOM_EMPTY_CONVERSATION_TOUR_ID,
  };
}

function selectLatestConversation(
  latest: LauncherConversationSummary | null,
  candidate: LauncherConversationSummary,
): LauncherConversationSummary {
  if (!latest) {
    return candidate;
  }
  return activityTime(candidate.last_activity) > activityTime(latest.last_activity)
    ? candidate
    : latest;
}

function activityTime(value: string): number {
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}
