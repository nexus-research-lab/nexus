/**
 * INPUT: Explicit stage-tab visibility and the current conversation runtime key.
 * OUTPUT: A short-lived backend presence lease scoped to this browser stage instance.
 * POS: Operation Stage visibility boundary; hidden persistent panels must never own a lease.
 */
import { useEffect } from "react";

import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";

import {
  closeOperationStagePresenceApi,
  touchOperationStagePresenceApi,
} from "./operation-stage-api";

const STAGE_PRESENCE_HEARTBEAT_MS = 10_000;

export function useOperationStagePresence(
  identity: AgentConversationIdentity | null,
  active: boolean,
): void {
  const session_key = identity?.session_key?.trim() ?? "";

  useEffect(() => {
    if (!active || !session_key) {
      return;
    }

    const client_id = create_stage_presence_client_id();
    const touch = () => {
      void touchOperationStagePresenceApi(session_key, client_id);
    };
    touch();
    const heartbeat = window.setInterval(touch, STAGE_PRESENCE_HEARTBEAT_MS);
    document.addEventListener("visibilitychange", touch);

    return () => {
      window.clearInterval(heartbeat);
      document.removeEventListener("visibilitychange", touch);
      void closeOperationStagePresenceApi(session_key, client_id);
    };
  }, [active, session_key]);
}

function create_stage_presence_client_id(): string {
  if (typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `stage-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}
