// INPUT: 当前自定义 MCP 配置身份与工具目录 API。
// OUTPUT: 拒绝旧响应的工具目录快照、失败和显式刷新动作。
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
  const [catalog, setCatalog] = useState<CustomMCPToolCatalog | null>(null);
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
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
        setCatalog(nextCatalog);
      })
      .catch((error: unknown) => {
        if (requestId !== requestIdRef.current) return;
        setFailure(getResourceFailure(
          error,
          t("capability.custom_mcp_tools_load_failed"),
        ));
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

  return { catalog, failure, loading, refresh };
}
