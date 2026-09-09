import { useEffect, useState, useSyncExternalStore } from "react";

import { bootstrapTeam, type TeamBootstrap } from "@/lib/api/conversation/team-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";

export function useTeamBootstrap(): TeamBootstrap | null {
  const [bootstrap, setBootstrap] = useState<TeamBootstrap | null>(null);
  const generation = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );

  useEffect(() => {
    const controller = new AbortController();
    setBootstrap(null);
    void bootstrapTeam(controller.signal).then((value) => {
      if (isAuthOwnerScopeGenerationCurrent(generation)) {
        setBootstrap(value);
      }
    }).catch((error: unknown) => {
      if (!(error instanceof ApiRequestError && error.status === 404)) {
        console.warn("Team bootstrap failed", error);
      }
    });
    return () => controller.abort();
  }, [generation]);

  return bootstrap;
}
