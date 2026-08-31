"use client";

import { useCallback, useEffect, useState } from "react";

import { getAgentProfileTemplateApi } from "@/lib/api/agent/agent-api";

interface AgentProfileTemplateResource {
  content: string;
  error: string | null;
  loading: boolean;
  scopeKey: string;
}

const EMPTY_RESOURCE: AgentProfileTemplateResource = {
  content: "",
  error: null,
  loading: false,
  scopeKey: "",
};

export function useAgentProfileTemplate(
  enabled: boolean,
  scopeKey: string,
  fallbackError: string,
) {
  const [resource, setResource] =
    useState<AgentProfileTemplateResource>(EMPTY_RESOURCE);
  const [refreshRevision, setRefreshRevision] = useState(0);
  const retry = useCallback(() => {
    setRefreshRevision((current) => current + 1);
  }, []);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    let active = true;
    setResource({
      content: "",
      error: null,
      loading: true,
      scopeKey,
    });
    void getAgentProfileTemplateApi()
      .then((response) => {
        if (!active) {
          return;
        }
        setResource({
          content: response.content,
          error: null,
          loading: false,
          scopeKey,
        });
      })
      .catch(() => {
        if (!active) {
          return;
        }
        setResource({
          content: "",
          error: fallbackError,
          loading: false,
          scopeKey,
        });
      });
    return () => {
      active = false;
    };
  }, [enabled, fallbackError, refreshRevision, scopeKey]);

  if (!enabled) {
    return { ...EMPTY_RESOURCE, retry };
  }
  if (resource.scopeKey !== scopeKey) {
    return {
      content: "",
      error: null,
      loading: true,
      retry,
      scopeKey,
    };
  }
  return { ...resource, retry };
}
