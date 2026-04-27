"use client";

import { createContext } from "react";

import type { OnboardingTourContextValue } from "@/shared/ui/onboarding/tour-provider";

export const ONBOARDING_TOUR_CONTEXT = createContext<OnboardingTourContextValue | null>(null);
