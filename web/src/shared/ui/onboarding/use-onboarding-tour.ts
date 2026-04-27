"use client";

import { useContext } from "react";

import { ONBOARDING_TOUR_CONTEXT } from "@/shared/ui/onboarding/tour-context";

export function useOnboardingTour() {
  const context = useContext(ONBOARDING_TOUR_CONTEXT);
  if (!context) {
    throw new Error("useOnboardingTour 必须在 OnboardingTourProvider 内部使用");
  }
  return context;
}
