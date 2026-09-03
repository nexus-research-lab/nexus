// INPUT: Connector 目录 API、筛选条件和当前语言的安全失败兜底。
// OUTPUT: 保留最后成功快照的目录资源、筛选结果与显式刷新动作。
// POS: Connector catalog 的读取边界；不把普通异常正文交给视图。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getConnectorsApi } from "@/lib/api/capability/connector-api";
import {
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import type { ConnectorInfo } from "@/types/capability/connector";

import { filterConnectors } from "../catalog/connector-catalog-model";
import { getAvailableConnectorCategoryKeys } from "../catalog/connectors-categories";

interface UseConnectorCatalogOptions {
  failureFallback: string;
}

export function useConnectorCatalog({
  failureFallback,
}: UseConnectorCatalogOptions) {
  const requestIdRef = useRef(0);
  const [allConnectors, setAllConnectors] = useState<ConnectorInfo[]>([]);
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState("all");

  const reconcile = useCallback(async (): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const items = await getConnectorsApi({ status: "available" });
      if (requestId === requestIdRef.current) {
        setAllConnectors(items);
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
        setLoading(false);
      }
    }
  }, [failureFallback]);

  const refresh = useCallback(async (): Promise<void> => {
    await reconcile();
  }, [reconcile]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const catalogConnectors = useMemo(
    () => allConnectors.filter((item) => (
      item.kind !== "custom_mcp" && item.status === "available"
    )),
    [allConnectors],
  );
  const categoryKeys = useMemo(
    () => getAvailableConnectorCategoryKeys(catalogConnectors),
    [catalogConnectors],
  );
  useEffect(() => {
    if (
      activeCategory !== "all"
      && !categoryKeys.includes(activeCategory)
    ) {
      setActiveCategory("all");
    }
  }, [activeCategory, categoryKeys]);
  const connectors = useMemo(() => filterConnectors(
    catalogConnectors,
    activeCategory,
    searchQuery,
  ), [activeCategory, catalogConnectors, searchQuery]);

  return {
    activeCategory,
    allConnectors,
    categoryKeys,
    connectors,
    failure,
    loading,
    reconcile,
    refresh,
    searchQuery,
    setActiveCategory,
    setSearchQuery,
  };
}
