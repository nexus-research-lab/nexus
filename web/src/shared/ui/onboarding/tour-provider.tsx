"use client";

import {
  lazy,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  Suspense,
} from "react";

import { ONBOARDING_TOUR_CONTEXT } from "@/shared/ui/onboarding/tour-context";
import type {
  OnboardingTourContextValue,
  OnboardingTourDefinition,
} from "@/shared/ui/onboarding/tour-contract";
import {
  hydrateOnboardingStateFromDesktop,
  readCompletedTours,
  resetAllTourState,
  writeCompletedTours,
} from "@/shared/ui/onboarding/tour-state";

interface ActiveTourState {
  tourId: string;
  stepIndex: number;
}

const OnboardingTourOverlay = lazy(() =>
  import("@/shared/ui/onboarding/overlay/tour-overlay").then((m) => ({
    default: m.OnboardingTourOverlay,
  })),
);

function clampStepIndex(stepIndex: number, stepsCount: number): number {
  if (stepsCount <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(stepIndex, stepsCount - 1));
}

export function OnboardingTourProvider({ children }: { children: ReactNode }) {
  const toursRef = useRef<Record<string, OnboardingTourDefinition>>({});
  const [completedTours, setCompletedTours] = useState<Record<string, boolean>>(
    () => readCompletedTours(),
  );
  const [activeTour, setActiveTour] = useState<ActiveTourState | null>(null);
  const [isTourStateReady, setIsTourStateReady] = useState(false);
  const [resetVersion, setResetVersion] = useState(0);

  useEffect(() => {
    let cancelled = false;
    void hydrateOnboardingStateFromDesktop().then((state) => {
      if (cancelled) {
        return;
      }
      setCompletedTours(state.completedTours);
      setIsTourStateReady(true);
    }).catch(() => {
      if (!cancelled) {
        setIsTourStateReady(true);
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  const registerTour = useCallback((tour: OnboardingTourDefinition) => {
    toursRef.current[tour.id] = tour;
  }, []);

  const unregisterTour = useCallback((tourId: string) => {
    delete toursRef.current[tourId];
  }, []);

  const startTour = useCallback((tourId: string) => {
    const tour = toursRef.current[tourId];
    if (!tour || tour.steps.length === 0) {
      return;
    }

    setActiveTour({
      tourId: tourId,
      stepIndex: 0,
    });
  }, []);

  const closeTour = useCallback((options?: { completed?: boolean }) => {
    setActiveTour((currentTour) => {
      if (!currentTour) {
        return null;
      }

      if (options?.completed) {
        setCompletedTours((previous) => {
          const nextValue = {
            ...previous,
            [currentTour.tourId]: true,
          };
          writeCompletedTours(nextValue);
          return nextValue;
        });
      }

      return null;
    });
  }, []);

  const nextStep = useCallback(() => {
    setActiveTour((currentTour) => {
      if (!currentTour) {
        return null;
      }
      const currentDefinition = toursRef.current[currentTour.tourId];
      if (!currentDefinition) {
        return null;
      }
      const nextIndex = clampStepIndex(
        currentTour.stepIndex + 1,
        currentDefinition.steps.length,
      );
      return {
        ...currentTour,
        stepIndex: nextIndex,
      };
    });
  }, []);

  const previousStep = useCallback(() => {
    setActiveTour((currentTour) => {
      if (!currentTour) {
        return null;
      }
      const currentDefinition = toursRef.current[currentTour.tourId];
      if (!currentDefinition) {
        return null;
      }
      const nextIndex = clampStepIndex(
        currentTour.stepIndex - 1,
        currentDefinition.steps.length,
      );
      return {
        ...currentTour,
        stepIndex: nextIndex,
      };
    });
  }, []);

  const hasCompletedTour = useCallback((tourId: string) => {
    return Boolean(completedTours[tourId]);
  }, [completedTours]);

  const isTourRegistered = useCallback((tourId: string) => {
    return Boolean(toursRef.current[tourId]);
  }, []);

  const resetAllTours = useCallback(() => {
    resetAllTourState();
    setCompletedTours({});
    setActiveTour(null);
    setIsTourStateReady(true);
    setResetVersion((currentValue) => currentValue + 1);
  }, []);

  const contextValue = useMemo<OnboardingTourContextValue>(() => ({
    registerTour: registerTour,
    unregisterTour: unregisterTour,
    startTour: startTour,
    closeTour: closeTour,
    nextStep: nextStep,
    previousStep: previousStep,
    hasCompletedTour: hasCompletedTour,
    isTourRegistered: isTourRegistered,
    resetAllTours: resetAllTours,
    activeTourId: activeTour?.tourId ?? null,
    isTourStateReady: isTourStateReady,
    resetVersion: resetVersion,
  }), [
    activeTour?.tourId,
    closeTour,
    hasCompletedTour,
    isTourRegistered,
    isTourStateReady,
    nextStep,
    previousStep,
    registerTour,
    resetVersion,
    resetAllTours,
    startTour,
    unregisterTour,
  ]);

  const activeTourDefinition = activeTour
    ? toursRef.current[activeTour.tourId] ?? null
    : null;

  return (
    <ONBOARDING_TOUR_CONTEXT.Provider value={contextValue}>
      {children}
      {activeTourDefinition && activeTour ? (
        <Suspense fallback={null}>
          <OnboardingTourOverlay
            onClose={closeTour}
            onNext={nextStep}
            onPrevious={previousStep}
            stepIndex={activeTour.stepIndex}
            tour={activeTourDefinition}
          />
        </Suspense>
      ) : null}
    </ONBOARDING_TOUR_CONTEXT.Provider>
  );
}
