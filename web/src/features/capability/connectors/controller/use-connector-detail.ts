import { useCallback, useRef, useState } from "react";

import { getConnectorDetailApi } from "@/lib/api/capability/connector-api";
import type { ConnectorDetail } from "@/types/capability/connector";

interface UseConnectorDetailOptions {
  onError: (message: string) => void;
}

export function useConnectorDetail({ onError }: UseConnectorDetailOptions) {
  const requestIdRef = useRef(0);
  const selectedIdRef = useRef<string | null>(null);
  const [selectedDetail, setSelectedDetail] =
    useState<ConnectorDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const loadDetail = useCallback(async (
    connectorId: string,
    clearCurrent: boolean,
  ) => {
    const requestId = ++requestIdRef.current;
    selectedIdRef.current = connectorId;
    setDetailLoading(true);
    if (clearCurrent) {
      setSelectedDetail(null);
    }
    try {
      const detail = await getConnectorDetailApi(connectorId);
      if (requestId === requestIdRef.current) {
        setSelectedDetail(detail);
      }
    } catch (error) {
      if (requestId === requestIdRef.current) {
        onError(error instanceof Error ? error.message : "获取详情失败");
      }
    } finally {
      if (requestId === requestIdRef.current) {
        setDetailLoading(false);
      }
    }
  }, [onError]);

  const openDetail = useCallback((connectorId: string) => (
    loadDetail(connectorId, true)
  ), [loadDetail]);

  const refreshDetail = useCallback((connectorId: string) => {
    if (selectedIdRef.current !== connectorId) {
      return Promise.resolve();
    }
    return loadDetail(connectorId, false);
  }, [loadDetail]);

  const closeDetail = useCallback(() => {
    requestIdRef.current += 1;
    selectedIdRef.current = null;
    setSelectedDetail(null);
    setDetailLoading(false);
  }, []);

  return {
    closeDetail,
    detailLoading,
    openDetail,
    refreshDetail,
    selectedDetail,
  };
}
