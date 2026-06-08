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
  unregister_tour: (tour_id: string) => void;
  start_tour: (tour_id: string) => void;
  close_tour: (options?: { completed?: boolean }) => void;
  next_step: () => void;
  previous_step: () => void;
  has_completed_tour: (tour_id: string) => boolean;
  is_tour_registered: (tour_id: string) => boolean;
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

function clamp_step_index(step_index: number, steps_count: number): number {
  if (steps_count <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(step_index, steps_count - 1));
}

export function OnboardingTourProvider({ children }: { children: ReactNode }) {
  const tours_ref = useRef<Record<string, OnboardingTourDefinition>>({});
  const [completed_tours, set_completed_tours] = useState<Record<string, boolean>>(
    () => read_completed_tours(),
  );
  const [active_tour, set_active_tour] = useState<ActiveTourState | null>(null);
  const [is_tour_state_ready, set_is_tour_state_ready] = useState(false);
  const [reset_version, set_reset_version] = useState(0);

  useEffect(() => {
    let cancelled = false;
    void hydrate_onboarding_state_from_desktop().then((state) => {
      if (cancelled) {
        return;
      }
      set_completed_tours(state.completed_tours);
      set_is_tour_state_ready(true);
    }).catch(() => {
      if (!cancelled) {
        set_is_tour_state_ready(true);
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  const register_tour = useCallback((tour: OnboardingTourDefinition) => {
    tours_ref.current[tour.id] = tour;
  }, []);

  const unregister_tour = useCallback((tour_id: string) => {
    delete tours_ref.current[tour_id];
  }, []);

  const start_tour = useCallback((tour_id: string) => {
    const tour = tours_ref.current[tour_id];
    if (!tour || tour.steps.length === 0) {
      return;
    }

    set_active_tour({
      tour_id,
      step_index: 0,
    });
  }, []);

  const close_tour = useCallback((options?: { completed?: boolean }) => {
    set_active_tour((current_tour) => {
      if (!current_tour) {
        return null;
      }

      if (options?.completed) {
        set_completed_tours((previous) => {
          const next_value = {
            ...previous,
            [current_tour.tour_id]: true,
          };
          write_completed_tours(next_value);
          return next_value;
        });
      }

      return null;
    });
  }, []);

  const next_step = useCallback(() => {
    set_active_tour((current_tour) => {
      if (!current_tour) {
        return null;
      }
      const current_definition = tours_ref.current[current_tour.tour_id];
      if (!current_definition) {
        return null;
      }
      const next_index = clamp_step_index(
        current_tour.step_index + 1,
        current_definition.steps.length,
      );
      return {
        ...current_tour,
        step_index: next_index,
      };
    });
  }, []);

  const previous_step = useCallback(() => {
    set_active_tour((current_tour) => {
      if (!current_tour) {
        return null;
      }
      const current_definition = tours_ref.current[current_tour.tour_id];
      if (!current_definition) {
        return null;
      }
      const next_index = clamp_step_index(
        current_tour.step_index - 1,
        current_definition.steps.length,
      );
      return {
        ...current_tour,
        step_index: next_index,
      };
    });
  }, []);

  const has_completed_tour = useCallback((tour_id: string) => {
    return Boolean(completed_tours[tour_id]);
  }, [completed_tours]);

  const is_tour_registered = useCallback((tour_id: string) => {
    return Boolean(tours_ref.current[tour_id]);
  }, []);

  const reset_all_tours = useCallback(() => {
    reset_all_tour_state();
    set_completed_tours({});
    set_active_tour(null);
    set_is_tour_state_ready(true);
    set_reset_version((current_value) => current_value + 1);
  }, []);

  const context_value = useMemo<OnboardingTourContextValue>(() => ({
    register_tour,
    unregister_tour,
    start_tour,
    close_tour,
    next_step,
    previous_step,
    has_completed_tour,
    is_tour_registered,
    reset_all_tours,
    active_tour_id: active_tour?.tour_id ?? null,
    is_tour_state_ready,
    reset_version,
  }), [
    active_tour?.tour_id,
    close_tour,
    has_completed_tour,
    is_tour_registered,
    is_tour_state_ready,
    next_step,
    previous_step,
    register_tour,
    reset_version,
    reset_all_tours,
    start_tour,
    unregister_tour,
  ]);

  const active_tour_definition = active_tour
    ? tours_ref.current[active_tour.tour_id] ?? null
    : null;

  return (
    <ONBOARDING_TOUR_CONTEXT.Provider value={context_value}>
      {children}
      {active_tour_definition && active_tour ? (
        <Suspense fallback={null}>
          <OnboardingTourOverlay
            on_close={close_tour}
            on_next={next_step}
            on_previous={previous_step}
            step_index={active_tour.step_index}
            tour={active_tour_definition}
          />
        </Suspense>
      ) : null}
    </ONBOARDING_TOUR_CONTEXT.Provider>
  );
}
