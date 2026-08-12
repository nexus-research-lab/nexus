import { useMemo } from "react";

import {
  buildDmConversationTour,
  buildRoomConversationTour,
  buildRoomEmptyConversationTour,
} from "@/features/onboarding/tours/conversation-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";

const TOUR_BUILDERS = {
  dm: buildDmConversationTour,
  room: buildRoomConversationTour,
  empty: buildRoomEmptyConversationTour,
} as const;

interface UseRoomPageTourOptions {
  roomType: string | null;
  hasConversation: boolean;
  enabled: boolean;
}

export function useRoomPageTour({
  roomType,
  hasConversation,
  enabled,
}: UseRoomPageTourOptions) {
  const {t} = useI18n();
  const tour = useMemo(() => {
    const tourKind: keyof typeof TOUR_BUILDERS | null = !roomType
      ? null
      : roomType === "dm"
        ? "dm"
        : hasConversation
          ? "room"
          : "empty";
    return tourKind ? TOUR_BUILDERS[tourKind](t) : null;
  }, [hasConversation, roomType, t]);

  return usePageOnboardingTour({
    tour,
    enabled,
    autoStartDelayMs: 260,
  });
}
