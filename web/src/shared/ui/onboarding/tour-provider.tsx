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
import {
  hydrate_onboarding_state_from_desktop,
  read_completed_tours,
  reset_all_tour_state,
  write_completed_tours,
} from "@/shared/ui/onboarding/tour-state";

type TourPlacement = "top" | "right" | "bottom" | "left" | "center";

export interface OnboardingTourStepItem {
  icon: "bot" | "users" | "hash" | "puzzle";
  text: string;
}

export interface OnboardingTourStep {
  id: string;
  title: string;
  description: string;
  target?: string;
  placement?: TourPlacement;
  items?: OnboardingTourStepItem[];
  image?: string;
}

export interface OnboardingTourDefinition {
  id: string;
  steps: OnboardingTourStep[];
}

interface ActiveTourState {
  tour_id: string;
  step_index: number;
}

export interface OnboardingTourContextValue {
  register_tour: (tour: OnboardingTourDefinition) => void;
  unregister_tour: (tourId: string) => void;
  start_tour: (tourId: string) => void;
  close_tour: (options?: { completed?: boolean }) => void;
  next_step: () => void;
  previous_step: () => void;
  has_completed_tour: (tourId: string) => boolean;
  is_tour_registered: (tourId: string) => boolean;
  reset_all_tours: () => void;
  active_tour_id: string | null;
  is_tour_state_ready: boolean;
  reset_version: number;
}

const OnboardingTourOverlay = lazy(() =>
  import("@/shared/ui/onboarding/tour-overlay").then((m) => ({
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
    () => read_completed_tours(),
  );
  const [activeTour, setActiveTour] = useState<ActiveTourState | null>(null);
  const [isTourStateReady, setIsTourStateReady] = useState(false);
  const [resetVersion, setResetVersion] = useState(0);

  useEffect(() => {
    let cancelled = false;
    void hydrate_onboarding_state_from_desktop().then((state) => {
      if (cancelled) {
        return;
      }
      setCompletedTours(state.completed_tours);
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
      tour_id: tourId,
      step_index: 0,
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
            [currentTour.tour_id]: true,
          };
          write_completed_tours(nextValue);
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
      const currentDefinition = toursRef.current[currentTour.tour_id];
      if (!currentDefinition) {
        return null;
      }
      const nextIndex = clampStepIndex(
        currentTour.step_index + 1,
        currentDefinition.steps.length,
      );
      return {
        ...currentTour,
        step_index: nextIndex,
      };
    });
  }, []);

  const previousStep = useCallback(() => {
    setActiveTour((currentTour) => {
      if (!currentTour) {
        return null;
      }
      const currentDefinition = toursRef.current[currentTour.tour_id];
      if (!currentDefinition) {
        return null;
      }
      const nextIndex = clampStepIndex(
        currentTour.step_index - 1,
        currentDefinition.steps.length,
      );
      return {
        ...currentTour,
        step_index: nextIndex,
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
    reset_all_tour_state();
    setCompletedTours({});
    setActiveTour(null);
    setIsTourStateReady(true);
    setResetVersion((currentValue) => currentValue + 1);
  }, []);

  const contextValue = useMemo<OnboardingTourContextValue>(() => ({
    register_tour: registerTour,
    unregister_tour: unregisterTour,
    start_tour: startTour,
    close_tour: closeTour,
    next_step: nextStep,
    previous_step: previousStep,
    has_completed_tour: hasCompletedTour,
    is_tour_registered: isTourRegistered,
    reset_all_tours: resetAllTours,
    active_tour_id: activeTour?.tour_id ?? null,
    is_tour_state_ready: isTourStateReady,
    reset_version: resetVersion,
  }), [
    activeTour?.tour_id,
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
    ? toursRef.current[activeTour.tour_id] ?? null
    : null;

  return (
    <ONBOARDING_TOUR_CONTEXT.Provider value={contextValue}>
      {children}
      {activeTourDefinition && activeTour ? (
        <Suspense fallback={null}>
          <OnboardingTourOverlay
            on_close={closeTour}
            on_next={nextStep}
            on_previous={previousStep}
            step_index={activeTour.step_index}
            tour={activeTourDefinition}
          />
        </Suspense>
      ) : null}
    </ONBOARDING_TOUR_CONTEXT.Provider>
  );
}
