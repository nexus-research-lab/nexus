"use client";

import { useEffect, useRef } from "react";

import { SIDEBAR_NAVIGATION_TOUR_ID } from "@/features/onboarding/tours/sidebar-navigation-tour";

const AUTO_START_DELAY_MS = 220;

interface UseAutoStartSidebarTourOptions {
  activeTourId: string | null;
  hasCompletedTour: (tourId: string) => boolean;
  isTourStateReady: boolean;
  resetVersion: number;
  startTour: (tourId: string) => void;
}

export function useAutoStartSidebarTour({
  activeTourId,
  hasCompletedTour,
  isTourStateReady,
  resetVersion,
  startTour,
}: UseAutoStartSidebarTourOptions) {
  const hasAutoStartedRef = useRef(false);

  useEffect(() => {
    hasAutoStartedRef.current = false;
  }, [resetVersion]);

  useEffect(() => {
    const shouldStart = !hasAutoStartedRef.current
      && isTourStateReady
      && !activeTourId
      && !hasCompletedTour(SIDEBAR_NAVIGATION_TOUR_ID);
    if (!shouldStart) {
      return undefined;
    }
    hasAutoStartedRef.current = true;
    const timeoutId = window.setTimeout(() => {
      startTour(SIDEBAR_NAVIGATION_TOUR_ID);
    }, AUTO_START_DELAY_MS);
    return () => window.clearTimeout(timeoutId);
  }, [activeTourId, hasCompletedTour, isTourStateReady, startTour]);
}
