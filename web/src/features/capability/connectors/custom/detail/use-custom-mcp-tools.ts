// INPUT: 当前自定义 MCP 配置身份与工具目录 API。
// OUTPUT: 按配置身份隔离快照和失败，并拒绝旧响应的工具目录快照、失败和显式刷新动作。
// POS: 自定义 MCP 详情页工具发现资源控制器。
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getCustomMCPToolsApi } from "@/lib/api/capability/connector-api";
import {
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  CustomMCPServer,
  CustomMCPToolCatalog,
} from "@/types/capability/connector";

export function useCustomMCPTools(server: CustomMCPServer | null) {
  const { t } = useI18n();
  const requestIdRef = useRef(0);
  const [catalogSnapshot, setCatalog] = useState<{ identity: string; value: CustomMCPToolCatalog } | null>(null);
  const [failureSnapshot, setFailure] = useState<{ identity: string; value: ResourceFailure } | null>(null);
  const [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0);
  const identity = useMemo(() => server ? [
    server.connector_id,
    server.configuration_state,
    server.enabled ? "enabled" : "disabled",
    server.type,
    server.command ?? "",
    ...(server.args ?? []),
    server.url ?? "",
    server.auth_type ?? "",
  ].join("\0") : "", [server]);

  useEffect(() => {
    if (!server || server.configuration_state !== "ready") {
      requestIdRef.current += 1;
      setCatalog(null);
      setFailure(null);
      setLoading(false);
      return undefined;
    }
    if (!server.enabled) {
      requestIdRef.current += 1;
      setFailure(null);
      setLoading(false);
      return undefined;
    }
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setFailure(null);
    void getCustomMCPToolsApi(server.connector_id)
      .then((nextCatalog) => {
        if (requestId !== requestIdRef.current) return;
        setCatalog({ identity, value: nextCatalog });
      })
      .catch((error: unknown) => {
        if (requestId !== requestIdRef.current) return;
        const failure = getResourceFailure(error, t("capability.custom_mcp_tools_load_failed"));
        if (failure.access) setCatalog(null);
        setFailure({ identity, value: failure });
      })
      .finally(() => {
        if (requestId === requestIdRef.current) setLoading(false);
      });
    return () => {
      if (requestId === requestIdRef.current) requestIdRef.current += 1;
    };
  }, [identity, revision, server, t]);

  const refresh = useCallback(() => {
    setRevision((current) => current + 1);
  }, []);

  const catalog = catalogSnapshot?.identity === identity ? catalogSnapshot.value : null;
  const failure = failureSnapshot?.identity === identity ? failureSnapshot.value : null;

  return { catalog, failure, loading, refresh };
}
