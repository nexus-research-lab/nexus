/**
 * INPUT: Explicit stage-tab visibility and the current conversation runtime key.
 * OUTPUT: A short-lived backend presence lease scoped to this browser stage instance.
 * POS: Operation Stage visibility boundary; hidden persistent panels must never own a lease.
 */
import { useCallback, useEffect, useRef, useState } from "react";

import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";

import {
  closeOperationStagePresenceApi,
  touchOperationStagePresenceApi,
} from "./operation-stage-api";

const STAGE_PRESENCE_HEARTBEAT_MS = 10_000;

interface OperationStagePresenceLease {
  client_id: string;
  session_key: string;
}

export interface OperationStagePresenceController {
  activate: () => string | null;
  client_id: string | null;
  deactivate: () => void;
}

export function useOperationStagePresence(
  identity: AgentConversationIdentity | null,
  active: boolean,
): OperationStagePresenceController {
  const session_key = identity?.session_key?.trim() ?? "";
  const [lease, set_lease] = useState<OperationStagePresenceLease | null>(() => (
    active && session_key
      ? {
          client_id: create_stage_presence_client_id(),
          session_key,
        }
      : null
  ));
  const lease_ref = useRef<OperationStagePresenceLease | null>(lease);

  const deactivate = useCallback(() => {
    const current = lease_ref.current;
    if (!current) {
      return;
    }
    lease_ref.current = null;
    set_lease((candidate) => (
      candidate?.client_id === current.client_id ? null : candidate
    ));
    void closeOperationStagePresenceApi(current.session_key, current.client_id);
  }, []);

  const activate = useCallback((): string | null => {
    if (!session_key) {
      return null;
    }
    const current = lease_ref.current;
    if (current?.session_key === session_key) {
      void touchOperationStagePresenceApi(current.session_key, current.client_id);
      return current.client_id;
    }
    if (current) {
      void closeOperationStagePresenceApi(current.session_key, current.client_id);
    }
    const next = {
      client_id: create_stage_presence_client_id(),
      session_key,
    };
    lease_ref.current = next;
    set_lease(next);
    void touchOperationStagePresenceApi(next.session_key, next.client_id);
    return next.client_id;
  }, [session_key]);

  useEffect(() => {
    if (!active || !session_key) {
      deactivate();
      return;
    }

    const client_id = activate();
    if (!client_id) {
      return;
    }
    const touch = () => {
      const current = lease_ref.current;
      if (current?.session_key === session_key && current.client_id === client_id) {
        void touchOperationStagePresenceApi(session_key, client_id);
      }
    };
    const heartbeat = window.setInterval(touch, STAGE_PRESENCE_HEARTBEAT_MS);
    document.addEventListener("visibilitychange", touch);

    return () => {
      window.clearInterval(heartbeat);
      document.removeEventListener("visibilitychange", touch);
      deactivate();
    };
  }, [active, activate, deactivate, session_key]);

  return {
    activate,
    client_id: lease?.session_key === session_key ? lease.client_id : null,
    deactivate,
  };
}

function create_stage_presence_client_id(): string {
  if (typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `stage-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}
