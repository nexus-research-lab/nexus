export type TourPlacement = "top" | "right" | "bottom" | "left" | "center";

export interface OnboardingTourStepItem {
  icon: "bot" | "users" | "hash" | "puzzle";
  text: string;
}

export interface OnboardingTourStep {
  description: string;
  id: string;
  image?: string;
  items?: OnboardingTourStepItem[];
  placement?: TourPlacement;
  target?: string;
  title: string;
}

export interface OnboardingTourDefinition {
  id: string;
  steps: OnboardingTourStep[];
}

export interface OnboardingTourContextValue {
  activeTourId: string | null;
  closeTour: (options?: { completed?: boolean }) => void;
  hasCompletedTour: (tourId: string) => boolean;
  isTourRegistered: (tourId: string) => boolean;
  isTourStateReady: boolean;
  nextStep: () => void;
  previousStep: () => void;
  registerTour: (tour: OnboardingTourDefinition) => void;
  resetAllTours: () => void;
  resetVersion: number;
  startTour: (tourId: string) => void;
  unregisterTour: (tourId: string) => void;
}
