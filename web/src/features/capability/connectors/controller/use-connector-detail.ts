// INPUT: 当前 Connector identity、详情 API 与安全失败兜底。
// OUTPUT: 按 request generation 隔离的详情快照、失败事实和刷新动作。
// POS: Connector detail 的读取边界；切换 identity 时不得显示旧详情。
import { useCallback, useRef, useState } from "react";

import { getConnectorDetailApi } from "@/lib/api/capability/connector-api";
import {
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import type { ConnectorDetail } from "@/types/capability/connector";

interface UseConnectorDetailOptions {
  failureFallback: string;
}

export function useConnectorDetail({ failureFallback }: UseConnectorDetailOptions) {
  const requestIdRef = useRef(0);
  const selectedIdRef = useRef<string | null>(null);
  const [selectedDetail, setSelectedDetail] =
    useState<ConnectorDetail | null>(null);
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const loadDetail = useCallback(async (
    connectorId: string,
    clearCurrent: boolean,
  ): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    selectedIdRef.current = connectorId;
    setDetailLoading(true);
    setFailure(null);
    if (clearCurrent) {
      setSelectedDetail(null);
    }
    try {
      const detail = await getConnectorDetailApi(connectorId);
      if (requestId === requestIdRef.current) {
        setSelectedDetail(detail);
        setFailure(null);
      }
      return requestId === requestIdRef.current;
    } catch (error) {
      if (requestId === requestIdRef.current) {
        setFailure(getResourceFailure(error, failureFallback));
      }
      return false;
    } finally {
      if (requestId === requestIdRef.current) {
        setDetailLoading(false);
      }
    }
  }, [failureFallback]);

  const openDetail = useCallback(async (connectorId: string): Promise<void> => {
    await loadDetail(connectorId, true);
  }, [loadDetail]);

  const reconcileDetail = useCallback((connectorId: string) => {
    if (selectedIdRef.current !== connectorId) {
      return Promise.resolve(true);
    }
    return loadDetail(connectorId, false);
  }, [loadDetail]);

  const refreshDetail = useCallback(async (connectorId: string): Promise<void> => {
    await reconcileDetail(connectorId);
  }, [reconcileDetail]);

  const closeDetail = useCallback(() => {
    requestIdRef.current += 1;
    selectedIdRef.current = null;
    setSelectedDetail(null);
    setFailure(null);
    setDetailLoading(false);
  }, []);

  return {
    closeDetail,
    detailLoading,
    failure,
    openDetail,
    reconcileDetail,
    refreshDetail,
    selectedDetail,
  };
}
