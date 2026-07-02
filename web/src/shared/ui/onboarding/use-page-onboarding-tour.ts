"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-provider";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import {
  clear_requested_tour_id,
  is_tour_dismissed,
  read_requested_tour_id,
  set_tour_dismissed,
} from "@/shared/ui/onboarding/tour-state";

interface UsePageOnboardingTourOptions {
  tour: OnboardingTourDefinition | null;
  enabled?: boolean;
  auto_start_delay_ms?: number;
}

export function usePageOnboardingTour({
  tour,
  enabled = true,
  auto_start_delay_ms: autoStartDelayMs = 220,
}: UsePageOnboardingTourOptions) {
  const {
    active_tour_id: activeTourId,
    close_tour: closeTour,
    has_completed_tour: hasCompletedTour,
    is_tour_state_ready: isTourStateReady,
    register_tour: registerTour,
    reset_version: resetVersion,
    start_tour: startTour,
    unregister_tour: unregisterTour,
  } = useOnboardingTour();
  const autoStartedTourIdsRef = useRef<Set<string>>(new Set());
  const previousActiveTourIdRef = useRef<string | null>(null);

  useEffect(() => {
    autoStartedTourIdsRef.current.clear();
  }, [resetVersion]);

  useEffect(() => {
    if (!tour || !enabled || !isTourStateReady) {
      return undefined;
    }

    registerTour(tour);
    return () => {
      unregisterTour(tour.id);
    };
  }, [enabled, isTourStateReady, registerTour, tour, unregisterTour]);

  useEffect(() => {
    const previousActiveTourId = previousActiveTourIdRef.current;
    const currentTourId = tour?.id ?? null;

    if (
      previousActiveTourId &&
      previousActiveTourId === currentTourId &&
      activeTourId !== currentTourId &&
      currentTourId &&
      !hasCompletedTour(currentTourId)
    ) {
      set_tour_dismissed(currentTourId, true);
    }

    previousActiveTourIdRef.current = activeTourId;
  }, [activeTourId, hasCompletedTour, tour]);

  useEffect(() => {
    if (!tour || !enabled || !isTourStateReady) {
      return undefined;
    }
    if (activeTourId) {
      return undefined;
    }

    const requestedTourId = read_requested_tour_id();
    if (requestedTourId !== tour.id) {
      return undefined;
    }

    const timeoutId = window.setTimeout(() => {
      clear_requested_tour_id(tour.id);
      set_tour_dismissed(tour.id, false);
      startTour(tour.id);
    }, 120);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [activeTourId, enabled, isTourStateReady, startTour, tour]);

  useEffect(() => {
    if (!tour || !enabled || !isTourStateReady) {
      return undefined;
    }
    if (activeTourId) {
      return undefined;
    }
    if (hasCompletedTour(tour.id)) {
      return undefined;
    }
    if (is_tour_dismissed(tour.id)) {
      return undefined;
    }
    if (autoStartedTourIdsRef.current.has(tour.id)) {
      return undefined;
    }

    autoStartedTourIdsRef.current.add(tour.id);
    const timeoutId = window.setTimeout(() => {
      startTour(tour.id);
    }, autoStartDelayMs);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [
    activeTourId,
    autoStartDelayMs,
    enabled,
    hasCompletedTour,
    isTourStateReady,
    startTour,
    tour,
  ]);

  const startCurrentTour = useCallback(() => {
    if (!tour) {
      return;
    }
    set_tour_dismissed(tour.id, false);
    startTour(tour.id);
  }, [startTour, tour]);

  const closeCurrentTour = useCallback(() => {
    if (!tour) {
      return;
    }
    set_tour_dismissed(tour.id, true);
    closeTour();
  }, [closeTour, tour]);

  return useMemo(
    () => ({
      active_tour_id: activeTourId,
      close_current_tour: closeCurrentTour,
      has_completed_current_tour: tour ? hasCompletedTour(tour.id) : false,
      is_current_tour_running: tour ? activeTourId === tour.id : false,
      start_current_tour: startCurrentTour,
    }),
    [
      activeTourId,
      closeCurrentTour,
      hasCompletedTour,
      startCurrentTour,
      tour,
    ],
  );
}
