"use client";

import { useHomeAgentSessionController } from "@/hooks/use-home-agent-session-controller";

export function useLauncherPageController() {
  return useHomeAgentSessionController();
}
