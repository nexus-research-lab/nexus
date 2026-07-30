"use client";

import { useEffect, useState } from "react";

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
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        setResource({
          content: "",
          error: error instanceof Error ? error.message : fallbackError,
          loading: false,
          scopeKey,
        });
      });
    return () => {
      active = false;
    };
  }, [enabled, fallbackError, scopeKey]);

  if (!enabled) {
    return EMPTY_RESOURCE;
  }
  if (resource.scopeKey !== scopeKey) {
    return {
      content: "",
      error: null,
      loading: true,
      scopeKey,
    };
  }
  return resource;
}
