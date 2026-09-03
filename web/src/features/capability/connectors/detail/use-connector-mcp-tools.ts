// INPUT: 当前固定 Connector 详情与 MCP tools/list API。
// OUTPUT: 拒绝旧响应的固定 Connector 工具目录、失败和显式刷新动作。
// POS: 固定 MCP Connector 详情页的只读资源控制器；未连接时不发请求。
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getConnectorMCPToolsApi } from "@/lib/api/capability/connector-api";
import {
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import type {
  ConnectorDetail,
  CustomMCPToolCatalog,
} from "@/types/capability/connector";

export function useConnectorMCPTools(detail: ConnectorDetail | null) {
  const requestIdRef = useRef(0);
  const [catalog, setCatalog] = useState<CustomMCPToolCatalog | null>(null);
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
  const [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0);
  const supported = detail?.connector_id === "richmail";
  const connected = detail?.connection_state === "connected";
  const identity = useMemo(() => detail ? [
    detail.connector_id,
    detail.connection_state,
    detail.mcp_server_url ?? "",
  ].join("\0") : "", [detail]);

  useEffect(() => {
    if (!detail || !supported || !connected) {
      requestIdRef.current += 1;
      setCatalog(null);
      setFailure(null);
      setLoading(false);
      return undefined;
    }
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setFailure(null);
    void getConnectorMCPToolsApi(detail.connector_id)
      .then((nextCatalog) => {
        if (requestId === requestIdRef.current) setCatalog(nextCatalog);
      })
      .catch((error: unknown) => {
        if (requestId !== requestIdRef.current) return;
        setFailure(getResourceFailure(error, "工具读取失败"));
      })
      .finally(() => {
        if (requestId === requestIdRef.current) setLoading(false);
      });
    return () => {
      if (requestId === requestIdRef.current) requestIdRef.current += 1;
    };
  }, [connected, detail, identity, revision, supported]);

  const refresh = useCallback(() => {
    setRevision((current) => current + 1);
  }, []);

  return { catalog, failure, loading, refresh, supported };
}
