"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-provider";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import {
  clearRequestedTourId,
  isTourDismissed,
  readRequestedTourId,
  setTourDismissed,
} from "@/shared/ui/onboarding/tour-state";

interface UsePageOnboardingTourOptions {
  tour: OnboardingTourDefinition | null;
  enabled?: boolean;
  autoStartDelayMs?: number;
}

export function usePageOnboardingTour({
  tour,
  enabled = true,
  autoStartDelayMs: autoStartDelayMs = 220,
}: UsePageOnboardingTourOptions) {
  const {
    activeTourId: activeTourId,
    closeTour: closeTour,
    hasCompletedTour: hasCompletedTour,
    isTourStateReady: isTourStateReady,
    registerTour: registerTour,
    resetVersion: resetVersion,
    startTour: startTour,
    unregisterTour: unregisterTour,
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
      setTourDismissed(currentTourId, true);
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

    const requestedTourId = readRequestedTourId();
    if (requestedTourId !== tour.id) {
      return undefined;
    }

    const timeoutId = window.setTimeout(() => {
      clearRequestedTourId(tour.id);
      setTourDismissed(tour.id, false);
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
    if (isTourDismissed(tour.id)) {
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
    setTourDismissed(tour.id, false);
    startTour(tour.id);
  }, [startTour, tour]);

  const closeCurrentTour = useCallback(() => {
    if (!tour) {
      return;
    }
    setTourDismissed(tour.id, true);
    closeTour();
  }, [closeTour, tour]);

  return useMemo(
    () => ({
      activeTourId: activeTourId,
      closeCurrentTour: closeCurrentTour,
      hasCompletedCurrentTour: tour ? hasCompletedTour(tour.id) : false,
      isCurrentTourRunning: tour ? activeTourId === tour.id : false,
      startCurrentTour: startCurrentTour,
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
