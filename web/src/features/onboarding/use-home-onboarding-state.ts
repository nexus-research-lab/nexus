import { useEffect, useState } from "react";

import {
  getHomeOnboardingReturnPath,
  getHomeOnboardingStage,
  isHomeOnboardingCompleted,
} from "./home-agent-onboarding";

export function useHomeOnboardingState() {
  const [, setRevision] = useState(0);

  useEffect(() => {
    const refresh = () => setRevision((current) => current + 1);
    window.addEventListener("storage", refresh);
    window.addEventListener("nexus:home-onboarding-state-change", refresh);
    return () => {
      window.removeEventListener("storage", refresh);
      window.removeEventListener(
        "nexus:home-onboarding-state-change",
        refresh,
      );
    };
  }, []);

  return {
    active: !isHomeOnboardingCompleted(),
    returnPath: getHomeOnboardingReturnPath(),
    stage: getHomeOnboardingStage(),
  };
}
