import { useCallback, useEffect, useState } from "react";

import { getConnectorsApi } from "@/lib/api/capability/connector-api";
import type { ConnectorInfo } from "@/types/capability/connector";

interface AgentConnectorsState {
  error: string | null;
  items: ConnectorInfo[];
  loading: boolean;
}

const EMPTY_STATE: AgentConnectorsState = {
  error: null,
  items: [],
  loading: false,
};

export function useAgentConnectors(isVisible: boolean, fallbackError: string) {
  const [state, setState] = useState<AgentConnectorsState>(EMPTY_STATE);
  const [refreshRevision, setRefreshRevision] = useState(0);
  const retry = useCallback(() => {
    setRefreshRevision((current) => current + 1);
  }, []);

  useEffect(() => {
    if (!isVisible) return undefined;
    let active = true;
    setState((current) => ({ ...current, error: null, loading: true }));
    void getConnectorsApi({ status: "available" })
      .then((items) => {
        if (active) setState({ error: null, items, loading: false });
      })
      .catch(() => {
        if (!active) return;
        setState((current) => ({
          error: fallbackError,
          items: current.items,
          loading: false,
        }));
      });
    return () => {
      active = false;
    };
  }, [fallbackError, isVisible, refreshRevision]);

  return isVisible
    ? { ...state, retry }
    : { ...state, loading: false, retry };
}
