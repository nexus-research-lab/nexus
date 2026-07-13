import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getConnectorsApi } from "@/lib/api/capability/connector-api";
import type { ConnectorInfo } from "@/types/capability/connector";

import {
  countConnectedConnectors,
  filterConnectors,
} from "../catalog/connector-catalog-model";

interface UseConnectorCatalogOptions {
  onError: (message: string) => void;
}

export function useConnectorCatalog({ onError }: UseConnectorCatalogOptions) {
  const requestIdRef = useRef(0);
  const [allConnectors, setAllConnectors] = useState<ConnectorInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState("all");

  const refresh = useCallback(async () => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const items = await getConnectorsApi();
      if (requestId === requestIdRef.current) {
        setAllConnectors(items);
      }
    } catch (error) {
      if (requestId === requestIdRef.current) {
        onError(error instanceof Error ? error.message : "加载失败");
      }
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [onError]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const connectors = useMemo(() => filterConnectors(
    allConnectors,
    activeCategory,
    searchQuery,
  ), [activeCategory, allConnectors, searchQuery]);
  const connectedCount = useMemo(
    () => countConnectedConnectors(allConnectors),
    [allConnectors],
  );

  return {
    activeCategory,
    allConnectors,
    connectedCount,
    connectors,
    loading,
    refresh,
    searchQuery,
    setActiveCategory,
    setSearchQuery,
  };
}
