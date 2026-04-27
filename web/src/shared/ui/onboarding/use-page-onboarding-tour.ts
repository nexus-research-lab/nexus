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
  auto_start_delay_ms = 220,
}: UsePageOnboardingTourOptions) {
  const {
    active_tour_id,
    close_tour,
    has_completed_tour,
    register_tour,
    reset_version,
    start_tour,
    unregister_tour,
  } = useOnboardingTour();
  const auto_started_tour_ids_ref = useRef<Set<string>>(new Set());
  const previous_active_tour_id_ref = useRef<string | null>(null);

  useEffect(() => {
    auto_started_tour_ids_ref.current.clear();
  }, [reset_version]);

  useEffect(() => {
    if (!tour || !enabled) {
      return undefined;
    }

    register_tour(tour);
    return () => {
      unregister_tour(tour.id);
    };
  }, [enabled, register_tour, tour, unregister_tour]);

  useEffect(() => {
    const previous_active_tour_id = previous_active_tour_id_ref.current;
    const current_tour_id = tour?.id ?? null;

    if (
      previous_active_tour_id &&
      previous_active_tour_id === current_tour_id &&
      active_tour_id !== current_tour_id &&
      current_tour_id &&
      !has_completed_tour(current_tour_id)
    ) {
      set_tour_dismissed(current_tour_id, true);
    }

    previous_active_tour_id_ref.current = active_tour_id;
  }, [active_tour_id, has_completed_tour, tour]);

  useEffect(() => {
    if (!tour || !enabled) {
      return undefined;
    }
    if (active_tour_id) {
      return undefined;
    }

    const requested_tour_id = read_requested_tour_id();
    if (requested_tour_id !== tour.id) {
      return undefined;
    }

    const timeout_id = window.setTimeout(() => {
      clear_requested_tour_id(tour.id);
      set_tour_dismissed(tour.id, false);
      start_tour(tour.id);
    }, 120);

    return () => {
      window.clearTimeout(timeout_id);
    };
  }, [active_tour_id, enabled, start_tour, tour]);

  useEffect(() => {
    if (!tour || !enabled) {
      return undefined;
    }
    if (active_tour_id) {
      return undefined;
    }
    if (has_completed_tour(tour.id)) {
      return undefined;
    }
    if (is_tour_dismissed(tour.id)) {
      return undefined;
    }
    if (auto_started_tour_ids_ref.current.has(tour.id)) {
      return undefined;
    }

    auto_started_tour_ids_ref.current.add(tour.id);
    const timeout_id = window.setTimeout(() => {
      start_tour(tour.id);
    }, auto_start_delay_ms);

    return () => {
      window.clearTimeout(timeout_id);
    };
  }, [
    active_tour_id,
    auto_start_delay_ms,
    enabled,
    has_completed_tour,
    start_tour,
    tour,
  ]);

  const start_current_tour = useCallback(() => {
    if (!tour) {
      return;
    }
    set_tour_dismissed(tour.id, false);
    start_tour(tour.id);
  }, [start_tour, tour]);

  const close_current_tour = useCallback(() => {
    if (!tour) {
      return;
    }
    set_tour_dismissed(tour.id, true);
    close_tour();
  }, [close_tour, tour]);

  return useMemo(
    () => ({
      active_tour_id,
      close_current_tour,
      has_completed_current_tour: tour ? has_completed_tour(tour.id) : false,
      is_current_tour_running: tour ? active_tour_id === tour.id : false,
      start_current_tour,
    }),
    [
      active_tour_id,
      close_current_tour,
      has_completed_tour,
      start_current_tour,
      tour,
    ],
  );
}
